import {
  Alert02Icon,
  ArrowRight01Icon,
  Calendar03Icon,
  CheckmarkCircle02Icon,
  ChartLineData01Icon,
  Clock01Icon,
  CustomerSupportIcon,
  File01Icon,
  Mail01Icon,
  MessageQuestionIcon,
  TelephoneIcon,
  Tick02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'

import { PanelCard } from '@/components/panel-card'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { buttonVariants } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { portalDashboardQuery } from '@/features/portal/api'
import { History, StatTile } from '@/features/portal/dashboard'
import { PORTAL_PROJECT_STATUS, since } from '@/features/portal/format'
import { DONE_COLOR, parseApiDate, PROGRESS_COLOR, tintOf } from '@/features/projects/format'
import { Meter, ProgressBar, ProjectLogo, StatusPill } from '@/features/projects/ui'
import { cn } from '@/lib/utils'
import type { AudienceSite, PortalAction, PortalContact, PortalDashboard, PortalJourneyStep, PortalProject, PortalUptimeSite } from '@/types/api'

/**
 * L'accueil du portail, pense pour un client qui passe de temps en temps : ce
 * qu'on attend de lui d'abord, une seule fois et avec le bouton pour le faire ;
 * puis ou en est chaque projet, raconte en etapes plutot qu'en chiffres.
 */

const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })
const LONG = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long' })

const NUMBER = new Intl.NumberFormat('fr-FR')
const PERCENT = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 2 })

/** Ce que les modules savent des sites d'un projet : audience et disponibilite. */
export interface ProjectSite {
  audience?: AudienceSite
  uptime: PortalUptimeSite[]
}

/**
 * Le mois en quatre chiffres, ceux qui parlent a un client : ce qu'il a
 * valide, ce que l'agence a resolu, et — quand ses sites sont suivis — leur
 * audience et leur disponibilite. Sans module, la prochaine echeance et les
 * projets en cours prennent leur place.
 */
export function MonthTiles({ data, audience, uptime }: { data: PortalDashboard; audience: AudienceSite[]; uptime: PortalUptimeSite[] }) {
  const open = data.tickets.received + data.tickets.in_progress + data.tickets.ready
  const visitors = audience.reduce((sum, a) => sum + a.visitors, 0)
  const rated = uptime.filter((u) => u.uptime_30d !== null)
  const avg = rated.length === 0 ? null : rated.reduce((sum, u) => sum + (u.uptime_30d ?? 0), 0) / rated.length
  const down = uptime.filter((u) => u.status === 'down').length
  const next = data.next_milestone
  const due = parseApiDate(next?.due_on)

  return (
    <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
      <StatTile
        label="Livrables validés"
        value={data.validated_this_month}
        footer={data.deliverables_pending > 0 ? `ce mois-ci · ${data.deliverables_pending} à valider` : 'ce mois-ci'}
      />
      <StatTile label="Demandes résolues" value={data.tickets_resolved_this_month} footer={`ce mois-ci · ${open} en cours`} />
      {audience.length > 0 ? (
        <StatTile
          label="Visiteurs de vos sites"
          value={NUMBER.format(visitors)}
          footer={`30 derniers jours · ${audience.length} site${audience.length > 1 ? 's' : ''}`}
        />
      ) : (
        <StatTile label="Projets en cours" value={data.active_projects} footer={`sur ${data.projects.length} projet${data.projects.length > 1 ? 's' : ''}`} />
      )}
      {avg !== null ? (
        <StatTile
          label="Disponibilité"
          value={`${PERCENT.format(avg)} %`}
          footer={down > 0 ? `${down} site${down > 1 ? 's' : ''} hors ligne` : '30 derniers jours · tout est en ligne'}
        />
      ) : (
        <StatTile
          label="Prochaine échéance"
          value={due === null ? '—' : DAY.format(due)}
          footer={next === null ? 'Aucune étape à venir' : next.title}
        />
      )}
    </div>
  )
}

/** Au-dela, la boite « A faire » se replie : la page reste lisible d'un coup d'oeil. */
const ACTIONS_SHOWN = 4

