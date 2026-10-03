import {
  ArrowLeft01Icon,
  CheckmarkCircle02Icon,
  Download04Icon,
  LinkSquare02Icon,
  MessageEdit01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { portalDeliverableQuery, portalFileUrl, useDecide } from '@/features/portal/api'
import { PORTAL_DELIVERABLE_STATUS, since } from '@/features/portal/format'
import { StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { PortalDeliverableDetail, PortalVersion } from '@/types/api'

/**
 * Un livrable, vu par le client : la version a consulter, la reponse a
 * donner, et le fil des versions precedentes.
 *
 * Valider se fait d'un geste. Demander des retours ouvre un champ : des
 * retours sans un mot ne servent a rien a l'equipe, et le serveur les refuse.
 */
export const Route = createFileRoute('/client/livrables/$id')({
  loader: ({ context, params }) => context.queryClient.ensureQueryData(portalDeliverableQuery(params.id)),
  component: ClientDeliverablePage,
  errorComponent: () => (
    <div className="flex flex-col items-start gap-3">
      <Link to="/client" className="flex items-center gap-1 text-[14px] text-[#73757c] hover:text-[#1b1b1b]">
        <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
        Mes projets
      </Link>
      <p className="rounded-[14px] border border-[#e8e8e9] bg-white p-6 text-[15px] text-[#4b4b4f]">
        Ce livrable est introuvable, ou il ne fait pas partie de votre espace.
      </p>
    </div>
  ),
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

function ClientDeliverablePage() {
  const { id } = Route.useParams()
  const { data: deliverable } = useQuery(portalDeliverableQuery(id))

  if (deliverable === undefined) return null

  const [current, ...previous] = deliverable.versions
  const status = PORTAL_DELIVERABLE_STATUS[deliverable.status]

  return (
    <div className="mx-auto flex w-full max-w-[720px] flex-col gap-5">
      <Link
        to="/client/projets/$id"
        params={{ id: deliverable.project.id }}
        className="flex items-center gap-1 self-start text-[14px] text-[#73757c] hover:text-[#1b1b1b]"
      >
        <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
        {deliverable.project.name}
      </Link>

      <header className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="font-heading text-[24px] leading-tight font-medium text-[#1b1b1b]">{deliverable.title}</h1>
          <StatusPill label={status.label} color={status.color} pill={status.pill} />
        </div>
        {deliverable.milestone !== null && (
          <p className="text-[13px] text-[#8d8d8d]">Étape : {deliverable.milestone}</p>
        )}
        {deliverable.description !== '' && (
          <p className="text-[15px] leading-[1.6] text-[#4b4b4f]">{deliverable.description}</p>
        )}
      </header>

      {current !== undefined && <CurrentVersion deliverable={deliverable} version={current} />}

      {previous.length > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="text-[16px] font-medium text-[#1b1b1b]">Versions précédentes</h2>
          <ol className="flex flex-col rounded-[14px] border border-[#e8e8e9] bg-white">
            {previous.map((version) => (
              <li key={version.numero} className="flex flex-col gap-1 border-b border-[#f3f4f4] px-4 py-3 last:border-b-0">
                <span className="flex flex-wrap items-center gap-2">
                  <span className="text-[14px] font-medium text-[#1b1b1b]">Version {version.numero}</span>
                  <VersionLinks version={version} compact />
                </span>
                <Decision version={version} />
              </li>
            ))}
          </ol>
        </section>
      )}
    </div>
  )
}

/** Le lien vers la version : une preproduction, une maquette, ou un fichier. */
function VersionLinks({ version, compact = false }: { version: PortalVersion; compact?: boolean }) {
  const className = compact
    ? 'flex items-center gap-1 text-[13px] text-[#4b4b4f] underline underline-offset-2 hover:text-[#1b1b1b]'
    : 'flex items-center justify-center gap-2 rounded-[12px] border border-[#e8e8e9] bg-white px-4 py-3 text-[15px] font-medium text-[#1b1b1b] transition-colors hover:bg-[#fafafa]'

  return (
    <>
      {version.url !== '' && (
        <a href={version.url} target="_blank" rel="noopener noreferrer" className={className}>
          <HugeiconsIcon icon={LinkSquare02Icon} size={compact ? 14 : 18} strokeWidth={1.8} />
          Ouvrir la version {version.numero}
        </a>
      )}
      {version.file_id !== null && (
        <a href={portalFileUrl(version.file_id)} className={className}>
          <HugeiconsIcon icon={Download04Icon} size={compact ? 14 : 18} strokeWidth={1.8} />
          Télécharger
        </a>
      )}
    </>
  )
}

/** La reponse rendue sur une version, s'il y en a une. */
function Decision({ version }: { version: PortalVersion }) {
  if (version.decision === 'en_attente') {
    return <p className="text-[13px] text-[#8d8d8d]">Déposée le {WHEN.format(new Date(version.submitted_at))}</p>
  }

  return (
    <div className="flex flex-col gap-1">
      <p className="text-[13px] text-[#8d8d8d]">
        {version.decision === 'valide' ? 'Validée' : 'Retours envoyés'}
        {version.decided_at !== null && ` le ${WHEN.format(new Date(version.decided_at))}`}
        {version.decided_by !== '' && ` par ${version.decided_by}`}
      </p>
      {version.feedback !== '' && (
        <blockquote className="border-l-2 border-[#e3e4e6] pl-3 text-[14px] leading-[1.6] whitespace-pre-line text-[#4b4b4f]">
          {version.feedback}
        </blockquote>
      )}
    </div>
  )
}

function CurrentVersion({ deliverable, version }: { deliverable: PortalDeliverableDetail; version: PortalVersion }) {
  const decide = useDecide(deliverable.id)
  const [mode, setMode] = useState<'choice' | 'feedback'>('choice')
  const [feedback, setFeedback] = useState('')
  const [error, setError] = useState<string | null>(null)
  const pending = version.decision === 'en_attente'

  function send(decision: 'valide' | 'retours') {
    if (decision === 'retours' && feedback.trim() === '') {
      setError('Dites-nous ce qui doit changer')
      return
    }
    setError(null)

    decide.mutate(
      { decision, feedback: decision === 'retours' ? feedback.trim() : '' },
      {
        onSuccess: () =>
          toast.success(
            decision === 'valide'
              ? 'Merci, la version est validée. L’équipe est prévenue.'
              : 'Merci, vos retours sont envoyés à l’équipe.',
          ),
        onError: (err) => {
          if (err instanceof HttpError && err.code === 'VALIDATION_FAILED') {
            setError(String(err.details.feedback ?? err.message))
            return
          }
          toast.error(err instanceof HttpError ? err.message : 'Envoi impossible')
        },
      },
    )
  }

  return (
    <section
      className={cn(
        'flex flex-col gap-4 rounded-[16px] border bg-white p-4 sm:p-5',
        pending ? 'border-[#ffd9c2] shadow-[0_8px_28px_-12px_rgb(255_120_43/0.35)]' : 'border-[#e8e8e9]',
      )}
    >
      <div className="flex flex-col gap-0.5">
        <span className="text-[13px] tracking-wide text-[#8d8d8d] uppercase">Version {version.numero}</span>
        <span className="text-[14px] text-[#4b4b4f]">
          Déposée {since(version.submitted_at)}
          {version.submitted_by !== '' && ` par ${version.submitted_by}`}
        </span>
      </div>

      <div className="flex flex-col gap-2 sm:flex-row">
        <VersionLinks version={version} />
      </div>

      {!pending && <Decision version={version} />}

      {pending && mode === 'choice' && (
        <div className="flex flex-col gap-2 border-t border-[#f3f4f4] pt-4">
          <p className="text-[15px] text-[#1b1b1b]">Cette version vous convient-elle ?</p>
          <div className="flex flex-col gap-2 sm:flex-row">
            <Button size="lg" className="h-11 gap-2 sm:flex-1" disabled={decide.isPending} onClick={() => send('valide')}>
              <HugeiconsIcon icon={CheckmarkCircle02Icon} size={18} strokeWidth={2} />
              {decide.isPending ? 'Envoi…' : 'Valider cette version'}
            </Button>
            <Button
              size="lg"
              variant="outline"
              className="h-11 gap-2 sm:flex-1"
              disabled={decide.isPending}
              onClick={() => setMode('feedback')}
            >
              <HugeiconsIcon icon={MessageEdit01Icon} size={18} strokeWidth={1.8} />
              Demander des retours
            </Button>
          </div>
        </div>
      )}

      {pending && mode === 'feedback' && (
        <form
          noValidate
          className="flex flex-col gap-2 border-t border-[#f3f4f4] pt-4"
          onSubmit={(event) => {
            event.preventDefault()
            send('retours')
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
              if (error !== null) setError(null)
            }}
            placeholder="Soyez aussi précis que possible : la page, l’élément, ce que vous attendez à la place."
            aria-invalid={error !== null}
            className="text-[15px]"
          />
          {error !== null && <p className="text-[13px] text-[#e5484d]">{error}</p>}
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="ghost" size="lg" onClick={() => setMode('choice')}>
              Annuler
            </Button>
            <Button type="submit" size="lg" className="h-11" disabled={decide.isPending}>
              {decide.isPending ? 'Envoi…' : 'Envoyer mes retours'}
            </Button>
          </div>
        </form>
      )}
    </section>
  )
}
