import {
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Calendar03Icon,
  ChartLineData01Icon,
  CheckmarkSquare02Icon,
  Download04Icon,
  File01Icon,
  Flag02Icon,
  Mail01Icon,
  PackageIcon,
  TelephoneIcon,
  UserMultipleIcon,
  Wrench01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useRouterState } from '@tanstack/react-router'
import { motion } from 'framer-motion'
import type { ReactNode } from 'react'

import { buttonVariants } from '@/components/ui/button'
import { MILESTONE_STATE } from '@/features/milestones/format'
import { portalFileUrl, portalProjectQuery } from '@/features/portal/api'
import { fileSize, PORTAL_DELIVERABLE_STATUS, PORTAL_PROJECT_STATUS, since } from '@/features/portal/format'
import { parseApiDate, tracksSchedule } from '@/features/projects/format'
import { Avatars, ProgressBar, ProjectLogo, StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'
import type { Milestone, PortalContact, PortalDeliverable, PortalProjectDetail } from '@/types/api'

/**
 * Un projet vu par le client : ou il en est, ce qui attend sa reponse, les
 * etapes, les livrables et les documents que l'agence a partages.
 *
 * Au dessin de la fiche projet de l'admin : l'en-tete, une fiche en lignes
 * « libelle / valeur », puis des onglets. Les livrables a valider ont le
 * premier, ouvert d'office : c'est ce que le client a a faire.
 */
export const Route = createFileRoute('/client/projets/$id')({
  loader: ({ context, params }) => context.queryClient.ensureQueryData(portalProjectQuery(params.id)),
  component: ClientProjectPage,
  errorComponent: ({ error }) => <NotFound message={error instanceof HttpError && error.status === 404 ? undefined : error.message} />,
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
    <Link to="/client/projets" className="flex items-center gap-1 text-[14px] text-[#73757c] hover:text-[#1b1b1b]">
      <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
      Mes projets
    </Link>
  )
}

/** Les onglets de la page, chacun a son ancre : « #a-valider » ouvre le sien. */
type TabKey = 'a-valider' | 'etapes' | 'livrables' | 'documents' | 'interlocuteurs'

function ClientProjectPage() {
  const { id } = Route.useParams()
  const { data: project } = useQuery(portalProjectQuery(id))
  const hash = useRouterState({ select: (state) => state.location.hash })
  const transition = useSlideTransition()

  if (project === undefined) return null

  const status = PORTAL_PROJECT_STATUS[project.status]
  const start = parseApiDate(project.starts_on)
  const due = parseApiDate(project.due_on)
  const toValidate = project.deliverables.filter((d) => d.status === 'en_attente')
  const others = project.deliverables.filter((d) => d.status !== 'en_attente')
  const ongoing = tracksSchedule(project.status)

  const tabs: { key: TabKey; label: string; icon: IconSvgElement; badge?: number }[] = [
    ...(toValidate.length > 0 ? [{ key: 'a-valider' as const, label: 'À valider', icon: CheckmarkSquare02Icon, badge: toValidate.length }] : []),
    { key: 'etapes', label: 'Étapes', icon: Flag02Icon },
    { key: 'livrables', label: 'Livrables', icon: PackageIcon },
    { key: 'documents', label: 'Documents', icon: File01Icon },
    { key: 'interlocuteurs', label: 'Interlocuteurs', icon: UserMultipleIcon },
  ]
  const tab: TabKey = tabs.some((t) => t.key === hash) ? (hash as TabKey) : (tabs[0]?.key ?? 'etapes')

  return (
    <div className="-mx-4 -mt-4 flex flex-col sm:-mt-5">
      <div className="flex flex-col gap-5 p-4">
        <BackLink />

        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex min-w-0 flex-col gap-1">
            <div className="flex min-w-0 items-center gap-3">
              <ProjectLogo url={project.logo_url} size={40} bare className="rounded-[8px]" />
              <h1 className="truncate text-[24px] leading-[1.5] font-medium text-[#1b1b1b]">{project.name}</h1>
            </div>
            {project.description !== '' && <p className="text-[14px] leading-[1.5] text-[#73757c]">{project.description}</p>}
          </div>

          <div className="flex shrink-0 flex-wrap items-center gap-4">
            {project.team.length > 0 && <Avatars people={project.team} max={6} size={32} />}
            <div className="flex items-center gap-2">
              {project.maintenance && (
                <Link to="/client/projets/$id/maintenance" params={{ id: project.id }} className={cn(buttonVariants({ variant: 'outline', size: 'lg' }), 'border-[#d0d1d3]')}>
                  <HugeiconsIcon icon={Wrench01Icon} size={18} strokeWidth={1.8} />
                  Maintenance
                </Link>
              )}
              {project.statistics && (
                <Link
                  to="/client/projets/$id/statistiques"
                  params={{ id: project.id }}
                  className={buttonVariants({ size: 'lg' })}
                >
                  <HugeiconsIcon icon={ChartLineData01Icon} size={18} strokeWidth={1.8} />
                  Statistiques du site
                </Link>
              )}
            </div>
          </div>
        </div>

        <div className="flex flex-col gap-4">
          <MetaRow label="Statut">
            <StatusPill label={status.label} color={status.color} pill={status.pill} />
          </MetaRow>

          {ongoing ? (
            <>
              <MetaRow label="Début">
                <p className="text-[14px] text-[#1b1b1b]">{start === null ? 'Date à venir' : LONG.format(start)}</p>
              </MetaRow>
              <MetaRow label="Livraison">
                <p className="text-[14px] text-[#1b1b1b]">{due === null ? 'Date à venir' : `Prévue le ${LONG.format(due)}`}</p>
              </MetaRow>
              <MetaRow label="Avancement">
                <ProgressBar value={project.progress} />
                <AgencySide project={project} />
              </MetaRow>
              <MetaRow label="Prochain jalon">
                <NextMilestone project={project} />
              </MetaRow>
            </>
          ) : (
            <MetaRow label="Votre site">
              <p className="text-[14px] text-[#1b1b1b]">En ligne, hébergé et suivi par l’agence. Une question ou un souci : ouvrez un ticket.</p>
            </MetaRow>
          )}

          <MetaRow label="De votre côté">
            {toValidate.length === 0 ? (
              <p className="text-[14px] text-[#1b1b1b]">Rien n’attend votre réponse</p>
            ) : (
              <Link to="." hash="a-valider" className="text-brand text-[14px] font-medium hover:underline">
                {toValidate.length === 1 ? 'Un livrable à valider' : `${toValidate.length} livrables à valider`}
              </Link>
            )}
          </MetaRow>

          {project.last_deployment !== null && (
            <MetaRow label="Mise en ligne">
              <p className="text-[14px] text-[#1b1b1b]">
                {LONG.format(new Date(project.last_deployment.deployed_at))}
                {project.last_deployment.name !== '' && <span className="text-[#73757c]"> · {project.last_deployment.name}</span>}
              </p>
            </MetaRow>
          )}
        </div>
      </div>

      <nav aria-label="Sections du projet" className="flex shrink-0 overflow-x-auto border-b border-[#e8e8e9] px-4 [scrollbar-width:none]">
        {tabs.map((t) => {
          const active = t.key === tab
          return (
            <Link key={t.key} to="." hash={t.key} replace resetScroll={false} className="relative flex shrink-0 items-center gap-2 px-3.5 py-2">
              <HugeiconsIcon
                icon={t.icon}
                size={20}
                strokeWidth={1.6}
                className={cn('shrink-0 transition-colors', active ? 'text-brand' : 'text-[#73757c]')}
              />
              <span className={cn('text-[16px] whitespace-nowrap transition-colors', active ? 'text-brand font-medium' : 'text-[#73757c]')}>
                {t.label}
              </span>
              {t.badge !== undefined && (
                <span className="bg-brand flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-[11px] font-medium text-white tabular-nums">
                  {t.badge}
                </span>
              )}
              {active && (
                <motion.span
                  aria-hidden
                  layoutId="client-project-tab"
                  transition={transition}
                  className="bg-brand absolute right-0 -bottom-px left-0 h-[2px]"
                />
              )}
            </Link>
          )
        })}
      </nav>

      <div className="p-4">
        {tab === 'a-valider' && (
          <List>
            {toValidate.map((deliverable) => (
              <DeliverableRow key={deliverable.id} deliverable={deliverable} highlight />
            ))}
          </List>
        )}

        {tab === 'etapes' &&
          (project.milestones.length === 0 ? (
            <Empty>Les étapes du projet n’ont pas encore été posées.</Empty>
          ) : (
            <ol className="flex flex-col rounded-[12px] border border-[#e8e8e9] p-2">
              {project.milestones.map((milestone, index) => (
                <MilestoneLine key={milestone.id} milestone={milestone} last={index === project.milestones.length - 1} />
              ))}
            </ol>
          ))}

        {tab === 'livrables' &&
          (others.length === 0 ? (
            <Empty>{toValidate.length > 0 ? 'Pas d’autre livrable pour l’instant.' : 'Aucun livrable n’a encore été déposé.'}</Empty>
          ) : (
            <List>
              {others.map((deliverable) => (
                <DeliverableRow key={deliverable.id} deliverable={deliverable} />
              ))}
            </List>
          ))}

        {tab === 'documents' &&
          (project.files.length === 0 ? (
            <Empty>Aucun document partagé pour l’instant.</Empty>
          ) : (
            <List>
              {project.files.map((file) => (
                <li key={file.id} className="border-b border-[#f3f4f4] last:border-b-0">
                  <a href={portalFileUrl(file.id)} className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-[#fafafa]">
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
            </List>
          ))}

        {tab === 'interlocuteurs' &&
          (project.team.length === 0 ? (
            <Empty>Vos interlocuteurs apparaîtront ici dès que l’équipe du projet sera constituée.</Empty>
          ) : (
            <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {project.team.map((contact) => (
                <ContactCard key={contact.id} contact={contact} />
              ))}
            </ul>
          ))}
      </div>
    </div>
  )
}

/** Une ligne de la fiche, comme sur l'admin : le libelle, puis la valeur. */
function MetaRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-4">
      <p className="w-[110px] shrink-0 text-[14px] text-[#73757c]">{label}</p>
      <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">{children}</div>
    </div>
  )
}

/** Le prochain jalon : son nom, sa date, en retard le cas echeant. */
function NextMilestone({ project }: { project: PortalProjectDetail }) {
  const milestone = project.next_step.milestone
  if (milestone === null) {
    return <p className="text-[14px] text-[#1b1b1b]">{project.milestones.length === 0 ? 'Pas encore posé' : 'Tous les jalons sont atteints'}</p>
  }
  const due = parseApiDate(milestone.due_on)
  const state = MILESTONE_STATE[milestone.state]
  return (
    <>
      <p className="text-[14px] text-[#1b1b1b]">
        {milestone.title}
        <span className="text-[#73757c]"> · {due === null ? 'date à venir' : LONG.format(due)}</span>
      </p>
      {milestone.state === 'late' && <StatusPill label={state.label} color={state.color} pill={state.pill} />}
    </>
  )
}

/** Ou en est l'equipe : les taches du jalon en cours. */
function AgencySide({ project }: { project: PortalProjectDetail }) {
  const step = project.next_step
  return (
    <p className="text-[14px] text-[#73757c]">
      {step.tasks_total === 0
        ? 'Le travail n’a pas encore commencé'
        : `${step.tasks_done} tâche${step.tasks_done > 1 ? 's' : ''} sur ${step.tasks_total} terminée${step.tasks_done > 1 ? 's' : ''}`}
    </p>
  )
}

/** Une liste a filets, dans un cadre : livrables et documents. */
function List({ children }: { children: ReactNode }) {
  return <ul className="flex flex-col overflow-hidden rounded-[12px] border border-[#e8e8e9]">{children}</ul>
}

/** Un interlocuteur : qui, pour quoi, et comment le joindre. */
function ContactCard({ contact }: { contact: PortalContact }) {
  const name = `${contact.firstname} ${contact.lastname}`.trim()

  return (
    <li className="flex items-start gap-3 rounded-[12px] border border-[#e8e8e9] bg-white p-4">
      <Avatars people={[contact]} max={1} size={40} />
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex flex-col">
          <span className="truncate text-[15px] font-medium text-[#1b1b1b]">{name}</span>
          <span className="text-[13px] text-[#8d8d8d]">{contact.role}</span>
        </div>
        <div className="flex flex-col gap-1 text-[14px]">
          <a href={`mailto:${contact.email}`} className="flex items-center gap-1.5 text-[#4b4b4f] hover:text-[#1b1b1b] hover:underline">
            <HugeiconsIcon icon={Mail01Icon} size={15} strokeWidth={1.8} className="shrink-0 text-[#8d8d8d]" />
            <span className="truncate">{contact.email}</span>
          </a>
          {contact.phone !== '' && (
            <a href={`tel:${contact.phone}`} className="flex items-center gap-1.5 text-[#4b4b4f] hover:text-[#1b1b1b] hover:underline">
              <HugeiconsIcon icon={TelephoneIcon} size={15} strokeWidth={1.8} className="shrink-0 text-[#8d8d8d]" />
              {contact.phone}
            </a>
          )}
        </div>
        {contact.booking_url !== '' && (
          <a
            href={contact.booking_url}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-1 flex w-fit items-center gap-1.5 rounded-full border border-[#e8e8e9] px-3 py-1.5 text-[13px] font-medium text-[#1b1b1b] transition-colors hover:bg-[#fafafa]"
          >
            <HugeiconsIcon icon={Calendar03Icon} size={15} strokeWidth={1.8} />
            Prendre rendez-vous
          </a>
        )}
      </div>
    </li>
  )
}

function Empty({ children }: { children: ReactNode }) {
  return <p className="rounded-[12px] border border-dashed border-[#e3e4e6] bg-white px-4 py-5 text-[14px] text-[#8d8d8d]">{children}</p>
}

function DeliverableRow({ deliverable, highlight = false }: { deliverable: PortalDeliverable; highlight?: boolean }) {
  const status = PORTAL_DELIVERABLE_STATUS[deliverable.status]

  return (
    <li className="border-b border-[#f3f4f4] last:border-b-0">
      <Link
        to="/client/livrables/$id"
        params={{ id: deliverable.id }}
        className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-[#fafafa]"
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
    </li>
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
          <span className={cn('text-[15px] text-[#1b1b1b]', milestone.state === 'done' && 'text-[#73757c]')}>{milestone.title}</span>
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
