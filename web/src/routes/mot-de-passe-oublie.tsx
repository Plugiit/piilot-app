import { MailSend01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { z } from 'zod'

import { AuthCard } from '@/components/auth-card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { forgotConfigQuery, requestPasswordReset } from '@/features/accounts/public-api'
import { HttpError } from '@/lib/api'

const searchSchema = z.object({
  email: z.string().optional(),
})

export const Route = createFileRoute('/mot-de-passe-oublie')({
  validateSearch: searchSchema,
  component: ForgotPage,
})

/**
 * Demande de lien de reinitialisation.
 *
 * La confirmation est la meme, que l'adresse existe ou non : la page ne doit
 * pas servir a savoir qui a un compte. Elle le dit franchement plutot que
 * d'affirmer qu'un e-mail est parti.
 *
 * Sans envoi d'e-mails sur l'instance, la page ne promet rien : elle oriente
 * vers un administrateur, qui peut creer un lien depuis l'ecran des comptes.
 */
function ForgotPage() {
  const { email: prefilled } = Route.useSearch()
  const { data: config } = useQuery(forgotConfigQuery)
  const [email, setEmail] = useState(prefilled ?? '')
  const [error, setError] = useState('')

  const request = useMutation({
    mutationFn: () => requestPasswordReset(email.trim()),
    onError: (err) => {
      setError(
        err instanceof HttpError && err.status === 429
          ? 'Trop de demandes depuis cette connexion. Réessayez dans quelques minutes.'
          : err instanceof HttpError
            ? err.message
            : 'Demande impossible pour le moment',
      )
    },
  })

  if (config !== undefined && !config.mail_enabled) {
    return (
      <AuthCard title="Mot de passe oublié">
        <p className="text-[15px] leading-[1.6] text-[#4b4b4f]">
          Cette instance de Piilot n’envoie pas d’e-mails. Demandez à un administrateur de vous créer
          un lien de réinitialisation : il le trouve dans Paramètres, Comptes et rôles, sur votre
          compte.
        </p>
      </AuthCard>
    )
  }

  if (request.isSuccess) {
    return (
      <AuthCard title="Vérifiez vos e-mails">
        <div className="flex flex-col items-center gap-4 text-center">
          <div className="flex size-12 items-center justify-center rounded-full bg-[#fff1e8] text-[#ff782b]">
            <HugeiconsIcon icon={MailSend01Icon} size={24} strokeWidth={1.6} />
          </div>
          <p className="text-[15px] leading-[1.6] text-[#4b4b4f]">
            Si un compte actif existe pour <strong className="text-[#1b1b1b]">{email.trim()}</strong>,
            un e-mail vient de partir avec un lien valable une heure.
          </p>
          <p className="text-[13px] leading-[1.6] text-[#73757c]">
            Rien reçu d’ici quelques minutes ? Regardez dans les courriers indésirables, vérifiez
            l’adresse, ou demandez un lien à un administrateur.
          </p>
          <Button variant="outline" onClick={() => request.reset()}>
            Utiliser une autre adresse
          </Button>
        </div>
      </AuthCard>
    )
  }

  return (
    <AuthCard
      title="Mot de passe oublié"
      subtitle="Indiquez l’adresse de votre compte : nous vous enverrons un lien pour en choisir un nouveau."
    >
      <form
        noValidate
        onSubmit={(event) => {
          event.preventDefault()
          if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
            setError('Adresse invalide')
            return
          }
          setError('')
          request.mutate()
        }}
        className="flex flex-col gap-4"
      >
        <label className="flex flex-col gap-1.5">
          <span className="text-[14px] font-medium text-[#1b1b1b]">Adresse e-mail</span>
          <Input
            type="email"
            autoFocus
            autoComplete="email"
            value={email}
            aria-invalid={Boolean(error)}
            onChange={(event) => setEmail(event.target.value)}
            className="h-11"
          />
          {error && <span className="text-[13px] text-[#e5484d]">{error}</span>}
        </label>

        <Button type="submit" size="lg" className="h-11" disabled={request.isPending}>
          {request.isPending ? 'Envoi…' : 'Recevoir un lien'}
        </Button>
      </form>
    </AuthCard>
  )
}
