import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { CHAMP, Card, Field } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useChangePassword } from '@/lib/auth'

import { reportError } from './-shared'

export const Route = createFileRoute('/_app/compte/securite')({
  component: SecurityPage,
})

function SecurityPage() {
  return <PasswordForm />
}

const passwordSchema = z
  .object({
    current_password: z.string().min(1, 'Votre mot de passe actuel est requis'),
    new_password: z.string().min(12, 'Douze caractères au minimum'),
    confirmation: z.string(),
  })
  .refine((values) => values.new_password === values.confirmation, {
    path: ['confirmation'],
    message: 'Les deux saisies diffèrent',
  })

/**
 * Changement de mot de passe.
 *
 * Le mot de passe courant est demande alors que la session est ouverte : sans
 * lui, un poste laisse sans surveillance suffirait a verrouiller le compte de
 * son titulaire.
 */
function PasswordForm() {
  const change = useChangePassword()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const form = useForm<z.infer<typeof passwordSchema>>({
    resolver: zodResolver(passwordSchema),
    defaultValues: { current_password: '', new_password: '', confirmation: '' },
  })

  return (
    <form
      onSubmit={form.handleSubmit((values) =>
        change.mutate(
          {
            current_password: values.current_password,
            new_password: values.new_password,
          },
          {
            onSuccess: () => {
              toast.success('Mot de passe changé. Reconnectez-vous.')
              // Le serveur vient de revoquer toutes les sessions, celle-ci
              // comprise : le cache porte des donnees qu'on ne peut plus
              // rafraichir.
              queryClient.clear()
              void navigate({ to: '/login' })
            },
            onError: reportError,
          },
        ),
      )}
    >
      <Card
        title="Sécurité"
        description="Changer le mot de passe ferme toutes les sessions, y compris celle-ci."
      >
        <Field
          label="Mot de passe actuel"
          error={form.formState.errors.current_password?.message}
        >
          <Input
            type="password"
            autoComplete="current-password"
            className={CHAMP}
            {...form.register('current_password')}
          />
        </Field>

        <Field
          label="Nouveau mot de passe"
          hint="Douze caractères au minimum."
          error={form.formState.errors.new_password?.message}
        >
          <Input
            type="password"
            autoComplete="new-password"
            className={CHAMP}
            {...form.register('new_password')}
          />
        </Field>

        <Field label="Confirmation" error={form.formState.errors.confirmation?.message}>
          <Input
            type="password"
            autoComplete="new-password"
            className={CHAMP}
            {...form.register('confirmation')}
          />
        </Field>

        <div className="flex items-center justify-end">
          <Button type="submit" size="lg" disabled={change.isPending}>
            {change.isPending ? 'Changement…' : 'Changer le mot de passe'}
          </Button>
        </div>
      </Card>
    </form>
  )
}
