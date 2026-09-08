import {
  Calendar03Icon,
  CheckmarkCircle02Icon,
  Clock01Icon,
  File01Icon,
  Message01Icon,
  UserGroupIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { createFileRoute, getRouteApi } from '@tanstack/react-router'

import { DashboardCard } from '@/components/dashboard-card'
import {
  ACTIVITY,
  CONTACT,
  DELIVERABLE_STATE,
  DELIVERABLES,
  MILESTONES,
  STATUS,
  TICKETS,
  type Project,
} from '@/features/projects/fixtures'
import { BILLABLE_COLOR, Meter } from '@/features/projects/ui'
import { cn } from '@/lib/utils'

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })
const LONG_DATE = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})
const RELATIVE = new Intl.RelativeTimeFormat('fr-FR', { numeric: 'auto' })

/** Couleurs des jalons — celles des statuts, pour un vocabulaire unique. */
const MILESTONE_COLOR = {
  done: '#0db471',
  current: '#4956f4',
  todo: '#c4c4c4',
} as const

/** Le projet est charge par la route parente : les onglets le partagent. */
const parent = getRouteApi('/_app/pm/projets/$id')

export const Route = createFileRoute('/_app/pm/projets/$id/')({
  component: ProjectOverviewPage,
})

function inDays(offset: number) {
  const date = new Date()
  date.setHours(0, 0, 0, 0)
  date.setDate(date.getDate() + offset)

  return date
}

function ProjectOverviewPage() {
  const project = parent.useLoaderData()

  return (
    <div className="flex min-h-0 flex-1 flex-col xl:flex-row">
      <div className="flex min-w-0 flex-1 flex-col gap-4 p-4">
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Milestones />
          <Deliverables />
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Team project={project} />
          <Tickets />
        </div>

        <Activity />
      </div>

      {/* Meme place et meme largeur que l'agenda du tableau de bord : d'un
          ecran a l'autre du module, la colonne de droite reste la colonne de
          droite. Elle vit dans cet onglet et non dans le chassis — le tableau
          des taches a besoin de toute la largeur pour ses quatre colonnes. */}
      <aside className="flex w-full shrink-0 flex-col gap-4 border-t border-[#efefef] bg-white p-4 xl:w-[340px] xl:border-t-0 xl:border-l">
        <h2 className="text-[16px] font-medium text-[#111]">Fiche projet</h2>

        <dl className="flex flex-col gap-3 border-b border-[#ebebeb] pb-4">
          {[
            { label: 'Client', value: project.client },
            { label: 'Contact', value: `${CONTACT.name} — ${CONTACT.role}` },
            { label: 'Statut', value: STATUS[project.status].label },
            { label: 'Échéance', value: LONG_DATE.format(project.due) },
            { label: 'Budget vendu', value: `${project.hoursSold} h` },
          ].map((row) => (
            <div key={row.label} className="flex flex-col gap-0.5">
              <dt className="text-[12px] text-[#64748b]">{row.label}</dt>
              <dd className="text-[13px] font-medium text-[#0f172a]">{row.value}</dd>
            </div>
          ))}
        </dl>

        <div className="flex flex-col gap-2">
          <p className="text-[12px] text-[#64748b]">Équipe affectée</p>

          {project.team.map((member) => (
            <div key={member.initials} className="flex items-center gap-2.5">
              <span
                className="flex size-8 shrink-0 items-center justify-center rounded-[6px] text-[12px] font-semibold text-[#0f172a]"
                style={{ backgroundColor: member.tint }}
              >
                {member.initials}
              </span>
              <p className="truncate text-[13px] text-[#0f172a]">{member.name}</p>
            </div>
          ))}
        </div>
      </aside>
    </div>
  )
}

/**
 * Frise des jalons.
 *
 * Verticale et non horizontale : les libelles d'un jalon tiennent sur une
 * ligne, pas sous un point de 8px, et une frise horizontale les tronquerait
 * des la deuxieme etape.
 */
