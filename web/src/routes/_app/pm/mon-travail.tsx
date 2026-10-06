import {
  Alert02Icon,
  ArrowRight01Icon,
  CheckmarkSquare02Icon,
  Clock01Icon,
  DeliveryBox01Icon,
  Folder01Icon,
  Ticket02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { DashboardCard } from '@/components/dashboard-card'
import { StatCard } from '@/components/stat-card'
import { PageFrame } from '@/components/layout/page-frame'
import { myWorkQuery } from '@/features/my-work/api'
import {
  DONE_COLOR,
  parseApiDate,
  PROGRESS_COLOR,
  PROJECT_STATUS,
  TASK_STATUS,
} from '@/features/projects/format'
import { Meter, PriorityTag, StatusPill } from '@/features/projects/ui'
import { formatDuration } from '@/features/time/format'
import { TICKET_PRIORITY, TICKET_STATUS } from '@/features/tickets/format'
import { HttpError } from '@/lib/api'
import { can, sessionQuery } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { MyWork, WorkDeliverable, WorkProject, WorkTask, WorkTicket } from '@/types/api'

/**
 * « Mon travail » : l'accueil de l'equipe.
 *
 * Ce qui attend la personne connectee, tous projets confondus : ses taches
 * (les retards d'abord), les tickets qu'on lui a confies, les livrables a
 * deposer ou a reprendre, et les projets ou elle intervient. Un seul appel,
 * cinq blocs bornes ; chaque bloc mene a l'ecran complet de son objet.
 *
 * La question a laquelle la page repond est « par quoi je commence ? » — pas
 * « ou en est l'agence ? », qui est celle du tableau de bord.
 */
export const Route = createFileRoute('/_app/pm/mon-travail')({
  loader: ({ context }) => context.queryClient.ensureQueryData(myWorkQuery),
  component: MyWorkPage,
})

const DAY = new Intl.DateTimeFormat('fr-FR', { weekday: 'long', day: 'numeric', month: 'long' })
const SHORT = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })

/** Echeance en mots : « hier », « dans 3 jours », « 12 oct. ». */
function dueLabel(iso: string | null, today: string): string {
  const due = parseApiDate(iso)
  const now = parseApiDate(today)
  if (due === null || now === null) return 'Sans échéance'

  const days = Math.round((due.getTime() - now.getTime()) / 86_400_000)
  if (days === 0) return 'Aujourd’hui'
  if (days === 1) return 'Demain'
  if (days === -1) return 'Hier'
  if (days < 0) return `En retard de ${-days} j`
  if (days < 7) return `Dans ${days} jours`

  return SHORT.format(due)
}

function MyWorkPage() {
  const { data: work, isError, error } = useQuery(myWorkQuery)
  const { data: session } = useQuery(sessionQuery)

  if (isError) {
    return (
      <PageFrame title="Mon travail">
        <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
          {error instanceof HttpError ? error.message : 'Chargement impossible'}
        </p>
      </PageFrame>
    )
  }

  if (work === undefined) {
    return (
      <PageFrame title="Mon travail">
        <div className="m-4 h-[420px] animate-pulse rounded-[12px] bg-[#fafafa]" />
      </PageFrame>
    )
  }

  const today = parseApiDate(work.today) ?? new Date()

  return (
    <PageFrame title="Mon travail">
      <div className="flex min-h-full flex-col gap-4 p-4">
        <header className="flex flex-col gap-0.5">
          <h1 className="font-heading text-[22px] font-medium text-[#1b1b1b]">
            Bonjour {session?.firstname}
          </h1>
          <p className="text-[14px] text-[#73757c] first-letter:uppercase">{DAY.format(today)}</p>
        </header>

        <Summary work={work} canLogTime={can(session, 'time.write')} />

        <div className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
          <TasksCard work={work} />

          <div className="flex flex-col gap-4">
            {can(session, 'tickets.read') && <TicketsCard work={work} />}
            {can(session, 'deliverables.read') && <DeliverablesCard work={work} />}
          </div>
        </div>

        <ProjectsCard projects={work.projects} today={work.today} />
      </div>
    </PageFrame>
  )
}

