import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, redirect } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { z } from 'zod'

import { AuthCard } from '@/components/auth-card'
import { Button } from '@/components/ui/button'
import { consentQuery, useDecide } from '@/features/claude/api'
import { ConsentForm } from '@/features/claude/consent'
import { consentErrorMessage, decisionScopes } from '@/features/claude/format'
import { HttpError } from '@/lib/api'
import { homeFor, sessionQuery } from '@/lib/auth'

const searchSchema = z.object({
  // La demande validee par /oauth/authorize, en attente du consentement.
  demande: z.string().optional(),
  // Ou le refus du serveur, quand la demande ne pouvait pas revenir a Claude.
  erreur: z.string().optional(),
})

export const Route = createFileRoute('/autoriser')({
  validateSearch: searchSchema,
  // Il faut etre connecte pour consentir. Un refus du serveur, lui, s'affiche
  // sans session : il ne revele rien.
  beforeLoad: async ({ context, location, search }) => {
    if (search.erreur !== undefined || search.demande === undefined) return { user: null }
    try {
      const user = await context.queryClient.query({ ...sessionQuery, staleTime: 'static' })
      return { user }
    } catch {
      throw redirect({ to: '/login', search: { redirect: location.href } })
    }
  },
  component: AuthorizePage,
})

/**
 * L'ecran ou un compte consent a connecter Claude a Piilot.
 *
 * Claude y arrive par /oauth/authorize, qui a deja verifie le client, son
 * adresse de retour et sa preuve PKCE. Ici on dit qui demande quoi, et on
 * tranche ; la reponse renvoie le navigateur vers Claude.
 */
function AuthorizePage() {
  const { demande, erreur } = Route.useSearch()
  const { user } = Route.useRouteContext()

  if (erreur !== undefined || demande === undefined || user === null) {
    return (
      <Refusal
        title="Connexion impossible"
        message={consentErrorMessage(erreur ?? '')}
        action={
          <Button asChild size="lg" variant="outline">
            <Link to="/login">Aller à Piilot</Link>
          </Button>
        }
      />
    )
  }

  return <Consent id={demande} email={user.email} home={homeFor(user)} />
}

function Consent({ id, email, home }: { id: string; email: string; home: ReturnType<typeof homeFor> }) {
  const request = useQuery(consentQuery(id))
  const decide = useDecide(id)

  if (request.isPending) {
    return (
      <AuthCard title="Connecter Claude" footer={null}>
        <div className="h-[360px] animate-pulse rounded-[10px] bg-[#fafafa]" />
      </AuthCard>
    )
  }

  // Une demande expiree pendant la lecture de l'ecran se decouvre a la
  // decision : l'ecran bascule alors comme s'il l'avait vue en chargeant.
  const expiredOnDecide = decide.error instanceof HttpError && decide.error.code === 'OAUTH_REQUEST_EXPIRED'
  const failure = request.isError ? request.error : expiredOnDecide ? decide.error : null

  if (failure !== null) {
    const code = failure instanceof HttpError ? failure.code : ''
    return (
      <Refusal
        title={code === 'OAUTH_REQUEST_EXPIRED' ? 'Demande expirée' : code === 'FORBIDDEN' ? 'Connexion non permise' : 'Connexion impossible'}
        message={
          code === 'FORBIDDEN'
            ? 'Votre compte ne peut pas connecter Claude. Un administrateur peut vous en donner le droit.'
            : failure instanceof HttpError
              ? failure.message
              : 'La demande ne peut pas être lue pour le moment.'
        }
        action={
          <Button asChild size="lg" variant="outline">
            <Link to={home}>Retour à Piilot</Link>
          </Button>
        }
      />
    )
  }

  if (request.data === undefined) return null

  return (
    <AuthCard
      title={`Connecter ${request.data.client} à Piilot`}
      footer={
        <p className="text-center text-[13px] leading-[1.5] text-[#73757c]">
          Vous pourrez couper l’accès à tout moment, dans Mon compte › Intégrations.
        </p>
      }
    >
      <ConsentForm
        request={request.data}
        email={email}
        pending={decide.isPending || decide.isSuccess}
        onDecide={(approve, write) =>
          decide.mutate(
            { approve, scopes: decisionScopes(write) },
            {
              // Le navigateur repart vers Claude, avec le code ou le refus.
              onSuccess: (next) => window.location.assign(next),
              // Une demande expiree change l'ecran ; le reste se signale.
              onError: (error) => {
                if (error instanceof HttpError && error.code === 'OAUTH_REQUEST_EXPIRED') return
                toast.error(error instanceof HttpError ? error.message : 'La réponse n’a pas pu être enregistrée')
              },
            },
          )
        }
      />
    </AuthCard>
  )
}

function Refusal({ title, message, action }: { title: string; message: string; action: ReactNode }) {
  return (
    <AuthCard title={title} footer={null}>
      <div className="flex flex-col gap-4">
        <p className="text-[15px] leading-[1.6] text-[#4b4b4f]">{message}</p>
        {action}
      </div>
    </AuthCard>
  )
}