/** La phrase d'accueil : ce qui attend, ou que tout est a jour. */
export function Greeting({ firstname, data }: { firstname: string; data: PortalDashboard }) {
  const count = data.actions.length
  const next = data.next_milestone
  const due = parseApiDate(next?.due_on)
  return (
    <header className="flex flex-col gap-1.5">
      <h1 className="font-heading text-[28px] leading-tight font-medium text-[#1b1b1b]">
        Bonjour{firstname !== '' && ` ${firstname}`}
      </h1>
      <p className="text-[16px] text-[#73757c]">
        {count === 0 ? (
          'Tout est à jour, rien n’attend votre réponse.'
        ) : (
          <span className="font-medium text-[#b84a0c]">
            {count === 1 ? 'Une chose vous attend.' : `${count} choses vous attendent.`}
          </span>
        )}
        {next !== null && (
          <>
            {' '}
            Prochaine étape : {next.title}
            {due !== null && `, le ${LONG.format(due)}`}
            {data.projects.length > 1 && ` (${next.project.name})`}.
          </>
        )}
      </p>
    </header>
  )
}

/**
 * La boite « A faire » : une ligne par chose a faire, le bouton pour la faire
 * au bout. Le seul endroit de la page ou ces actions apparaissent.
 */
export function ActionsBox({ actions }: { actions: PortalAction[] }) {
  const [all, setAll] = useState(false)
  const shown = all ? actions : actions.slice(0, ACTIONS_SHOWN)

  if (actions.length === 0) {
    return (
      <div className="flex items-center gap-3 rounded-[12px] border border-[#cdeedb] bg-[#f3fbf6] px-4 py-3.5">
        <HugeiconsIcon icon={CheckmarkCircle02Icon} size={22} strokeWidth={1.8} className="shrink-0 text-[#1a7f4b]" />
        <p className="text-[14px] text-[#1b1b1b]">
          <span className="font-medium">Tout est à jour.</span>{' '}
          <span className="text-[#4b4b4f]">Vous serez prévenu par e-mail dès que l’agence aura besoin de vous.</span>
        </p>
      </div>
    )
  }

  return (
    <PanelCard
      icon={Tick02Icon}
      title="À FAIRE"
      action={
        <span className="bg-brand flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-[11px] font-semibold text-white tabular-nums">
          {actions.length}
        </span>
      }
    >
      <ul className="flex flex-col">
        {shown.map((a) => (
          <li key={`${a.kind}-${a.ref_id}`} className="border-b border-[#f3f4f4] last:border-b-0">
            <ActionRow action={a} />
          </li>
        ))}
      </ul>
      {actions.length > ACTIONS_SHOWN && (
        <button
          type="button"
          onClick={() => setAll((v) => !v)}
          className="mt-1 self-start rounded-[6px] px-1.5 py-1 text-[13px] font-medium text-[#4b4b4f] transition-colors hover:bg-[#f3f4f4] hover:text-[#1b1b1b]"
        >
          {all ? 'Afficher moins' : `Afficher les ${actions.length - ACTIONS_SHOWN} autres`}
        </button>
      )}
    </PanelCard>
  )
}

