import { CheckmarkCircle02Icon, LinkSquare02Icon, MessageEdit01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import { z } from 'zod'

import logoUrl from '@/assets/sidebar/logo.svg'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { portalReviewQuery } from '@/features/portal/api'
import { api, HttpError, unwrap } from '@/lib/api'
import type { PortalReview } from '@/types/api'

/**
 * Repondre a un livrable depuis l'e-mail, sans se connecter.
 *
 * Hors du portail (`client_` : pas de garde de session) : le jeton signe du
 * lien dit qui repond. La page montre ce qu'on juge, puis le geste choisi
 * dans l'e-mail — valider, ou dire ce qui doit changer — sans rien decider
 * avant qu'on l'ait confirme ici : un filtre anti-spam qui suit les liens d'un
 * e-mail ne valide rien.
 */
const searchSchema = z.object({
  token: z.string().catch(''),
  decision: z.enum(['valide', 'retours']).optional().catch(undefined),
})

export const Route = createFileRoute('/client_/livrables/$id/repondre')({
  validateSearch: searchSchema,
  component: ReviewPage,
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

function ReviewPage() {
  const { id } = Route.useParams()
  const { token, decision } = Route.useSearch()
  const { data, isPending, error } = useQuery({ ...portalReviewQuery(id, token), retry: false })

  if (isPending) {
    return (
      <Frame title="Un instant…">
        <div className="h-[160px] animate-pulse rounded-[14px] bg-white" />
      </Frame>
    )
  }

  if (error !== null || data === undefined) {
    const expired = error instanceof HttpError && error.status === 410
    return (
      <Frame
        title={expired ? 'Ce lien n’est plus valable' : 'Lien introuvable'}
        subtitle={
          expired
            ? 'Il a expiré, ou une nouvelle version a été déposée depuis. Votre espace client, lui, est toujours là.'
            : (error instanceof HttpError ? error.message : 'Ce lien ne mène nulle part.')
        }
      >
        <PortalLink id={id} label="Ouvrir le livrable dans mon espace" />
      </Frame>
    )
  }

  return <Review id={id} token={token} preset={decision} review={data} />
}

function Review({ id, token, preset, review }: { id: string; token: string; preset?: 'valide' | 'retours'; review: PortalReview }) {
  const queryClient = useQueryClient()
  const [mode, setMode] = useState<'valide' | 'retours'>(preset ?? 'valide')
  const [feedback, setFeedback] = useState('')
  const [fieldError, setFieldError] = useState<string | null>(null)

  const send = useMutation({
    mutationFn: async (values: { decision: 'valide' | 'retours'; feedback: string }) =>
      unwrap(
        await api.POST('/api/v1/public/deliverables/{id}/review', {
          params: { path: { id } },
          body: { token, ...values },
        }),
      ),
    onSuccess: (next) => queryClient.setQueryData(portalReviewQuery(id, token).queryKey, next),
  })

  function submit(decision: 'valide' | 'retours') {
    if (decision === 'retours' && feedback.trim() === '') {
      setFieldError('Dites-nous ce qui doit changer')
      return
    }
    setFieldError(null)
    send.mutate(
      { decision, feedback: decision === 'retours' ? feedback.trim() : '' },
      {
        onError: (err) => {
          if (err instanceof HttpError && err.code === 'VALIDATION_FAILED') {
            setFieldError(String(err.details.feedback ?? err.message))
          }
        },
      },
    )
  }

  const answered = review.status !== 'en_attente'
  const hello = review.firstname === '' ? 'Bonjour,' : `Bonjour ${review.firstname},`

  if (answered) {
    const validated = review.status === 'valide'
    return (
      <Frame
        title={validated ? 'Merci, la version est validée' : 'Merci, vos retours sont envoyés'}
        subtitle={
          validated
            ? `L’équipe est prévenue. « ${review.title} », version ${review.version}, est validée${review.decided_at !== null ? ` le ${WHEN.format(new Date(review.decided_at))}` : ''}.`
            : `L’équipe reprend « ${review.title} » avec vos remarques et vous proposera une nouvelle version.`
        }
      >
        <PortalLink id={id} label="Suivre dans mon espace" />
      </Frame>
    )
  }

  return (
    <Frame
      title={review.title}
      subtitle={
        <>
          {hello} {review.project_name} · version {review.version}
          {review.milestone !== null && ` · ${review.milestone}`}
        </>
      }
    >
      <section className="flex flex-col gap-4 rounded-[16px] border border-[#ffd9c2] bg-white p-4 shadow-[0_8px_28px_-12px_rgb(255_120_43/0.35)] sm:p-5">
        {review.description !== '' && (
          <p className="text-[15px] leading-[1.6] text-[#4b4b4f]">{review.description}</p>
        )}

        {review.url !== '' ? (
          <a
            href={review.url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center justify-center gap-2 rounded-[12px] border border-[#e8e8e9] bg-white px-4 py-3 text-[15px] font-medium text-[#1b1b1b] transition-colors hover:bg-[#fafafa]"
          >
            <HugeiconsIcon icon={LinkSquare02Icon} size={18} strokeWidth={1.8} />
            Ouvrir la version {review.version}
          </a>
        ) : (
          <p className="text-[13px] text-[#73757c]">
            Le fichier se télécharge depuis votre espace client, après connexion.
          </p>
        )}

        {mode === 'valide' ? (
          <div className="flex flex-col gap-2 border-t border-[#f3f4f4] pt-4">
            <p className="text-[15px] text-[#1b1b1b]">Cette version vous convient-elle ?</p>
            <Button size="lg" className="h-11 gap-2" disabled={send.isPending} onClick={() => submit('valide')}>
              <HugeiconsIcon icon={CheckmarkCircle02Icon} size={18} strokeWidth={2} />
              {send.isPending ? 'Envoi…' : 'Oui, je valide cette version'}
            </Button>
            <Button type="button" variant="ghost" size="lg" className="gap-2" disabled={send.isPending} onClick={() => setMode('retours')}>
              <HugeiconsIcon icon={MessageEdit01Icon} size={18} strokeWidth={1.8} />
              Plutôt faire un retour
            </Button>
          </div>
        ) : (
          <form
            noValidate
            className="flex flex-col gap-2 border-t border-[#f3f4f4] pt-4"
            onSubmit={(event) => {
              event.preventDefault()
              submit('retours')
            }}
          >
            <label htmlFor="feedback" className="text-[15px] text-[#1b1b1b]">
              Qu’est-ce qui doit changer ?
            </label>
            <Textarea
              id="feedback"
              autoFocus
              rows={5}
              value={feedback}
              onChange={(event) => {
                setFeedback(event.target.value)
                if (fieldError !== null) setFieldError(null)
              }}
              placeholder="Soyez aussi précis que possible : la page, l’élément, ce que vous attendez à la place."
              aria-invalid={fieldError !== null}
              className="text-[15px]"
            />
            {fieldError !== null && <p className="text-[13px] text-[#e5484d]">{fieldError}</p>}
            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button type="button" variant="ghost" size="lg" onClick={() => setMode('valide')}>
                Finalement, valider
              </Button>
              <Button type="submit" size="lg" className="h-11" disabled={send.isPending}>
                {send.isPending ? 'Envoi…' : 'Envoyer mes retours'}
              </Button>
            </div>
          </form>
        )}

        {send.error !== null && !(send.error instanceof HttpError && send.error.code === 'VALIDATION_FAILED') && (
          <p className="rounded-[10px] bg-[#fdf3f3] px-3 py-2 text-[13px] text-[#e5484d]">
            {send.error instanceof HttpError ? send.error.message : 'Envoi impossible, réessayez.'}
          </p>
        )}
      </section>

      <PortalLink id={id} label="Voir toutes les versions dans mon espace" muted />
    </Frame>
  )
}

/** Vers la page du livrable dans le portail : la connexion s'y fait s'il faut. */
function PortalLink({ id, label, muted = false }: { id: string; label: string; muted?: boolean }) {
  return (
    <Link
      to="/client/livrables/$id"
      params={{ id }}
      className={
        muted
          ? 'self-center text-[13px] text-[#73757c] underline underline-offset-2 hover:text-[#1b1b1b]'
          : 'flex items-center justify-center rounded-[12px] bg-[#1b1b1b] px-4 py-3 text-[15px] font-medium text-white hover:bg-[#333]'
      }
    >
      {label}
    </Link>
  )
}

/** Meme sobriete que les pages ouvertes depuis un e-mail : le logo, le sujet, l'action. */
function Frame({ title, subtitle, children }: { title: string; subtitle?: ReactNode; children: ReactNode }) {
  return (
    <main className="flex min-h-full items-center justify-center bg-[#f7f7f8] px-4 py-12">
      <div className="flex w-full max-w-[560px] flex-col gap-6">
        <div className="flex flex-col items-center gap-4 text-center">
          <img src={logoUrl} alt="Piilot" className="size-12" />
          <div className="flex flex-col gap-1.5">
            <h1 className="font-heading text-[24px] leading-[1.3] font-medium text-[#1b1b1b]">{title}</h1>
            {subtitle !== undefined && <p className="text-[15px] leading-[1.5] text-[#73757c]">{subtitle}</p>}
          </div>
        </div>
        {children}
      </div>
    </main>
  )
}