function Milestones() {
  return (
    <DashboardCard icon={Calendar03Icon} title="JALONS">
      <ol className="flex flex-1 flex-col">
        {MILESTONES.map((milestone, index) => {
          const date = inDays(milestone.offset)
          const last = index === MILESTONES.length - 1

          return (
            <li key={milestone.label} className="flex gap-3">
              {/* Le trait relie les pastilles entre elles ; le dernier jalon
                  n'a rien apres lui, donc pas de trait qui pende. */}
              <div className="flex flex-col items-center">
                <span
                  aria-hidden
                  className={cn(
                    'mt-1 size-2.5 shrink-0 rounded-full',
                    milestone.state === 'current' && 'ring-4 ring-[#4956f4]/15',
                  )}
                  style={{ backgroundColor: MILESTONE_COLOR[milestone.state] }}
                />
                {!last && <span aria-hidden className="w-px flex-1 bg-[#ebebeb]" />}
              </div>

              <div className={cn('flex min-w-0 flex-col', last ? 'pb-0' : 'pb-4')}>
                <p
                  className={cn(
                    'truncate text-[13px]',
                    milestone.state === 'todo' ? 'text-[#8d8d8d]' : 'font-semibold text-[#0f172a]',
                  )}
                >
                  {milestone.label}
                </p>
                <p className="text-[12px] text-[#64748b] tabular-nums">
                  {DATE_FORMAT.format(date)} · {RELATIVE.format(milestone.offset, 'day')}
                </p>
              </div>
            </li>
          )
        })}
      </ol>
    </DashboardCard>
  )
}

/**
 * Livrables et leur validation.
 *
 * L'etat prime sur la date : ce qu'on vient chercher ici, c'est ce qui attend
 * le client et ce qu'il a renvoye, pas un calendrier.
 */
function Deliverables() {
  return (
    <DashboardCard icon={File01Icon} title="LIVRABLES">
      <ul className="flex flex-1 flex-col gap-1">
        {DELIVERABLES.map((item) => {
          const state = DELIVERABLE_STATE[item.state]

          return (
            <li
              key={item.name}
              className="flex items-center justify-between gap-2 rounded-[8px] px-2 py-2 transition-colors hover:bg-[#f8f8f8]"
            >
              <div className="flex min-w-0 flex-1 items-center gap-2.5">
                <span
                  aria-hidden
                  className="size-1.5 shrink-0 rounded-full"
                  style={{ backgroundColor: state.color }}
                />
                <p className="truncate text-[13px] text-[#0f172a]">{item.name}</p>
              </div>

              <div className="flex shrink-0 items-center gap-3">
                <span
                  className="text-[12px] font-medium whitespace-nowrap"
                  style={{ color: state.color }}
                >
                  {state.label}
                </span>
                <span className="w-14 text-right text-[12px] text-[#8d8d8d] tabular-nums">
                  {DATE_FORMAT.format(inDays(item.offset))}
                </span>
              </div>
            </li>
          )
        })}
      </ul>
    </DashboardCard>
  )
}

/**
 * Repartition du temps saisi dans l'equipe.
 *
 * Les heures sont reparties depuis le total du projet, decroissantes : la
 * somme des lignes tombe donc juste avec le chiffre de l'en-tete, ce qu'un
 * tirage independant ne garantirait pas.
 */
function Team({ project }: { project: Project }) {
  const weights = project.team.map((_, index) => project.team.length - index)
  const total = weights.reduce((sum, weight) => sum + weight, 0)

  return (
    <DashboardCard icon={UserGroupIcon} title="TEMPS PAR PERSONNE">
      <ul className="flex flex-1 flex-col gap-1">
        {project.team.map((member, index) => {
          const hours = Math.round((project.hoursSpent * (weights[index] ?? 0)) / total)
          const share = (hours / project.hoursSpent) * 100

          return (
            <li
              key={member.initials}
              className="flex items-center justify-between gap-2 rounded-[8px] px-2 py-2 transition-colors hover:bg-[#f8f8f8]"
            >
              <div className="flex min-w-0 flex-1 items-center gap-2.5">
                <span
                  className="flex size-8 shrink-0 items-center justify-center rounded-[6px] text-[12px] font-semibold text-[#0f172a]"
                  style={{ backgroundColor: member.tint }}
                >
                  {member.initials}
                </span>
                <p className="truncate text-[13px] text-[#0f172a]">{member.name}</p>
              </div>

              <div className="flex shrink-0 items-center gap-2">
                <Meter ratio={share} color={BILLABLE_COLOR} className="w-16" />
                <span className="w-12 text-right text-[12px] font-medium text-[#0f172a] tabular-nums">
                  {hours} h
                </span>
              </div>
            </li>
          )
        })}
      </ul>
    </DashboardCard>
  )
}