/**
 * Bandeau de chiffres : ce qui presse, d'un coup d'oeil. Chaque chiffre est un
 * lien vers le bloc ou l'ecran qui le detaille.
 */
function Summary({ work, canLogTime }: { work: MyWork; canLogTime: boolean }) {
  return (
    <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
      <Figure
        label="En retard"
        value={String(work.tasks_overdue)}
        hint={work.tasks_overdue === 0 ? 'Rien en retard' : 'Tâche' + (work.tasks_overdue > 1 ? 's' : '') + ' à rattraper'}
        tone={work.tasks_overdue > 0 ? 'alert' : undefined}
        href="#taches"
      />
      <Figure
        label="Tâches ouvertes"
        value={String(work.tasks_total)}
        hint="Assignées à vous"
        href="#taches"
      />
      <Figure
        label="Tickets"
        value={String(work.tickets_total)}
        hint={work.tickets_total === 0 ? 'Aucun ticket ouvert' : 'À traiter'}
        href="#tickets"
      />
      <Figure
        label="Cette semaine"
        value={formatDuration(work.week_minutes)}
        hint={canLogTime ? 'Voir ma semaine' : 'Temps saisi depuis lundi'}
        to={canLogTime ? '/pm/temps/saisie' : undefined}
      />
    </div>
  )
}

/**
 * Chiffre du bandeau : la carte du tableau de bord, rendue cliquable. Le lien
 * enveloppe la carte sans rien lui ajouter, sinon un leger relief au survol.
 */
function Figure({
  label,
  value,
  hint,
  tone,
  href,
  to,
}: {
  label: string
  value: string
  hint: string
  tone?: 'alert'
  href?: string
  to?: '/pm/temps/saisie'
}) {
  const body = (
    <StatCard
      label={label}
      value={value}
      tone={tone}
      footer={<p className="min-w-px flex-1 text-xs leading-[1.5] text-[#111]">{hint}</p>}
      className="h-full transition-shadow group-hover:shadow-[0_6px_18px_-10px_rgb(16_24_40/0.25)]"
    />
  )

  const className = 'group flex rounded-[12px] outline-none focus-visible:ring-3 focus-visible:ring-ring/50'

  if (to !== undefined) {
    return (
      <Link to={to} search={{ vue: 'semaine' }} className={className}>
        {body}
      </Link>
    )
  }

  return (
    <a href={href} className={className}>
      {body}
    </a>
  )
}

/** Lien « tout voir » du coin d'une carte. */
function SeeAll({ children, ...link }: { children: ReactNode; to: '/pm/taches' | '/pm/tickets' | '/pm/livrables' | '/pm/projets' }) {
  return (
    <Link
      {...link}
      className="ml-auto flex items-center gap-0.5 text-[12px] text-[#73757c] transition-colors hover:text-[#1b1b1b]"
    >
      {children}
      <HugeiconsIcon icon={ArrowRight01Icon} size={14} strokeWidth={1.8} />
    </Link>
  )
}

function Empty({ children }: { children: ReactNode }) {
  return <p className="py-6 text-center text-[13px] text-[#73757c]">{children}</p>
}

/** Plus de lignes que la carte n'en montre : on le dit, sans les montrer. */
function More({ shown, total }: { shown: number; total: number }) {
  if (total <= shown) return null

  return (
    <p className="pt-2 text-[12px] text-[#8d8d8d]">
      Et {total - shown} de plus — la liste complète est dans l’écran dédié.
    </p>
  )
}

