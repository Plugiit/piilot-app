import {
  Add01Icon,
  ArrowRight01Icon,
  Calendar03Icon,
  Flag02Icon,
  Mail01Icon,
  TelephoneIcon,
  Tick02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { motion } from 'framer-motion'
import type { ReactNode } from 'react'

import { PanelCard } from '@/components/panel-card'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { buttonVariants } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { AGENDA_ENTRY, AgendaEntry, History } from '@/features/portal/dashboard'
import { PORTAL_DELIVERABLE_STATUS, PORTAL_PROJECT_STATUS, portalTicketStatus, since } from '@/features/portal/format'
import { parseApiDate, PROGRESS_COLOR, tintOf, tracksSchedule, type StatusPill as PillTokens } from '@/features/projects/format'
import { Meter, ProjectLogo, StatusPill } from '@/features/projects/ui'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'
import type {
  PortalAction,
  PortalActivityItem,
  PortalContact,
  PortalDashboard,
  PortalDeliverable,
  PortalProject,
  PortalProjectDetail,
} from '@/types/api'

/**
 * Le corps du tableau de bord du portail, dans le vocabulaire du back-office :
 * les memes coques grises, les memes lignes de liste, les memes vignettes
 * d'etat, et a droite un panneau facon agenda. Ce qui distingue l'espace
 * client tient a ce qu'on y montre — un projet raconte etape par etape — pas
 * a un dessin a part.
 */

const LONG = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })
const SHORT = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })

const ROW = '-mx-1.5 flex items-center gap-3 rounded-[8px] px-1.5 py-2 transition-colors hover:bg-[#fafafa]'

/** Le filet entre deux lignes, porte par l'element de liste. */
const ITEM = 'border-b border-[#f3f4f4] last:border-b-0'

function progressOf(p: { status: PortalProject['status']; progress: number }) {
  return tracksSchedule(p.status) ? p.progress : 100
}

/** Etat d'une etape, en vignette. L'etape courante prend la couleur de la marque. */
const STEP: Record<'done' | 'current' | 'late' | 'upcoming', { label: string; color: string; pill: PillTokens }> = {
  done: { label: 'Atteinte', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } },
  current: { label: 'En cours', color: '#ff782b', pill: { bg: '#fff2ea', border: '#ffd9c2', text: '#b84a0c' } },
  late: { label: 'En retard', color: '#e5484d', pill: { bg: '#fdf3f3', border: '#f2d5d6', text: '#e5484d' } },
  upcoming: { label: 'À venir', color: '#8a8c91', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#4b4b4f' } },
}

// --- Colonne principale -------------------------------------------------------

export function ProjectJourney({
  data,
  focus,
  summary,
  projects,
  follow,
}: {
  data: PortalDashboard
  focus: PortalProjectDetail
  summary: PortalProject | undefined
  projects: PortalProject[]
  follow: (id: string) => void
}) {
  return (
    // La zone du projet suivi se detache des chiffres globaux par sa barre
    // d'onglets et son en-tete, pas par une carte.
    <section className="flex flex-col gap-5 pt-3">
      {projects.length > 1 && <ProjectTabs projects={projects} focusId={focus.id} follow={follow} />}
      <ProjectHeader focus={focus} summary={summary} single={projects.length === 1} />
      <StepsCard focus={focus} />
      <TicketsSection data={data} />
    </section>
  )
}

/** Au-dela, les projets passent dans le menu « autres projets ». */
const TABS_SHOWN = 4

/**
 * Les projets du client en onglets, ceux du back-office : le libelle en 16px,
 * le filet orange qui glisse de l'un a l'autre, la pastille de ce qui attend.
 * Les plus recemment actifs d'abord ; le projet suivi reste toujours visible.
 */