/** Tickets du projet, par etat. */
function Tickets() {
  const rows = [
    { label: 'Ouverts', value: TICKETS.open, color: '#e5484d' },
    { label: 'En attente client', value: TICKETS.waiting, color: '#eab308' },
    { label: 'Résolus', value: TICKETS.closed, color: '#0db471' },
  ]

  const total = rows.reduce((sum, row) => sum + row.value, 0)

  return (
    <DashboardCard icon={Message01Icon} title="TICKETS">
      <div className="flex flex-1 flex-col justify-between gap-3">
        <div className="flex flex-wrap items-end gap-x-3 gap-y-1">
          <p className="text-[32px] leading-[1.3] font-semibold text-[#1f1f1f] tabular-nums">
            {TICKETS.open}
          </p>
          <p className="pb-1 text-[12px] leading-[1.5] font-medium text-[#8d8d8d]">
            ouverts sur {total} depuis le début
          </p>
        </div>

        {/* Une barre d'un seul tenant plutot que trois chiffres cote a cote :
            la part de resolu se lit sans faire la division. */}
        <div className="flex h-8 overflow-hidden rounded-[4px]">
          {rows.map((row) => (
            <div
              key={row.label}
              style={{
                width: `${(row.value / total) * 100}%`,
                backgroundImage: `repeating-linear-gradient(90deg, ${row.color} 0 2px, transparent 2px 5px)`,
              }}
            />
          ))}
        </div>

        <div className="flex flex-wrap items-center gap-x-[11px] gap-y-2">
          {rows.map((row) => (
            <div key={row.label} className="flex items-center gap-2">
              <div
                aria-hidden
                className="size-2 shrink-0 rounded-[2px]"
                style={{ backgroundColor: row.color }}
              />
              <p className="text-[12px] leading-[1.5] font-medium whitespace-nowrap text-[#030512]">
                {row.label}
                <span className="pl-1 text-[#8d8d8d] tabular-nums">{row.value}</span>
              </p>
            </div>
          ))}
        </div>
      </div>
    </DashboardCard>
  )
}

/** Fil d'activite du projet. */
function Activity() {
  return (
    <DashboardCard icon={Clock01Icon} title="ACTIVITÉ RÉCENTE">
      <ul className="flex flex-1 flex-col gap-1">
        {ACTIVITY.map((entry) => (
          <li
            key={`${entry.who}-${entry.target}`}
            className="flex flex-wrap items-center gap-x-1.5 gap-y-0.5 rounded-[8px] px-2 py-2 transition-colors hover:bg-[#f8f8f8]"
          >
            <HugeiconsIcon
              icon={CheckmarkCircle02Icon}
              size={14}
              strokeWidth={1.6}
              className="shrink-0 text-[#c4c4c4]"
            />
            <span className="text-[13px] font-semibold text-[#0f172a]">{entry.who}</span>
            <span className="text-[13px] text-[#64748b]">{entry.what}</span>
            <span className="text-[13px] font-medium text-[#0f172a]">{entry.target}</span>
            <span className="ml-auto shrink-0 text-[12px] whitespace-nowrap text-[#8d8d8d]">
              {RELATIVE.format(entry.offset, 'day')}
            </span>
          </li>
        ))}
      </ul>
    </DashboardCard>
  )
}
