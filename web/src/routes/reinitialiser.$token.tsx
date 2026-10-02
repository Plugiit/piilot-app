import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { AuthCard, PasswordField } from '@/components/auth-card'
import { Button } from '@/components/ui/button'
import { passwordResetQuery, resetPassword } from '@/features/accounts/public-api'
import { HttpError } from '@/lib/api'

export const Route = createFileRoute('/reinitialiser/$token')({
  component: ResetPage,
})

/**
 * Choix d'un nouveau mot de passe, depuis le lien recu par e-mail.
 *
 * Toutes les sessions du compte sont fermees a la validation : un mot de passe
 * oublie peut etre un mot de passe vole. La personne repart donc vers la
 * connexion, son adresse deja saisie.
 */
function ResetPage() {
  const { token } = Route.useParams()
  const { data, error, isPending } = useQuery(passwordResetQuery(token))

  if (isPending) {
    return (
      <AuthCard title="Nouveau mot de passe">
        <div className="h-[200px] animate-pulse rounded-[10px] bg-[#fafafa]" />
      </AuthCard>
    )
  }

  if (error !== null || data === undefined) {
    return (
      <AuthCard title="Lien non valable" footer={null}>
        <div className="flex flex-col gap-4">
          <p className="text-[15px] leading-[1.6] text-[#4b4b4f]">
            {error instanceof HttpError ? error.message : 'Ce lien ne peut pas être lu pour le moment.'}
          </p>
          <Button asChild size="lg">
            <Link to="/mot-de-passe-oublie">Faire une nouvelle demande</Link>
          </Button>
          <Link to="/login" className="text-center text-[14px] text-[#73757c] hover:underline">
            Retour à la connexion
          </Link>
        </div>
      </AuthCard>
    )
  }

  return <ResetForm token={token} email={data.email} />
}

function ResetForm({ token, email }: { token: string; email: string }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})

  const reset = useMutation({
    mutationFn: () => resetPassword(token, password),
    onSuccess: (result) => {
      queryClient.clear()
      toast.success('Mot de passe changé. Connectez-vous avec le nouveau.')
      void navigate({ to: '/login', search: { email: result.email } })
    },
    onError: (error) => {
      if (error instanceof HttpError && error.code === 'VALIDATION_FAILED') {
        setErrors({ password: String(error.details.password ?? error.message) })
        return
      }
      toast.error(error instanceof HttpError ? error.message : 'Changement impossible')
    },
  })

  return (
    <AuthCard
      title="Nouveau mot de passe"
      subtitle={
        <>
          Pour le compte <strong className="text-[#1b1b1b]">{email}</strong>.
        </>
      }
    >
      <form
        noValidate
        onSubmit={(event) => {
          event.preventDefault()
          const local: Record<string, string> = {}
          if ([...password].length < 12) local.password = '12 caractères au minimum'
          if (confirmation !== password) local.confirmation = 'Les deux saisies diffèrent'
          setErrors(local)
          if (Object.keys(local).length === 0) reset.mutate()
        }}
        className="flex flex-col gap-4"
      >
        <PasswordField
          id="password"
          label="Nouveau mot de passe"
          value={password}
          onChange={setPassword}
          error={errors.password}
          autoFocus
          showMeter
        />
        <PasswordField
          id="confirmation"
          label="Confirmez le mot de passe"
          value={confirmation}
          onChange={setConfirmation}
          error={errors.confirmation}
        />
        <p className="text-[13px] text-[#73757c]">
          Par sécurité, toutes vos sessions seront fermées, sur tous vos appareils.
        </p>
        <Button type="submit" size="lg" className="h-11" disabled={reset.isPending}>
          {reset.isPending ? 'Enregistrement…' : 'Changer le mot de passe'}
        </Button>
      </form>
    </AuthCard>
  )
}
