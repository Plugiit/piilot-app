import { ArrowLeft02Icon, ArrowRight02Icon, Delete02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { toast } from 'sonner'
import { z } from 'zod'

import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { useDeleteTimeEntry, timeSheetQuery } from '@/features/time/api'
import { TimeEntryForm } from '@/features/time/entry-form'
import { formatDuration } from '@/features/time/format'
import { mondayOf } from '@/features/time/week'
import { WeekSheet } from '@/features/time/week-sheet'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { TimeEntry } from '@/types/api'

/**
 * Saisie du temps.
 *
 * Deux vues. La journee, pour qui pointe le soir ce qu'il vient de faire : un
 * formulaire et ses lignes. La semaine, pour qui pointe le vendredi : une
 * grille projet par jour, ou l'on remplit les cases d'un Tab a l'autre.
 *
 * Le jour et la vue vivent dans l'adresse : « ma semaine du 28 » se partage,
 * et le bouton Retour y ramene. En semaine, le jour designe la semaine qui le
 * contient.
 */
const searchSchema = z.object({
  jour: z.string().optional(),
  vue: z.enum(['jour', 'semaine']).optional().catch(undefined),
})

/** Aujourd'hui au format de l'API, dans le fuseau du navigateur. */
function today(): string {
  const now = new Date()
  const mois = String(now.getMonth() + 1).padStart(2, '0')
  const jour = String(now.getDate()).padStart(2, '0')

  return `${now.getFullYear()}-${mois}-${jour}`
}

/** Le jour voisin, en arriere ou en avant. */
function shiftDay(day: string, delta: number): string {
  const date = new Date(`${day}T12:00:00`)
  date.setDate(date.getDate() + delta)

  const mois = String(date.getMonth() + 1).padStart(2, '0')
  const jour = String(date.getDate()).padStart(2, '0')

  return `${date.getFullYear()}-${mois}-${jour}`
}

const SHORT_DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long' })

const LONG_DAY = new Intl.DateTimeFormat('fr-FR', {
  weekday: 'long',
  day: 'numeric',
  month: 'long',
})

export const Route = createFileRoute('/_app/pm/temps/saisie/')({
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'time.write')) throw redirect({ to: '/pm/mon-travail', replace: true })
  },
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ jour: search.jour ?? today(), semaine: search.vue === 'semaine' }),
  loader: ({ context, deps }) => {
    const from = deps.semaine ? mondayOf(deps.jour) : deps.jour
    const to = deps.semaine ? shiftDay(from, 6) : deps.jour

    return context.queryClient.query({ ...timeSheetQuery(from, to), staleTime: 'static' })
  },
  component: SaisiePage,
})

function SaisiePage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const jour = search.jour ?? today()
  const semaine = search.vue === 'semaine'
  const monday = mondayOf(jour)

  function goTo(next: string, vue: 'jour' | 'semaine' = semaine ? 'semaine' : 'jour') {
    void navigate({
      search: {
        jour: next === today() ? undefined : next,
        vue: vue === 'semaine' ? 'semaine' : undefined,
      },
    })
  }

  const step = semaine ? 7 : 1
  const atToday = semaine ? monday === mondayOf(today()) : jour === today()

  return (
    <PageFrame title="Saisie du temps">
      <div className="flex min-h-full flex-col gap-4 p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <ViewSwitch value={semaine ? 'semaine' : 'jour'} onChange={(vue) => goTo(jour, vue)} />

            <Button
              variant="outline"
              size="icon-sm"
              aria-label={semaine ? 'Semaine précédente' : 'Jour précédent'}
              onClick={() => goTo(shiftDay(jour, -step))}
            >
              <HugeiconsIcon icon={ArrowLeft02Icon} size={16} strokeWidth={1.8} />
            </Button>

            <p className="min-w-[190px] text-center text-[15px] font-medium text-[#1b1b1b] first-letter:uppercase">
              {semaine
                ? `Semaine du ${SHORT_DAY.format(new Date(`${monday}T12:00:00`))}`
                : LONG_DAY.format(new Date(`${jour}T12:00:00`))}
            </p>

            <Button
              variant="outline"
              size="icon-sm"
              aria-label={semaine ? 'Semaine suivante' : 'Jour suivant'}
              onClick={() => goTo(shiftDay(jour, step))}
            >
              <HugeiconsIcon icon={ArrowRight02Icon} size={16} strokeWidth={1.8} />
            </Button>

            {!atToday && (
              <Button variant="ghost" size="sm" onClick={() => goTo(today())}>
                {semaine ? 'Cette semaine' : 'Aujourd’hui'}
              </Button>
            )}
          </div>

          {!semaine && <DayTotal jour={jour} />}
        </div>

        {semaine ? (
          <WeekSheet key={monday} monday={monday} onOpenDay={(day) => goTo(day, 'jour')} />
        ) : (
          <DayView jour={jour} />
        )}
      </div>
    </PageFrame>
  )
}

