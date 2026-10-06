import { ArrowLeft01Icon, ArrowRight01Icon, Download04Icon, File01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { MILESTONE_STATE } from '@/features/milestones/format'
import { portalFileUrl, portalProjectQuery } from '@/features/portal/api'
import { fileSize, PORTAL_DELIVERABLE_STATUS, PORTAL_PROJECT_STATUS, since } from '@/features/portal/format'
import { DONE_COLOR, parseApiDate, PROGRESS_COLOR, tracksSchedule } from '@/features/projects/format'
import { Meter, StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { Milestone, PortalDeliverable } from '@/types/api'

/**
 * Un projet vu par le client : ou il en est, ce qui attend sa reponse, les
 * etapes, les livrables et les documents que l'agence a partages.
 *
 * Les livrables a valider passent devant tout le reste : c'est ce que le
 * client a a faire. Le reste se lit.
 */
export const Route = createFileRoute('/client/projets/$id')({
  loader: ({ context, params }) => context.queryClient.ensureQueryData(portalProjectQuery(params.id)),
  component: ClientProjectPage,
  errorComponent: ({ error }) => (
    <NotFound message={error instanceof HttpError && error.status === 404 ? undefined : error.message} />
  ),
})

const LONG = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })

function NotFound({ message }: { message?: string }) {
  return (
    <div className="flex flex-col items-start gap-3">
      <BackLink />
      <p className="rounded-[14px] border border-[#e8e8e9] bg-white p-6 text-[15px] text-[#4b4b4f]">
        {message ?? 'Ce projet est introuvable, ou il ne fait pas partie de votre espace.'}
      </p>
    </div>
  )
}

function BackLink() {
  return (
    <Link to="/client" className="flex items-center gap-1 text-[14px] text-[#73757c] hover:text-[#1b1b1b]">
      <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
      Mes projets
    </Link>
  )
}

