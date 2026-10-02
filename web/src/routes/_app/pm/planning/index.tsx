import {
  ArrowLeft02Icon,
  ArrowRight02Icon,
  CheckmarkSquare02Icon,
  Flag02Icon,
  Folder01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { z } from 'zod'

import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { planningQuery } from '@/features/milestones/api'
import { currentMonth, monthGrid, shiftMonth } from '@/features/milestones/calendar'
import { toDay } from '@/features/time/period'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { PlanningItem } from '@/types/api'

/**
 * Planning : les jalons, les echeances des projets et ses propres taches, sur
 * un mois.
 *
 * En lecture seule, et sans synchronisation avec un agenda exterieur : il dit
 * ce que Piilot sait, on agit depuis le projet. Le mois et la portee vivent
 * dans l'adresse — « le planning de novembre » se partage.
 */
const searchSchema = z.object({
  mois: z
    .string()
    .regex(/^\d{4}-\d{2}$/)
    .optional()
    .catch(undefined),
  portee: z.enum(['tous', 'miens']).optional().catch(undefined),
})

export const Route = createFileRoute('/_app/pm/planning/')({
  validateSearch: searchSchema,
  component: PlanningPage,
})

const MONTH = new Intl.DateTimeFormat('fr-FR', { month: 'long', year: 'numeric' })
const WEEKDAYS = ['Lun.', 'Mar.', 'Mer.', 'Jeu.', 'Ven.', 'Sam.', 'Dim.']
const LONG_DAY = new Intl.DateTimeFormat('fr-FR', { weekday: 'long', day: 'numeric', month: 'long' })

/** Nombre de lignes montrees dans une case avant « +N ». */
const PER_CELL = 3

const KINDS: Record<PlanningItem['kind'], { label: string; icon: IconSvgElement }> = {
  milestone: { label: 'Jalon', icon: Flag02Icon },
  project_due: { label: 'Échéance de projet', icon: Folder01Icon },
  task_due: { label: 'Ma tâche', icon: CheckmarkSquare02Icon },
}

function PlanningPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const { user } = Route.useRouteContext()

  const month = search.mois ?? currentMonth()
  // Par defaut, qui pilote l'agence voit tout ; l'equipe, ses projets.
  const scope = search.portee ?? (can(user, 'dashboard.read') ? 'tous' : 'miens')
  const days = monthGrid(month)
  const today = toDay(new Date())

  const { data, isError, error, isPending } = useQuery(
    planningQuery(days[0]!, days.at(-1)!, scope === 'miens' ? 'mine' : 'all'),
  )

  const byDay = new Map<string, PlanningItem[]>()
  for (const item of data?.items ?? []) {
    byDay.set(item.day, [...(byDay.get(item.day) ?? []), item])
  }

  function go(patch: { mois?: string; portee?: 'tous' | 'miens' }) {
    void navigate({ search: (prev) => ({ ...prev, ...patch }) })
  }

  return (
    <PageFrame title="Planning">
      <div className="flex min-h-full flex-col gap-3 p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon-sm"
              aria-label="Mois précédent"
              onClick={() => go({ mois: shiftMonth(month, -1) })}
            >
              <HugeiconsIcon icon={ArrowLeft02Icon} size={16} strokeWidth={1.8} />
            </Button>
            <h1 className="min-w-[160px] text-center text-[16px] font-medium text-[#1b1b1b] first-letter:uppercase">
              {MONTH.format(new Date(`${month}-15T12:00:00`))}
            </h1>
            <Button
              variant="outline"
              size="icon-sm"
              aria-label="Mois suivant"
              onClick={() => go({ mois: shiftMonth(month, 1) })}
            >
              <HugeiconsIcon icon={ArrowRight02Icon} size={16} strokeWidth={1.8} />
            </Button>
            {month !== currentMonth() && (
              <Button variant="ghost" size="sm" onClick={() => go({ mois: undefined })}>
                Ce mois-ci
              </Button>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <Legend />
            <div role="radiogroup" aria-label="Portée" className="flex rounded-[10px] bg-[#f3f4f4] p-0.5">
              {(['miens', 'tous'] as const).map((value) => (
                <button
                  key={value}
                  type="button"
                  role="radio"
                  aria-checked={scope === value}
                  onClick={() => go({ portee: value })}
                  className={cn(
                    'cursor-pointer rounded-[8px] px-3 py-1 text-[13px] transition-colors',
                    scope === value
                      ? 'bg-white text-[#1b1b1b] shadow-[0_1px_2px_rgb(16_24_40/0.08)]'
                      : 'text-[#73757c] hover:text-[#1b1b1b]',
                  )}
                >
                  {value === 'miens' ? 'Mes projets' : 'Toute l’agence'}
                </button>
              ))}
            </div>
          </div>
        </div>

        {isError && (
          <p className="rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
            {error instanceof HttpError ? error.message : 'Chargement impossible'}
          </p>
        )}

        <div className="overflow-x-auto rounded-[12px] border border-[#e8e8e9] bg-white">
          <div className="grid min-w-[760px] grid-cols-7">
            {WEEKDAYS.map((label) => (
              <div
                key={label}
                className="border-b border-[#e8e8e9] bg-[#f3f4f4] px-2 py-1.5 text-[12px] text-[#73757c]"
              >
                {label}
              </div>
            ))}

            {days.map((day, index) => (
              <DayCell
                key={day}
                day={day}
                items={byDay.get(day) ?? []}
                outside={!day.startsWith(month)}
                today={day === today}
                loading={isPending}
                lastColumn={index % 7 === 6}
              />
            ))}
          </div>
        </div>
      </div>
    </PageFrame>
  )
}

function Legend() {
  return (
    <ul className="hidden items-center gap-3 text-[12px] text-[#73757c] md:flex">
      {Object.entries(KINDS).map(([kind, { label, icon }]) => (
        <li key={kind} className="flex items-center gap-1">
          <HugeiconsIcon icon={icon} size={13} strokeWidth={1.8} />
          {label}
        </li>
      ))}
    </ul>
  )
}

function DayCell({
  day,
  items,
  outside,
  today,
  loading,
  lastColumn,
}: {
  day: string
  items: PlanningItem[]
  outside: boolean
  today: boolean
  loading: boolean
  lastColumn: boolean
}) {
  const shown = items.slice(0, PER_CELL)
  const hidden = items.length - shown.length
  const date = new Date(`${day}T12:00:00`)

  return (
    <div
      className={cn(
        'flex min-h-[112px] flex-col gap-1 border-b border-[#f0f0f1] p-1.5',
        !lastColumn && 'border-r',
        outside && 'bg-[#fcfcfc]',
      )}
    >
      <span
        className={cn(
          'flex size-6 items-center justify-center self-end rounded-full text-[12px] tabular-nums',
          today ? 'bg-brand font-medium text-white' : outside ? 'text-[#c4c4c4]' : 'text-[#4b4b4f]',
        )}
      >
        {date.getDate()}
      </span>

      {loading && <span className="h-5 animate-pulse rounded-[6px] bg-[#f3f4f4]" />}

      {shown.map((item) => (
        <Entry key={`${item.kind}:${item.id}`} item={item} day={day} />
      ))}

      {hidden > 0 && (
        <Popover>
          <PopoverTrigger className="cursor-pointer self-start rounded-[6px] px-1.5 text-[12px] text-[#73757c] hover:bg-[#f3f4f4]">
            +{hidden} de plus
          </PopoverTrigger>
          <PopoverContent align="start" className="flex w-[280px] flex-col gap-1 p-2">
            <p className="px-1 pb-1 text-[12px] font-medium text-[#73757c] first-letter:uppercase">
              {LONG_DAY.format(date)}
            </p>
            {items.map((item) => (
              <Entry key={`${item.kind}:${item.id}`} item={item} day={day} />
            ))}
          </PopoverContent>
        </Popover>
      )}
    </div>
  )
}

/** Une ligne du calendrier, qui mene la ou l'on agit. */
function Entry({ item, day }: { item: PlanningItem; day: string }) {
  const late = !item.done && day < toDay(new Date())
  const { icon, label } = KINDS[item.kind]

  const tone = item.done
    ? 'bg-[#f3f4f4] text-[#8d8d8d] line-through decoration-[#c4c4c4]'
    : late
      ? 'bg-[#ffe8ec] text-[#a30f2c]'
      : item.kind === 'milestone'
        ? 'bg-[#eef0fe] text-[#3b45c9]'
        : item.kind === 'project_due'
          ? 'bg-[#1b1b1b] text-white'
          : 'bg-[#fff1d4] text-[#9a6a00]'

  const title =
    item.kind === 'milestone' && item.deliverables_total > 0
      ? `${label} · ${item.project.name} · ${item.deliverables_validated}/${item.deliverables_total} livrables validés`
      : `${label} · ${item.project.name}`

  const className = cn(
    'flex min-w-0 items-center gap-1 rounded-[6px] px-1.5 py-0.5 text-[12px] leading-[1.4] transition-opacity hover:opacity-80',
    tone,
  )
  const body = (
    <>
      <HugeiconsIcon icon={icon} size={12} strokeWidth={1.8} className="shrink-0" />
      <span className="truncate">{item.title}</span>
    </>
  )

  if (item.kind === 'task_due') {
    return (
      <Link
        to="/pm/projets/$id/taches"
        params={{ id: item.project.id }}
        search={{ tache: item.id }}
        title={title}
        className={className}
      >
        {body}
      </Link>
    )
  }

  return (
    <Link
      to={item.kind === 'milestone' ? '/pm/projets/$id/jalons' : '/pm/projets/$id'}
      params={{ id: item.project.id }}
      title={title}
      className={className}
    >
      {body}
    </Link>
  )
}