/** Bascule entre la journee et la semaine. */
function ViewSwitch({
  value,
  onChange,
}: {
  value: 'jour' | 'semaine'
  onChange: (value: 'jour' | 'semaine') => void
}) {
  return (
    <div role="radiogroup" aria-label="Vue" className="flex rounded-[10px] bg-[#f3f4f4] p-0.5">
      {(['jour', 'semaine'] as const).map((vue) => (
        <button
          key={vue}
          type="button"
          role="radio"
          aria-checked={value === vue}
          onClick={() => onChange(vue)}
          className={cn(
            'cursor-pointer rounded-[8px] px-3 py-1 text-[13px] transition-colors',
            value === vue ? 'bg-white text-[#1b1b1b] shadow-[0_1px_2px_rgb(16_24_40/0.08)]' : 'text-[#73757c] hover:text-[#1b1b1b]',
          )}
        >
          {vue === 'jour' ? 'Jour' : 'Semaine'}
        </button>
      ))}
    </div>
  )
}

/** Le total du jour, en gros : c'est la seule chose qu'on verifie avant de fermer l'ecran. */
function DayTotal({ jour }: { jour: string }) {
  const { data } = useQuery(timeSheetQuery(jour, jour))

  return (
    <p className="text-[20px] font-medium text-[#1b1b1b] tabular-nums">
      {formatDuration(data?.total_minutes ?? 0)}
    </p>
  )
}

function DayView({ jour }: { jour: string }) {
  const { data, isError, error } = useQuery(timeSheetQuery(jour, jour))
  const items = data?.items ?? []

  return (
    <>
      <TimeEntryForm day={jour} />

      {isError ? (
        <p className="rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
          {error instanceof HttpError ? error.message : 'Chargement impossible'}
        </p>
      ) : items.length === 0 ? (
        <p className="rounded-[12px] border border-dashed border-[#e8e8e9] p-8 text-center text-[14px] text-[#73757c]">
          Rien de pointé ce jour-là.
        </p>
      ) : (
        <div className="flex flex-col gap-2">
          {items.map((entry) => (
            <EntryRow key={entry.id} entry={entry} />
          ))}
        </div>
      )}
    </>
  )
}

/** Une ligne pointee. */
function EntryRow({ entry }: { entry: TimeEntry }) {
  const remove = useDeleteTimeEntry(entry.id)

  return (
    <div className="group flex items-center gap-3 rounded-[10px] border border-[#e8e8e9] bg-white px-3 py-2.5">
      <span className="w-[70px] shrink-0 text-[15px] font-medium text-[#1b1b1b] tabular-nums">
        {formatDuration(entry.minutes)}
      </span>

      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5">
          <span className="truncate text-[14px] text-[#1b1b1b]">{entry.project.name}</span>

          {entry.task !== null && (
            <>
              <span aria-hidden className="size-1 shrink-0 rounded-full bg-[#d0d1d3]" />
              <span className="truncate text-[13px] text-[#73757c]">{entry.task.name}</span>
            </>
          )}

          {entry.service !== null && (
            <span className="flex shrink-0 items-center gap-1.5">
              <span
                aria-hidden
                className="size-2 rounded-full"
                style={{ backgroundColor: entry.service.color }}
              />
              <span className="text-[12px] text-[#73757c]">{entry.service.name}</span>
            </span>
          )}
        </span>

        {entry.note !== '' && (
          <span className="truncate text-[12px] text-[#a2a3a7]">{entry.note}</span>
        )}
      </div>

      {/* Ne se montre qu'au survol : une rangee de corbeilles sur un ecran
          qu'on remplit vite invite surtout a cliquer par erreur. */}
      <button
        type="button"
        aria-label={`Supprimer la saisie de ${formatDuration(entry.minutes)}`}
        disabled={remove.isPending}
        onClick={() =>
          remove.mutate(undefined, {
            onError: (error) =>
              toast.error(error instanceof HttpError ? error.message : 'Suppression impossible'),
          })
        }
        className="shrink-0 cursor-pointer text-[#a2a3a7] opacity-0 transition-opacity group-hover:opacity-100 hover:text-[#e5484d] focus-visible:opacity-100"
      >
        <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.6} />
      </button>
    </div>
  )
}