function ClientProjectPage() {
  const { id } = Route.useParams()
  const { data: project } = useQuery(portalProjectQuery(id))

  if (project === undefined) return null

  const status = PORTAL_PROJECT_STATUS[project.status]
  const start = parseApiDate(project.starts_on)
  const due = parseApiDate(project.due_on)
  const toValidate = project.deliverables.filter((d) => d.status === 'en_attente')
  const others = project.deliverables.filter((d) => d.status !== 'en_attente')

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-3">
        <BackLink />
        <header className="flex flex-col gap-3 rounded-[14px] border border-[#e8e8e9] bg-white p-4 sm:p-5">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="flex min-w-0 flex-col gap-1">
              <h1 className="font-heading text-[24px] leading-tight font-medium text-[#1b1b1b]">{project.name}</h1>
              {project.description !== '' && <p className="text-[15px] text-[#73757c]">{project.description}</p>}
            </div>
            <StatusPill label={status.label} color={status.color} pill={status.pill} />
          </div>

          {tracksSchedule(project.status) ? (
            <div className="flex flex-col gap-1.5">
              <div className="flex items-baseline justify-between text-[13px] text-[#73757c]">
                <span>Avancement</span>
                <span className="font-medium text-[#1b1b1b] tabular-nums">{project.progress} %</span>
              </div>
              <Meter ratio={project.progress} color={project.progress >= 100 ? DONE_COLOR : PROGRESS_COLOR} className="h-2" />
            </div>
          ) : (
            <p className="text-[13px] text-[#73757c]">
              Votre site est en ligne, hébergé et suivi par l’agence. Une question ou un souci : ouvrez un ticket.
            </p>
          )}

          {tracksSchedule(project.status) && (start !== null || due !== null) && (
            <p className="text-[13px] text-[#73757c]">
              {start !== null && `Démarré le ${LONG.format(start)}`}
              {start !== null && due !== null && ' · '}
              {due !== null && `Livraison prévue le ${LONG.format(due)}`}
            </p>
          )}
        </header>
      </div>

      {toValidate.length > 0 && (
        <Section id="a-valider" title="À valider" count={toValidate.length} tone="brand">
          <div className="flex flex-col gap-2">
            {toValidate.map((deliverable) => (
              <DeliverableRow key={deliverable.id} deliverable={deliverable} highlight />
            ))}
          </div>
        </Section>
      )}

      <Section title="Étapes du projet" count={project.milestones.length}>
        {project.milestones.length === 0 ? (
          <Empty>Les étapes du projet n’ont pas encore été posées.</Empty>
        ) : (
          <ol className="flex flex-col rounded-[14px] border border-[#e8e8e9] bg-white p-2">
            {project.milestones.map((milestone, index) => (
              <MilestoneLine key={milestone.id} milestone={milestone} last={index === project.milestones.length - 1} />
            ))}
          </ol>
        )}
      </Section>

      <Section title="Livrables" count={others.length}>
        {others.length === 0 ? (
          <Empty>
            {toValidate.length > 0 ? 'Pas d’autre livrable pour l’instant.' : 'Aucun livrable n’a encore été déposé.'}
          </Empty>
        ) : (
          <div className="flex flex-col gap-2">
            {others.map((deliverable) => (
              <DeliverableRow key={deliverable.id} deliverable={deliverable} />
            ))}
          </div>
        )}
      </Section>

      <Section title="Documents" count={project.files.length}>
        {project.files.length === 0 ? (
          <Empty>Aucun document partagé pour l’instant.</Empty>
        ) : (
          <ul className="flex flex-col rounded-[14px] border border-[#e8e8e9] bg-white">
            {project.files.map((file) => (
              <li key={file.id} className="border-b border-[#f3f4f4] last:border-b-0">
                <a
                  href={portalFileUrl(file.id)}
                  className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-[#fafafa]"
                >
                  <HugeiconsIcon icon={File01Icon} size={18} strokeWidth={1.6} className="shrink-0 text-[#8d8d8d]" />
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="truncate text-[14px] text-[#1b1b1b]">{file.filename}</span>
                    <span className="text-[12px] text-[#8d8d8d]">
                      {fileSize(file.size_bytes)} · partagé {since(file.created_at)}
                    </span>
                  </span>
                  <HugeiconsIcon icon={Download04Icon} size={18} strokeWidth={1.6} className="shrink-0 text-[#73757c]" />
                </a>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </div>
  )
}

function Section({
  id,
  title,
  count,
  tone,
  children,
}: {
  id?: string
  title: string
  count: number
  tone?: 'brand'
  children: ReactNode
}) {
  return (
    <section id={id} className="flex scroll-mt-20 flex-col gap-2">
      <h2 className="flex items-center gap-2 text-[16px] font-medium text-[#1b1b1b]">
        {title}
        {count > 0 && (
          <span
            className={cn(
              'rounded-full px-2 py-0.5 text-[12px] tabular-nums',
              tone === 'brand' ? 'bg-brand text-white' : 'bg-[#ebebeb] text-[#4b4b4f]',
            )}
          >
            {count}
          </span>
        )}
      </h2>
      {children}
    </section>
  )
}

function Empty({ children }: { children: ReactNode }) {
  return (
    <p className="rounded-[14px] border border-dashed border-[#e3e4e6] bg-white px-4 py-5 text-[14px] text-[#8d8d8d]">
      {children}
    </p>
  )
}

function DeliverableRow({ deliverable, highlight = false }: { deliverable: PortalDeliverable; highlight?: boolean }) {
  const status = PORTAL_DELIVERABLE_STATUS[deliverable.status]

  return (
    <Link
      to="/client/livrables/$id"
      params={{ id: deliverable.id }}
      className={cn(
        'flex items-center gap-3 rounded-[14px] border bg-white px-4 py-3 transition-shadow hover:shadow-[0_6px_20px_-8px_rgb(16_24_40/0.18)]',
        highlight ? 'border-[#ffd9c2]' : 'border-[#e8e8e9]',
      )}
    >
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-[15px] text-[#1b1b1b]">{deliverable.title}</span>
        <span className="truncate text-[12px] text-[#8d8d8d]">
          Version {deliverable.version} · déposée {since(deliverable.submitted_at)}
          {deliverable.milestone !== null && ` · ${deliverable.milestone}`}
        </span>
      </span>
      {highlight ? (
        <span className="flex shrink-0 items-center gap-1 rounded-full bg-brand px-3 py-1.5 text-[13px] font-medium text-white">
          Voir et répondre
          <HugeiconsIcon icon={ArrowRight01Icon} size={14} strokeWidth={2} />
        </span>
      ) : (
        <StatusPill label={status.label} color={status.color} pill={status.pill} />
      )}
    </Link>
  )
}

function MilestoneLine({ milestone, last }: { milestone: Milestone; last: boolean }) {
  const state = MILESTONE_STATE[milestone.state]
  const due = parseApiDate(milestone.due_on)

  return (
    <li className="flex gap-3 px-2">
      <div className="flex w-4 shrink-0 flex-col items-center pt-3">
        <span aria-hidden className="size-2.5 shrink-0 rounded-full" style={{ backgroundColor: state.color }} />
        {!last && <span aria-hidden className="mt-1 w-px flex-1 bg-[#e8e8e9]" />}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5 pt-2 pb-3">
        <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span className={cn('text-[15px] text-[#1b1b1b]', milestone.state === 'done' && 'text-[#73757c]')}>
            {milestone.title}
          </span>
          <StatusPill label={state.label} color={state.color} pill={state.pill} />
        </span>
        <span className="text-[13px] text-[#8d8d8d]">
          {due === null ? 'Date à venir' : LONG.format(due)}
          {milestone.deliverables.length > 0 &&
            ` · ${milestone.deliverables.filter((d) => d.status === 'valide').length}/${milestone.deliverables.length} livrable${milestone.deliverables.length > 1 ? 's' : ''} validé${milestone.deliverables.length > 1 ? 's' : ''}`}
        </span>
        {milestone.description !== '' && <span className="text-[13px] text-[#4b4b4f]">{milestone.description}</span>}
      </div>
    </li>
  )
}