function ActionRow({ action: a }: { action: PortalAction }) {
  const deliverable = a.kind === 'deliverable'
  const content = (
    <>
      <span
        aria-hidden
        className={cn(
          'flex size-10 shrink-0 items-center justify-center rounded-[10px]',
          a.overdue ? 'bg-[#fdecec] text-[#e5484d]' : 'bg-[#fff2ea] text-[#ff782b]',
        )}
      >
        <HugeiconsIcon icon={deliverable ? File01Icon : MessageQuestionIcon} size={20} strokeWidth={1.6} />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-[15px] text-[#1b1b1b]">{deliverable ? a.title : 'L’agence attend votre retour'}</span>
        <span className={cn('flex min-w-0 items-center gap-1 text-[13px]', a.overdue ? 'text-[#e5484d]' : 'text-[#8d8d8d]')}>
          {a.overdue && <HugeiconsIcon icon={Alert02Icon} size={14} strokeWidth={1.8} className="shrink-0" />}
          <span className="truncate">
            {a.project.name}
            {a.since !== null && ` · ${deliverable ? 'déposé' : 'depuis'} ${since(a.since)}`}
          </span>
        </span>
      </span>
      <span className={cn(buttonVariants({ size: 'sm' }), 'shrink-0 max-sm:hidden')}>
        {deliverable ? 'Voir et valider' : 'Répondre'}
        <HugeiconsIcon icon={ArrowRight01Icon} size={14} strokeWidth={2} />
      </span>
      <HugeiconsIcon icon={ArrowRight01Icon} size={18} strokeWidth={1.8} className="shrink-0 text-[#a2a3a7] sm:hidden" />
    </>
  )
  const className = '-mx-1.5 flex items-center gap-3 rounded-[8px] px-1.5 py-2.5 transition-colors hover:bg-[#fafafa]'
  return deliverable ? (
    <Link to="/client/livrables/$id" params={{ id: a.ref_id }} className={className}>
      {content}
    </Link>
  ) : (
    <Link to="/client/projets/$id" params={{ id: a.ref_id }} className={className}>
      {content}
    </Link>
  )
}

/**
 * Un projet en cours, raconte en etapes : une frise ou l'etape en cours est
 * marquee « vous etes ici », et dessous, de qui depend la suite.
 */
export function JourneyCard({ project, steps, site }: { project: PortalProject; steps: PortalJourneyStep[]; site: ProjectSite }) {
  const status = PORTAL_PROJECT_STATUS[project.status]
  const current = steps.findIndex((s) => s.state !== 'done')
  const [history, setHistory] = useState(false)

  return (
    <article className="flex flex-col gap-4 rounded-[12px] border border-[#e8e8e9] bg-white p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <ProjectLogo url={project.logo_url} size={28} bare className="rounded-[6px]" />
          <Link
            to="/client/projets/$id"
            params={{ id: project.id }}
            className="truncate text-[16px] leading-[1.5] font-medium text-[#1b1b1b] hover:underline"
          >
            {project.name}
          </Link>
          <StatusPill label={status.label} color={status.color} pill={status.pill} />
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            onClick={() => setHistory(true)}
            className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'text-[#4b4b4f]')}
          >
            <HugeiconsIcon icon={Clock01Icon} size={14} strokeWidth={1.8} />
            Historique
          </button>
          <Link
            to="/client/projets/$id"
            params={{ id: project.id }}
            className={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'border-[#d0d1d3]')}
          >
            Ouvrir le projet
            <HugeiconsIcon icon={ArrowRight01Icon} size={14} strokeWidth={1.8} />
          </Link>
        </div>
      </div>

      {steps.length === 0 ? (
        <div className="flex flex-wrap items-center gap-3">
          <ProgressBar value={project.progress} />
          <span className="text-[13px] text-[#73757c]">Les étapes du projet seront bientôt posées.</span>
        </div>
      ) : (
        <Stepper steps={steps} current={current} />
      )}

      <NowLine project={project} step={current < 0 ? null : (steps[current] ?? null)} />

      <ProjectFacts project={project} site={site} />

      <HistorySheet project={project} open={history} onOpenChange={setHistory} />
    </article>
  )
}

/** La frise : un rond par etape, le trait colore jusqu'a l'etape en cours. */
function Stepper({ steps, current }: { steps: PortalJourneyStep[]; current: number }) {
  return (
    <ol className="-mx-1 flex overflow-x-auto px-1 pb-1 [scrollbar-width:thin]">
      {steps.map((s, i) => {
        const done = s.state === 'done'
        const here = i === current
        const late = s.state === 'late'
        const due = parseApiDate(s.due_on)
        return (
          <li key={`${s.title}-${i}`} className="flex min-w-[112px] flex-1 flex-col gap-2">
            <div className="flex items-center">
              <span
                aria-hidden
                className={cn('h-[3px] flex-1 rounded-full', i === 0 ? 'opacity-0' : done || here ? 'bg-[#1a7f4b]' : 'bg-[#ececed]')}
              />
              <span
                aria-hidden
                className={cn(
                  'flex size-7 shrink-0 items-center justify-center rounded-full border-2 text-[12px] font-semibold',
                  done && 'border-[#1a7f4b] bg-[#1a7f4b] text-white',
                  here && !late && 'border-brand bg-white text-brand ring-4 ring-[#ff782b]/15',
                  here && late && 'border-[#e5484d] bg-white text-[#e5484d] ring-4 ring-[#e5484d]/15',
                  !done && !here && 'border-[#dcdcde] bg-white text-[#a2a3a7]',
                )}
              >
                {done ? <HugeiconsIcon icon={Tick02Icon} size={14} strokeWidth={2.4} /> : i + 1}
              </span>
              <span
                aria-hidden
                className={cn('h-[3px] flex-1 rounded-full', i === steps.length - 1 ? 'opacity-0' : done ? 'bg-[#1a7f4b]' : 'bg-[#ececed]')}
              />
            </div>
            <div className="flex flex-col items-center gap-0.5 px-1 text-center">
              <span className={cn('line-clamp-2 text-[13px] leading-snug', here ? 'font-medium text-[#1b1b1b]' : done ? 'text-[#73757c]' : 'text-[#4b4b4f]')}>
                {s.title}
              </span>
              <span className={cn('text-[12px]', here && late ? 'font-medium text-[#e5484d]' : 'text-[#a2a3a7]')}>
                {here ? 'Vous êtes ici' : due === null ? '—' : DAY.format(due)}
                {here && due !== null && ` · ${DAY.format(due)}`}
              </span>
            </div>
          </li>
        )
      })}
    </ol>
  )
}