function TasksCard({ work }: { work: MyWork }) {
  const overdue = work.tasks.filter((task) => task.overdue)
  const upcoming = work.tasks.filter((task) => !task.overdue)

  return (
    <div id="taches" className="scroll-mt-4">
      <DashboardCard
        icon={CheckmarkSquare02Icon}
        title="Mes tâches"
        action={<SeeAll to="/pm/taches">Toutes les tâches</SeeAll>}
        className="h-full"
      >
        {work.tasks.length === 0 ? (
          <Empty>Aucune tâche ouverte ne vous est assignée.</Empty>
        ) : (
          <div className="flex flex-col gap-3">
            {overdue.length > 0 && (
              <section className="flex flex-col">
                <h3 className="flex items-center gap-1.5 pb-1 text-[12px] font-medium tracking-wide text-[#e5484d] uppercase">
                  <HugeiconsIcon icon={Alert02Icon} size={14} strokeWidth={1.8} />
                  En retard
                </h3>
                {overdue.map((task) => (
                  <TaskRow key={task.id} task={task} today={work.today} />
                ))}
              </section>
            )}

            {upcoming.length > 0 && (
              <section className="flex flex-col">
                <h3 className="pb-1 text-[12px] font-medium tracking-wide text-[#73757c] uppercase">
                  À venir
                </h3>
                {upcoming.map((task) => (
                  <TaskRow key={task.id} task={task} today={work.today} />
                ))}
              </section>
            )}

            <More shown={work.tasks.length} total={work.tasks_total} />
          </div>
        )}
      </DashboardCard>
    </div>
  )
}