function ProjectTabs({ projects, focusId, follow }: { projects: PortalProject[]; focusId: string; follow: (id: string) => void }) {
  const transition = useSlideTransition()

  let shown = projects.slice(0, TABS_SHOWN)
  if (!shown.some((p) => p.id === focusId)) {
    const focused = projects.find((p) => p.id === focusId)
    if (focused !== undefined) shown = [...shown.slice(0, TABS_SHOWN - 1), focused]
  }
  const rest = projects.filter((p) => !shown.includes(p))

  return (
    <div className="flex items-center justify-between gap-3 border-b border-[#e8e8e9]">
      <nav aria-label="Vos projets" className="-mb-px flex min-w-0 items-center overflow-x-auto [scrollbar-width:none]">
        {shown.map((p) => {
          const active = p.id === focusId
          return (
            <button
              key={p.id}
              type="button"
              onClick={() => follow(p.id)}
              aria-current={active ? 'true' : undefined}
              className="relative flex shrink-0 cursor-pointer items-center gap-2 px-3.5 py-2.5"
            >
              <span
                className={cn(
                  'max-w-[220px] truncate text-[16px] whitespace-nowrap transition-colors',
                  active ? 'text-brand font-medium' : 'text-[#73757c] hover:text-[#1b1b1b]',
                )}
              >
                {p.name}
              </span>
              {p.deliverables_pending > 0 && (
                <span
                  title={`${p.deliverables_pending} livrable${p.deliverables_pending > 1 ? 's' : ''} à valider`}
                  className="bg-brand flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-[11px] font-medium text-white tabular-nums"
                >
                  {p.deliverables_pending}
                </span>
              )}
              {active && (
                <motion.span
                  aria-hidden
                  layoutId="portal-project-tab"
                  transition={transition}
                  className="bg-brand absolute right-0 bottom-0 left-0 h-[2px]"
                />
              )}
            </button>
          )
        })}
      </nav>

      {rest.length > 0 && (
        <Select value="" onValueChange={follow}>
          <SelectTrigger size="sm" className="h-8 shrink-0 bg-white text-[13px]" aria-label="Autres projets">
            <SelectValue placeholder={`${rest.length} autre${rest.length > 1 ? 's' : ''} projet${rest.length > 1 ? 's' : ''}`} />
          </SelectTrigger>
          <SelectContent align="end">
            {rest.map((p) => (
              <SelectItem key={p.id} value={p.id}>
                {p.name}
                {p.deliverables_pending > 0 && ` · ${p.deliverables_pending} à valider`}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </div>
  )
}

/** Titre de section a plat, celui du panneau de droite. */
function SectionTitle({ title, action }: { title: string; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2 pb-1">
      <h2 className="text-[16px] font-medium text-[#111]">{title}</h2>
      {action}
    </div>
  )
}

/** Lien « tout voir » du coin d'une carte, celui de « Mon travail ». */
function SeeAll({ children, ...link }: { children: ReactNode; to: '/client/projets' | '/client/tickets' }) {
  return (
    <Link {...link} className="flex items-center gap-0.5 text-[12px] text-[#73757c] transition-colors hover:text-[#1b1b1b]">
      {children}
      <HugeiconsIcon icon={ArrowRight01Icon} size={14} strokeWidth={1.8} />
    </Link>
  )
}

function Empty({ children }: { children: ReactNode }) {
  return <p className="py-6 text-center text-[13px] text-[#73757c]">{children}</p>
}

/**
 * Le projet suivi : son nom en grand, son etat, et quatre reperes lus comme
 * des chiffres. C'est la taille et l'air autour qui le detachent, pas un cadre.
 */
function ProjectHeader({ focus, summary, single }: { focus: PortalProjectDetail; summary: PortalProject | undefined; single: boolean }) {
  const status = PORTAL_PROJECT_STATUS[focus.status]
  const due = parseApiDate(focus.due_on)
  const progress = progressOf(focus)

  const facts = [
    { label: 'Livraison estimée', value: tracksSchedule(focus.status) ? (due === null ? 'À venir' : LONG.format(due)) : 'Site en ligne' },
    { label: 'Avancement', value: `${progress} %`, bar: progress },
    {
      label: 'Tâches terminées',
      value: focus.next_step.tasks_total === 0 ? '—' : `${focus.next_step.tasks_done} / ${focus.next_step.tasks_total}`,
    },
    { label: 'Dernière activité', value: summary === undefined ? '—' : since(summary.last_activity_at) },
  ]

  return (
    <header className="flex flex-col gap-5">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex min-w-0 items-center gap-4">
          <ProjectLogo url={focus.logo_url} size={56} className="rounded-[12px]" />
          <div className="flex min-w-0 flex-col gap-1.5">
            {single && <span className="text-[13px] text-[#73757c]">Votre projet</span>}
            <h1 className="font-heading truncate text-[30px] leading-[1.15] font-medium text-[#1b1b1b]">{focus.name}</h1>
            <div className="flex min-w-0 items-center gap-2">
              <StatusPill label={status.label} color={status.color} pill={status.pill} />
              {focus.description !== '' && <span className="truncate text-[14px] text-[#73757c]">{focus.description}</span>}
            </div>
          </div>
        </div>
        <Link to="/client/projets/$id" params={{ id: focus.id }} className={buttonVariants({ variant: 'outline', size: 'sm' })}>
          Ouvrir le projet
          <HugeiconsIcon icon={ArrowRight01Icon} size={14} strokeWidth={1.8} />
        </Link>
      </div>

      {/* Les reperes, separes d'un filet comme les compteurs du temps
          facturable, mais a la taille des chiffres des tuiles. */}
      <dl className="grid grid-cols-2 gap-y-4 md:grid-cols-4">
        {facts.map((f, i) => (
          <div
            key={f.label}
            className={cn(
              'flex min-w-0 flex-col gap-1 px-4',
              i % 2 === 1 && 'border-l border-[#ebebeb]',
              i >= 2 && 'md:border-l md:border-[#ebebeb]',
              i === 0 && 'pl-0',
              i === 2 && 'max-md:pl-0',
            )}
          >
            <dt className="truncate text-[13px] text-[#73757c]">{f.label}</dt>
            <dd className="font-heading truncate text-[20px] leading-tight font-medium text-[#1b1b1b] tabular-nums">{f.value}</dd>
            {f.bar !== undefined && <Meter ratio={f.bar} color={PROGRESS_COLOR} className="mt-1 max-w-[160px]" />}
          </div>
        ))}
      </dl>
    </header>
  )
}

/** Les etapes du projet, les livrables ranges sous l'etape qu'ils font avancer. */
function StepsCard({ focus }: { focus: PortalProjectDetail }) {
  const current = focus.milestones.findIndex((m) => m.state !== 'done')

  const byMilestone = new Map<string, PortalDeliverable[]>()
  const loose: PortalDeliverable[] = []
  for (const d of focus.deliverables) {
    if (d.milestone === null) loose.push(d)
    else byMilestone.set(d.milestone, [...(byMilestone.get(d.milestone) ?? []), d])
  }
  const validated = focus.deliverables.filter((d) => d.status === 'valide').length

  return (
    <PanelCard
      icon={Flag02Icon}
      title="ÉTAPES ET LIVRABLES"
      contentClassName="p-3"
      action={
        focus.deliverables.length > 0 && (
          <span className="text-[12px] text-[#73757c] tabular-nums">
            {validated} validé{validated > 1 ? 's' : ''} sur {focus.deliverables.length}
          </span>
        )
      }
    >
      {focus.milestones.length === 0 && loose.length === 0 ? (
        <Empty>Les étapes du projet n’ont pas encore été posées.</Empty>
      ) : (
        <div className="flex flex-col gap-4">
          {focus.milestones.length > 0 && (
            <ol className="flex flex-col">
              {focus.milestones.map((m, i) => {
                const state = m.state === 'done' ? 'done' : m.state === 'late' ? 'late' : i === current ? 'current' : 'upcoming'
                const step = STEP[state]
                const due = parseApiDate(m.due_on)
                const items = byMilestone.get(m.title) ?? []
                const last = i === focus.milestones.length - 1

                return (
                  <li key={m.id} className="relative flex gap-3 pb-4 last:pb-0">
                    {/* Le rail : un filet qui relie les reperes, vert sur ce
                        qui est atteint. */}
                    {!last && (
                      <span
                        aria-hidden
                        className="absolute top-6 bottom-0 left-[11px] w-px"
                        style={{ backgroundColor: state === 'done' ? '#afe3ca' : '#ebebeb' }}
                      />
                    )}
                    <StepMarker state={state} index={i + 1} />

                    <div className="flex min-w-0 flex-1 flex-col gap-1">
                      <div className="flex min-h-6 flex-wrap items-center gap-x-2 gap-y-1">
                        <span className={cn('text-[14px] font-medium', state === 'done' ? 'text-[#73757c]' : 'text-[#1b1b1b]')}>
                          {m.title}
                        </span>
                        <StatusPill label={step.label} color={step.color} pill={step.pill} />
                        <span className={cn('ml-auto text-[12px] tabular-nums', state === 'late' ? 'font-medium text-[#e5484d]' : 'text-[#73757c]')}>
                          {state === 'done' && m.completed_at !== null
                            ? `Atteinte le ${SHORT.format(new Date(m.completed_at))}`
                            : due === null
                              ? 'Date à venir'
                              : `Prévue le ${LONG.format(due)}`}
                        </span>
                      </div>
                      {m.description !== '' && <p className="text-[13px] text-[#73757c]">{m.description}</p>}
                      {items.length > 0 && (
                        <ul className="flex flex-col pt-1">
                          {items.map((d) => (
                            <DeliverableRow key={d.id} deliverable={d} />
                          ))}
                        </ul>
                      )}
                    </div>
                  </li>
                )
              })}
            </ol>
          )}

          {loose.length > 0 && (
            <section className="flex flex-col">
              <h3 className="pb-1 text-[12px] font-medium tracking-wide text-[#73757c] uppercase">Hors étape</h3>
              <ul className="flex flex-col">
                {loose.map((d) => (
                  <DeliverableRow key={d.id} deliverable={d} />
                ))}
              </ul>
            </section>
          )}
        </div>
      )}
    </PanelCard>
  )
}

function StepMarker({ state, index }: { state: keyof typeof STEP; index: number }) {
  if (state === 'done') {
    return (
      <span className="relative z-10 flex size-6 shrink-0 items-center justify-center rounded-full border border-[#afe3ca] bg-[#dcf7ea] text-[#006f1f]">
        <HugeiconsIcon icon={Tick02Icon} size={14} strokeWidth={2} />
      </span>
    )
  }

  return (
    <span
      className={cn(
        'relative z-10 flex size-6 shrink-0 items-center justify-center rounded-full border bg-white text-[12px] font-semibold tabular-nums',
        state === 'current' && 'border-[#ffd9c2] text-[#b84a0c]',
        state === 'late' && 'border-[#f2d5d6] text-[#e5484d]',
        state === 'upcoming' && 'border-[#e3e4e6] text-[#a2a3a7]',
      )}
    >
      {index}
    </span>
  )
}

/** Un livrable : le titre, sa version, sa vignette, et le geste quand il attend. */
function DeliverableRow({ deliverable: d }: { deliverable: PortalDeliverable }) {
  const status = PORTAL_DELIVERABLE_STATUS[d.status]
  const waiting = d.status === 'en_attente'

  return (
    <li className={ITEM}>
      <Link to="/client/livrables/$id" params={{ id: d.id }} className={ROW}>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="truncate text-[14px] text-[#1b1b1b]">{d.title}</span>
          <span className="flex items-center gap-1.5 truncate text-[12px] text-[#8d8d8d]">
            <span aria-hidden className="size-1.5 shrink-0 rounded-full sm:hidden" style={{ backgroundColor: status.color }} />
            Version {d.version} · déposée le {SHORT.format(new Date(d.submitted_at))}
          </span>
        </span>
        <StatusPill label={status.label} color={status.color} pill={status.pill} className="hidden sm:inline-flex" />
        {/* Le geste n'apparait que quand le livrable attend le client. La
            ligne entiere reste le lien ; la reserve aligne les vignettes. */}
        <span className={cn('shrink-0 justify-end sm:flex sm:w-[76px]', waiting ? 'flex' : 'hidden')}>
          {waiting && <span className={buttonVariants({ size: 'xs' })}>Répondre</span>}
        </span>
      </Link>
    </li>
  )
}

/** Les interlocuteurs, en tete du panneau de droite, et de quoi les joindre. */
function Contacts({ team }: { team: PortalContact[] }) {
  return (
    <section className="flex flex-col">
      <SectionTitle title="Vos interlocuteurs" />
      {team.length === 0 ? (
        <p className="text-[14px] text-[#111]/50">L’équipe n’est pas encore constituée.</p>
      ) : (
        <ul className="flex flex-col">
          {team.map((c) => (
            <ContactRow key={c.id} contact={c} />
          ))}
        </ul>
      )}
    </section>
  )
}

function ContactRow({ contact: c }: { contact: PortalContact }) {
  const links = [
    { href: `mailto:${c.email}`, icon: Mail01Icon, label: 'Écrire', external: false },
    ...(c.phone !== '' ? [{ href: `tel:${c.phone}`, icon: TelephoneIcon, label: 'Appeler', external: false }] : []),
    ...(c.booking_url !== '' ? [{ href: c.booking_url, icon: Calendar03Icon, label: 'Prendre rendez-vous', external: true }] : []),
  ]

  return (
    <li className={cn(ROW, ITEM)}>
      <Avatar className="size-9 rounded-[6px] after:rounded-[6px]">
        {c.avatar_url != null && c.avatar_url !== '' && <AvatarImage src={c.avatar_url} alt="" className="rounded-[6px]" />}
        <AvatarFallback className="rounded-[6px] text-[12px] font-semibold text-[#0f172a]" style={{ backgroundColor: tintOf(c.id) }}>
          {c.initials}
        </AvatarFallback>
      </Avatar>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-[14px] text-[#1b1b1b]">
          {c.firstname} {c.lastname}
        </span>
        <span className="truncate text-[12px] text-[#8d8d8d]">{c.role}</span>
      </span>
      <span className="flex shrink-0 items-center gap-1">
        {links.map((l) => (
          <a
            key={l.label}
            href={l.href}
            title={l.label}
            aria-label={l.label}
            {...(l.external ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
            className={buttonVariants({ variant: 'outline', size: 'icon-sm' })}
          >
            <HugeiconsIcon icon={l.icon} size={16} strokeWidth={1.6} />
          </a>
        ))}
      </span>
    </li>
  )
}

/** Les demandes du projet suivi, comme les tickets de « Mon travail ». */
function TicketsSection({ data }: { data: PortalDashboard }) {
  return (
    <section className="flex flex-col">
      <SectionTitle title="Demandes" action={<SeeAll to="/client/tickets">Toutes les demandes</SeeAll>} />
      {data.focus_tickets.length === 0 ? (
        <div className="flex flex-col items-start gap-3 py-2">
          <p className="text-[14px] text-[#111]/50">Aucune demande sur ce projet.</p>
          <Link to="/client/tickets/nouvelle" className={buttonVariants({ variant: 'outline', size: 'sm' })}>
            <HugeiconsIcon icon={Add01Icon} size={14} strokeWidth={1.8} />
            Nouvelle demande
          </Link>
        </div>
      ) : (
        <ul className="flex flex-col">
          {data.focus_tickets.slice(0, 5).map((t) => {
            const s = portalTicketStatus(t.status)
            return (
              <li key={t.id} className={ITEM}>
                <Link to="/client/tickets/$id" params={{ id: t.id }} className={ROW}>
                  <span className="w-[44px] shrink-0 text-[12px] text-[#a2a3a7] tabular-nums">#{t.numero}</span>
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="truncate text-[14px] text-[#1b1b1b]">{t.subject}</span>
                    <span className="truncate text-[12px] text-[#8d8d8d]">Mise à jour le {SHORT.format(new Date(t.updated_at))}</span>
                  </span>
                  <StatusPill label={s.label} color={s.color} pill={s.pill} />
                </Link>
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}

// --- Panneau de droite --------------------------------------------------------

/**
 * Le panneau de droite, celui de l'agenda du back-office : ce qui attend le
 * client, ses interlocuteurs, ce qui l'attend, puis l'historique du projet
 * suivi.
 */
export function PortalAside({
  actions,
  activity,
  focus,
  className,
}: {
  actions: PortalAction[]
  activity: PortalActivityItem[]
  focus: PortalProjectDetail | null
  className?: string
}) {
  return (
    <aside className={cn('flex flex-col gap-3 bg-white p-4', className)}>
      {focus !== null && (
        <div className="border-surface-sunken border-b pb-3">
          <Contacts team={focus.team} />
        </div>
      )}

      <div className="flex items-center justify-between gap-2">
        <h2 className="text-[16px] font-medium text-[#111]">À faire de votre côté</h2>
        {actions.length > 0 && (
          <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-[#ff782b] px-1.5 text-[11px] font-semibold text-white tabular-nums">
            {actions.length}
          </span>
        )}
      </div>

      <div className="border-surface-sunken flex flex-col gap-4 border-b pb-4">
        {actions.length === 0 ? (
          <p className="text-[14px] text-[#111]/50">Rien ne vous attend : l’agence avance.</p>
        ) : (
          <>
            {actions.slice(0, 5).map((a) => (
              <ActionEntry key={`${a.kind}-${a.ref_id}`} action={a} />
            ))}
            {actions.length > 5 && (
              <p className="text-[12px] text-[#8d8d8d]">Et {actions.length - 5} autres sur vos projets.</p>
            )}
          </>
        )}
      </div>

      {focus !== null && <History key={focus.id} items={activity} />}
    </aside>
  )
}

function ActionEntry({ action: a }: { action: PortalAction }) {
  const color = a.overdue ? '#e5484d' : '#ff782b'
  const body = (
    <AgendaEntry
      color={color}
      label={a.kind === 'deliverable' ? 'À VALIDER' : 'VOTRE RETOUR'}
      meta={a.since === null ? undefined : since(a.since)}
      title={a.kind === 'deliverable' ? a.title : `L’agence attend votre retour`}
      detail={a.project.name}
    />
  )

  return a.kind === 'deliverable' ? (
    <Link to="/client/livrables/$id" params={{ id: a.ref_id }} className={AGENDA_ENTRY}>
      {body}
    </Link>
  ) : (
    <Link to="/client/projets/$id" params={{ id: a.ref_id }} className={AGENDA_ENTRY}>
      {body}
    </Link>
  )
}