/** De qui depend la suite : le client, ou l'agence. */
function NowLine({ project, step }: { project: PortalProject; step: PortalJourneyStep | null }) {
  const pending = project.deliverables_pending
  // L'action elle-meme est dans « A faire » : ici, on dit seulement de qui
  // depend la suite, sans la repeter en orange.
  if (pending > 0 || project.status === 'attente') {
    return (
      <p className="flex items-center gap-2 rounded-[10px] bg-[#f6f7f8] px-3 py-2.5 text-[14px] text-[#4b4b4f]">
        <span aria-hidden className="size-2 shrink-0 rounded-full bg-[#ff782b]" />
        <span>
          <span className="font-medium text-[#1b1b1b]">En attente de votre retour</span>
          {pending > 0 && ` · ${pending === 1 ? 'un livrable' : `${pending} livrables`} à valider`}
          {step !== null && ` pour « ${step.title} »`}
        </span>
      </p>
    )
  }
  return (
    <p className="flex items-center gap-2 rounded-[10px] bg-[#f6f7f8] px-3 py-2.5 text-[14px] text-[#4b4b4f]">
      <span aria-hidden className="size-2 shrink-0 rounded-full bg-[#4956f4]" />
      <span>
        <span className="font-medium text-[#1b1b1b]">L’agence avance</span>
        {step !== null && ` sur « ${step.title} »`}
      </span>
    </p>
  )
}

/**
 * Le pied de la carte : la livraison, l'avancement, le site — sa
 * disponibilite et ses visiteurs quand les modules le suivent —, la derniere
 * activite.
 */
function ProjectFacts({ project, site }: { project: PortalProject; site: ProjectSite }) {
  const due = parseApiDate(project.due_on)
  const uptime = site.uptime[0]
  const down = site.uptime.some((u) => u.status === 'down')
  return (
    <div className="flex flex-wrap items-center gap-x-5 gap-y-2 border-t border-[#f3f4f4] pt-3 text-[13px] text-[#4b4b4f]">
      <span className="flex items-center gap-1.5">
        <HugeiconsIcon icon={Calendar03Icon} size={16} strokeWidth={1.6} className="text-[#8d8d8d]" />
        {due === null ? 'Livraison à définir' : `Livraison prévue le ${LONG.format(due)}`}
      </span>
      <span className="flex items-center gap-2">
        <Meter ratio={project.progress} color={project.progress >= 100 ? DONE_COLOR : PROGRESS_COLOR} className="h-1.5 w-[72px]" />
        <span className="font-medium text-[#1b1b1b] tabular-nums">{project.progress} %</span>
      </span>
      {uptime !== undefined && (
        <span className={cn('flex items-center gap-1.5', down && 'font-medium text-[#e5484d]')}>
          <span aria-hidden className={cn('size-2 rounded-full', down ? 'bg-[#e5484d]' : uptime.status === 'up' ? 'bg-[#1a7f4b]' : 'bg-[#a2a3a7]')} />
          {down ? 'Site hors ligne' : 'Site en ligne'}
          {uptime.uptime_30d !== null && !down && <span className="text-[#8d8d8d]">· {PERCENT.format(uptime.uptime_30d)} % sur 30 j</span>}
        </span>
      )}
      {site.audience !== undefined && (
        <span className="flex items-center gap-1.5">
          <HugeiconsIcon icon={ChartLineData01Icon} size={16} strokeWidth={1.6} className="text-[#8d8d8d]" />
          {NUMBER.format(site.audience.visitors)} visiteurs sur 30 j
        </span>
      )}
      <span className="flex items-center gap-1.5 text-[#8d8d8d]">
        <HugeiconsIcon icon={Clock01Icon} size={16} strokeWidth={1.6} />
        Mis à jour {since(project.last_activity_at)}
      </span>
    </div>
  )
}