function TaskRow({ task, today }: { task: WorkTask; today: string }) {
  const status = TASK_STATUS[task.status]

  return (
    <Link
      to="/pm/projets/$id/taches"
      params={{ id: task.project.id }}
      search={{ tache: task.id }}
      className="-mx-1.5 flex items-center gap-3 rounded-[8px] border-b border-[#f3f4f4] px-1.5 py-2 transition-colors last:border-b-0 hover:bg-[#fafafa]"
    >
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-[14px] text-[#1b1b1b]">{task.title}</span>
        <span className="truncate text-[12px] text-[#8d8d8d]">{task.project.name}</span>
      </span>

      <PriorityTag priority={task.priority} className="hidden sm:inline-flex" />
      <StatusPill label={status.label} color={status.color} pill={status.pill} className="hidden md:inline-flex" />

      <span
        className={cn(
          'w-[118px] shrink-0 text-right text-[12px] tabular-nums',
          task.overdue ? 'font-medium text-[#e5484d]' : 'text-[#73757c]',
        )}
      >
        {dueLabel(task.due_on, today)}
      </span>
    </Link>
  )
}

function TicketsCard({ work }: { work: MyWork }) {
  return (
    <div id="tickets" className="scroll-mt-4">
      <DashboardCard
        icon={Ticket02Icon}
        title="Tickets qui m’attendent"
        action={<SeeAll to="/pm/tickets">Mes tickets</SeeAll>}
      >
        {work.tickets.length === 0 ? (
          <Empty>Aucun ticket ouvert ne vous est confié.</Empty>
        ) : (
          <div className="flex flex-col">
            {work.tickets.map((ticket) => (
              <TicketRow key={ticket.id} ticket={ticket} />
            ))}
            <More shown={work.tickets.length} total={work.tickets_total} />
          </div>
        )}
      </DashboardCard>
    </div>
  )
}

function TicketRow({ ticket }: { ticket: WorkTicket }) {
  const priority = TICKET_PRIORITY[ticket.priority]
  const status = TICKET_STATUS[ticket.status]

  return (
    <Link
      to="/pm/tickets/$id"
      params={{ id: ticket.id }}
      className="-mx-1.5 flex items-center gap-3 rounded-[8px] border-b border-[#f3f4f4] px-1.5 py-2 transition-colors last:border-b-0 hover:bg-[#fafafa]"
    >
      <span className="w-[44px] shrink-0 text-[12px] text-[#a2a3a7] tabular-nums">#{ticket.numero}</span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-[14px] text-[#1b1b1b]">{ticket.subject}</span>
        <span className="truncate text-[12px] text-[#8d8d8d]">
          {ticket.project.name} · {status.label}
        </span>
      </span>
      <StatusPill label={priority.label} color={priority.pill.text} pill={priority.pill} />
    </Link>
  )
}

function DeliverablesCard({ work }: { work: MyWork }) {
  return (
    <DashboardCard
      icon={DeliveryBox01Icon}
      title="Livrables à déposer"
      action={<SeeAll to="/pm/livrables">Tous les livrables</SeeAll>}
    >
      {work.deliverables.length === 0 ? (
        <Empty>Rien à déposer : tout est chez le client ou validé.</Empty>
      ) : (
        <div className="flex flex-col">
          {work.deliverables.map((deliverable) => (
            <DeliverableRow key={deliverable.id} deliverable={deliverable} />
          ))}
          <More shown={work.deliverables.length} total={work.deliverables_total} />
        </div>
      )}
    </DashboardCard>
  )
}

function DeliverableRow({ deliverable }: { deliverable: WorkDeliverable }) {
  const feedback = deliverable.state === 'feedback'

  return (
    <Link
      to="/pm/livrables"
      search={{ page: 1, projet: deliverable.project.id }}
      className="-mx-1.5 flex flex-col gap-0.5 rounded-[8px] border-b border-[#f3f4f4] px-1.5 py-2 transition-colors last:border-b-0 hover:bg-[#fafafa]"
    >
      <span className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-[14px] text-[#1b1b1b]">{deliverable.title}</span>
        <span
          className={cn(
            'shrink-0 rounded-full border px-2 py-0.5 text-[12px]',
            feedback
              ? 'border-[#ffe8b7] bg-[#fff1d4] text-[#9a6a00]'
              : 'border-[#e8e8e9] bg-[#f3f4f4] text-[#1b1b1b]',
          )}
        >
          {feedback ? `Retours sur la V${deliverable.version}` : 'À déposer'}
        </span>
      </span>
      <span className="truncate text-[12px] text-[#8d8d8d]">{deliverable.project.name}</span>
      {feedback && deliverable.feedback !== '' && (
        <span className="line-clamp-2 text-[12px] text-[#4b4b4f] italic">« {deliverable.feedback} »</span>
      )}
    </Link>
  )
}

function ProjectsCard({ projects, today }: { projects: WorkProject[]; today: string }) {
  return (
    <DashboardCard
      icon={Folder01Icon}
      title="Mes projets"
      action={<SeeAll to="/pm/projets">Tous les projets</SeeAll>}
    >
      {projects.length === 0 ? (
        <Empty>Vous ne faites partie de l’équipe d’aucun projet en cours.</Empty>
      ) : (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {projects.map((project) => (
            <ProjectTile key={project.id} project={project} today={today} />
          ))}
        </div>
      )}
    </DashboardCard>
  )
}

function ProjectTile({ project, today }: { project: WorkProject; today: string }) {
  const status = PROJECT_STATUS[project.status]
  const ratio = project.tasks_total === 0 ? 0 : (project.tasks_done / project.tasks_total) * 100

  return (
    <Link
      to="/pm/projets/$id"
      params={{ id: project.id }}
      className="flex flex-col gap-2 rounded-[10px] border border-[#ebebeb] p-3 transition-colors hover:border-[#d8d8d8] hover:bg-[#fcfcfc]"
    >
      <span className="flex items-start gap-2">
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate text-[14px] font-medium text-[#1b1b1b]">{project.name}</span>
          <span className="truncate text-[12px] text-[#8d8d8d]">{project.client_name}</span>
        </span>
        <StatusPill label={status.label} color={status.color} pill={status.pill} />
      </span>

      <Meter ratio={ratio} color={ratio >= 100 ? DONE_COLOR : PROGRESS_COLOR} />

      <span className="flex items-center justify-between text-[12px] text-[#73757c] tabular-nums">
        <span>
          {project.tasks_done}/{project.tasks_total} tâches
        </span>
        <span className="flex items-center gap-1">
          <HugeiconsIcon icon={Clock01Icon} size={12} strokeWidth={1.8} />
          {dueLabel(project.due_on, today)}
        </span>
      </span>
    </Link>
  )
}
