import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { HttpError } from '@/lib/api'
import { homeFor, login, sessionQuery } from '@/lib/auth'

const searchSchema = z.object({
  redirect: z.string().optional(),
})

export const Route = createFileRoute('/login')({
  validateSearch: searchSchema,
  component: LoginPage,
})

const formSchema = z.object({
  email: z.email("L'adresse e-mail est invalide"),
  password: z.string().min(1, 'Le mot de passe est requis'),
})

type FormValues = z.infer<typeof formSchema>

function LoginPage() {
  const { redirect } = Route.useSearch()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { email: '', password: '' },
  })

  const mutation = useMutation({
    mutationFn: (values: FormValues) => login(values.email, values.password),
    onSuccess: (user) => {
      // La session est ecrite directement dans le cache : la redirection ne
      // declenche pas un second aller-retour vers /auth/me.
      queryClient.setQueryData(sessionQuery.queryKey, user)

      // `redirect` porte la page demandee avant la connexion. A defaut, le
      // role decide de l'espace d'atterrissage.
      void navigate({ to: redirect ?? homeFor(user) })
    },
    onError: (error) => {
      toast.error(error instanceof HttpError ? error.message : 'Connexion impossible')
    },
  })

  return (
    <div className="flex h-full items-center justify-center bg-surface-sunken p-6">
      <div className="w-full max-w-sm rounded-xl border border-border bg-surface-raised p-6 shadow-sm">
        <h1 className="text-xl font-semibold text-ink">Plugiit Admin</h1>
        <p className="mt-1 text-sm text-ink-muted">Connectez-vous pour continuer.</p>

        <form
          className="mt-6 space-y-4"
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
          noValidate
        >
          <div className="space-y-1.5">
            <label htmlFor="email" className="text-sm font-medium text-ink-soft">
              E-mail
            </label>
            <Input
              id="email"
              type="email"
              autoComplete="username"
              autoFocus
              {...form.register('email')}
            />
            {form.formState.errors.email && (
              <p className="text-xs text-danger">{form.formState.errors.email.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <label htmlFor="password" className="text-sm font-medium text-ink-soft">
              Mot de passe
            </label>
            <Input
              id="password"
              type="password"
              autoComplete="current-password"
              {...form.register('password')}
            />
            {form.formState.errors.password && (
              <p className="text-xs text-danger">{form.formState.errors.password.message}</p>
            )}
          </div>

          <Button type="submit" className="w-full" disabled={mutation.isPending}>
            {mutation.isPending ? 'Connexion…' : 'Se connecter'}
          </Button>
        </form>
      </div>
    </div>
  )
}