/** L'historique d'un projet, dans un tiroir : lu seulement quand on l'ouvre. */
function HistorySheet({ project, open, onOpenChange }: { project: PortalProject; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { data } = useQuery({ ...portalDashboardQuery(project.id), enabled: open })
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full gap-0 sm:max-w-[420px]">
        <SheetHeader>
          <SheetTitle>Historique</SheetTitle>
          <SheetDescription>{project.name}</SheetDescription>
        </SheetHeader>
        <div className="flex-1 overflow-y-auto px-4 pb-4">
          {data === undefined ? (
            <p className="py-6 text-center text-[13px] text-[#8d8d8d]">Chargement…</p>
          ) : (
            <History items={data.activity} />
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}

/** Un projet livre ou en ligne : une ligne, il ne demande plus rien. */
export function DoneProject({ project }: { project: PortalProject }) {
  const status = PORTAL_PROJECT_STATUS[project.status]
  return (
    <Link
      to="/client/projets/$id"
      params={{ id: project.id }}
      className="group flex items-center gap-3 rounded-[12px] border border-[#e8e8e9] bg-white px-3 py-2.5 transition-colors hover:border-[#d0d1d3] hover:bg-[#fcfcfc]"
    >
      <ProjectLogo url={project.logo_url} size={24} bare className="rounded-[6px]" />
      <span className="min-w-0 flex-1 truncate text-[14px] font-medium text-[#1b1b1b]">{project.name}</span>
      <StatusPill label={status.label} color={status.color} pill={status.pill} />
      <HugeiconsIcon icon={ArrowRight01Icon} size={16} strokeWidth={1.8} className="shrink-0 text-[#a2a3a7] transition-transform group-hover:translate-x-0.5" />
    </Link>
  )
}

/** Les demandes du client, en bref, avec de quoi en ouvrir une. */
export function RequestsCard({ data }: { data: PortalDashboard }) {
  const open = data.tickets.received + data.tickets.in_progress + data.tickets.ready
  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-[16px] font-medium text-[#111]">Vos demandes</h2>
      <p className="text-[14px] text-[#4b4b4f]">
        {open === 0 ? 'Aucune demande en cours.' : `${open} demande${open > 1 ? 's' : ''} en cours.`}
        {data.tickets_resolved_this_month > 0 &&
          ` ${data.tickets_resolved_this_month} résolue${data.tickets_resolved_this_month > 1 ? 's' : ''} ce mois-ci.`}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <Link to="/client/tickets/nouvelle" className={buttonVariants({ size: 'sm' })}>
          <HugeiconsIcon icon={CustomerSupportIcon} size={14} strokeWidth={1.8} />
          Nouvelle demande
        </Link>
        <Link to="/client/tickets" className={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'border-[#d0d1d3]')}>
          Voir mes demandes
        </Link>
      </div>
    </section>
  )
}

// --- Interlocuteurs ---------------------------------------------------------

/** Une ligne de liste, et le filet qui la separe de la suivante. */
const ROW = '-mx-1.5 flex items-center gap-3 rounded-[8px] px-1.5 py-2 transition-colors hover:bg-[#fafafa]'
const ITEM = 'border-b border-[#f3f4f4] last:border-b-0'

export function Contacts({ team }: { team: PortalContact[] }) {
  return (
    <section className="flex flex-col">
      <h2 className="pb-1 text-[16px] font-medium text-[#111]">Vos interlocuteurs</h2>
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
            className={cn(buttonVariants({ variant: 'outline', size: 'icon-sm' }), 'border-[#d0d1d3]')}
          >
            <HugeiconsIcon icon={l.icon} size={16} strokeWidth={1.6} />
          </a>
        ))}
      </span>
    </li>
  )
}
