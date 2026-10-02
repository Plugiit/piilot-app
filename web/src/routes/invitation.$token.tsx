import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { AuthCard, PasswordField } from '@/components/auth-card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { roleOf } from '@/features/accounts/format'
import { acceptInvitation, invitationQuery } from '@/features/accounts/public-api'
import { HttpError } from '@/lib/api'
import { homeFor, sessionQuery } from '@/lib/auth'

export const Route = createFileRoute('/invitation/$token')({
  component: InvitationPage,
})

/**
 * Activation d'un compte a partir d'une invitation.
 *
 * La page dit d'abord de quoi il s'agit — qui invite, pour quel role, avec
 * quelle adresse — avant de demander quoi que ce soit : on ne choisit pas un
 * mot de passe pour un service qu'on n'identifie pas. A la validation, la
 * session s'ouvre et la personne arrive dans son espace, sans repasser par la
 * connexion.
 */
function InvitationPage() {
  const { token } = Route.useParams()
  const { data: invitation, error, isPending } = useQuery(invitationQuery(token))

  if (isPending) {
    return (
      <AuthCard title="Votre invitation">
        <div className="h-[260px] animate-pulse rounded-[10px] bg-[#fafafa]" />
      </AuthCard>
    )
  }

  if (error !== null || invitation === undefined) {
    const code = error instanceof HttpError ? error.code : ''
    const used = code === 'INVITATION_USED'
    return (
      <AuthCard
        title={used ? 'Invitation déjà utilisée' : code === 'INVITATION_EXPIRED' ? 'Invitation expirée' : 'Lien non valable'}
        footer={null}
      >
        <div className="flex flex-col gap-4">
          <p className="text-[15px] leading-[1.6] text-[#4b4b4f]">
            {error instanceof HttpError ? error.message : 'Ce lien ne peut pas être lu pour le moment.'}
          </p>
          <Button asChild size="lg" variant={used ? 'default' : 'outline'}>
            <Link to="/login">{used ? 'Se connecter' : 'Aller à la connexion'}</Link>
          </Button>
        </div>
      </AuthCard>
    )
  }

  return <AcceptForm token={token} invitation={invitation} />
}

function AcceptForm({
  token,
  invitation,
}: {
  token: string
  invitation: { email: string; firstname: string; lastname: string; role: string; invited_by: string; client_name: string }
}) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [firstname, setFirstname] = useState(invitation.firstname)
  const [lastname, setLastname] = useState(invitation.lastname)
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})

  const accept = useMutation({
    mutationFn: () => acceptInvitation(token, { firstname, lastname, password }),
    onSuccess: (user) => {
      queryClient.setQueryData(sessionQuery.queryKey, user)
      toast.success(`Bienvenue sur Piilot, ${user.firstname} !`)
      void navigate({ to: homeFor(user) })
    },
    onError: (error) => {
      if (error instanceof HttpError && error.code === 'VALIDATION_FAILED') {
        setErrors(Object.fromEntries(Object.entries(error.details).map(([k, v]) => [k, String(v)])))
        return
      }
      toast.error(error instanceof HttpError ? error.message : 'Activation impossible')
    },
  })

  function submit(event: React.FormEvent) {
    event.preventDefault()
    const local: Record<string, string> = {}
    if (firstname.trim() === '') local.firstname = 'Votre prénom est requis'
    if (lastname.trim() === '') local.lastname = 'Votre nom est requis'
    if ([...password].length < 12) local.password = '12 caractères au minimum'
    if (confirmation !== password) local.confirmation = 'Les deux saisies diffèrent'
    setErrors(local)
    if (Object.keys(local).length === 0) accept.mutate()
  }

  const role = roleOf(invitation.role)
  const context = invitation.client_name !== '' ? ` pour suivre les projets de ${invitation.client_name}` : ''

  return (
    <AuthCard
      title="Activez votre compte"
      subtitle={
        invitation.invited_by !== ''
          ? `${invitation.invited_by} vous invite à rejoindre Piilot${context}.`
          : `Votre compte Piilot vous attend${context}.`
      }
    >
      <form noValidate onSubmit={submit} className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-3 rounded-[10px] bg-[#f7f7f8] px-3 py-2.5">
          <div className="flex min-w-0 flex-col">
            <span className="text-[12px] text-[#73757c]">Votre adresse de connexion</span>
            <span className="truncate text-[14px] font-medium text-[#1b1b1b]">{invitation.email}</span>
          </div>
          <span className="inline-flex shrink-0 items-center gap-1.5 rounded-full border border-[#e8e8e9] bg-white px-2 py-0.5 text-[12px]">
            <span aria-hidden className="size-1.5 rounded-full" style={{ backgroundColor: role.color }} />
            {role.label}
          </span>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <label className="flex flex-col gap-1.5">
            <span className="text-[14px] font-medium text-[#1b1b1b]">Prénom</span>
            <Input
              value={firstname}
              autoFocus={invitation.firstname === ''}
              autoComplete="given-name"
              aria-invalid={Boolean(errors.firstname)}
              onChange={(event) => setFirstname(event.target.value)}
              className="h-11"
            />
            {errors.firstname && <span className="text-[13px] text-[#e5484d]">{errors.firstname}</span>}
          </label>
          <label className="flex flex-col gap-1.5">
            <span className="text-[14px] font-medium text-[#1b1b1b]">Nom</span>
            <Input
              value={lastname}
              autoComplete="family-name"
              aria-invalid={Boolean(errors.lastname)}
              onChange={(event) => setLastname(event.target.value)}
              className="h-11"
            />
            {errors.lastname && <span className="text-[13px] text-[#e5484d]">{errors.lastname}</span>}
          </label>
        </div>

        <PasswordField
          id="password"
          label="Mot de passe"
          value={password}
          onChange={setPassword}
          error={errors.password}
          autoFocus={invitation.firstname !== ''}
          showMeter
        />
        <PasswordField
          id="confirmation"
          label="Confirmez le mot de passe"
          value={confirmation}
          onChange={setConfirmation}
          error={errors.confirmation}
        />

        <Button type="submit" size="lg" className="mt-1 h-11" disabled={accept.isPending}>
          {accept.isPending ? 'Activation…' : 'Activer mon compte'}
        </Button>
      </form>
    </AuthCard>
  )
}
