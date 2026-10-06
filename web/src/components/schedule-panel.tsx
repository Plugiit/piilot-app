import { ArrowLeft01Icon, ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { AnimatePresence, motion, useReducedMotion } from 'framer-motion'
import { useMemo, useState } from 'react'

import { planningQuery } from '@/features/milestones/api'
import { toDay } from '@/features/time/period'
import { cn } from '@/lib/utils'
import type { PlanningItem } from '@/types/api'

const WEEKDAY_FORMAT = new Intl.DateTimeFormat('fr-FR', { weekday: 'short' })
const MONTH_FORMAT = new Intl.DateTimeFormat('fr-FR', { month: 'long', year: 'numeric' })
const DAY_FORMAT = new Intl.DateTimeFormat('fr-FR', { weekday: 'long', day: 'numeric', month: 'long' })

/** Genres d'entree : libelle et teinte du filet, ceux de la maquette. */
const KINDS: Record<PlanningItem['kind'], { label: string; color: string }> = {
  milestone: { label: 'JALON', color: '#cf029f' },
  project_due: { label: 'ÉCHÉANCE', color: '#6060d8' },
  task_due: { label: 'MA TÂCHE', color: '#00ac47' },
}

/** Filtres de la barre. `kind` nul laisse tout passer. */
const FILTERS: { label: string; kind: PlanningItem['kind'] | null }[] = [
  { label: 'Tout', kind: null },
  { label: 'Jalons', kind: 'milestone' },
  { label: 'Échéances', kind: 'project_due' },
  { label: 'Mes tâches', kind: 'task_due' },
]

/** La semaine de lundi a dimanche qui contient `date`. */
function weekOf(date: Date) {
  const monday = new Date(date)
  // getDay() commence au dimanche : le decalage ramene au lundi precedent.
  monday.setDate(date.getDate() - ((date.getDay() + 6) % 7))

  return Array.from({ length: 7 }, (_, index) => {
    const day = new Date(monday)
    day.setDate(monday.getDate() + index)
    return day
  })
}

function isSameDay(a: Date, b: Date) {
  return a.getDate() === b.getDate() && a.getMonth() === b.getMonth() && a.getFullYear() === b.getFullYear()
}

/** Bouton de navigation de mois. */
function MonthArrow({ direction, onClick }: { direction: 'previous' | 'next'; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={direction === 'previous' ? 'Mois précédent' : 'Mois suivant'}
      className="flex size-[26px] shrink-0 cursor-pointer items-center justify-center rounded-[6px] border border-[#e6e6e6] bg-white text-[#111] transition-colors hover:bg-[#f0f0f0]"
    >
      <HugeiconsIcon icon={direction === 'previous' ? ArrowLeft01Icon : ArrowRight01Icon} size={16} strokeWidth={2} />
    </button>
  )
}

/**
 * Agenda du tableau de bord : la semaine du jour choisi, et ce qui tombe ce
 * jour-la — les jalons et les echeances des projets de l'agence, et ses
 * propres taches.
 *
 * Il lit le planning (`GET /admin/planning`), une requete par semaine
 * affichee : changer de jour dans la meme semaine ne redemande rien. Chaque
 * entree mene la ou l'on agit — les jalons du projet, sa fiche, la tache.
 */
export function SchedulePanel({ className }: { className?: string }) {
  const [selected, setSelected] = useState(() => new Date())

  // La semaine se deduit du jour choisi : changer de mois deplace le jour, et
  // la bande suit.
  const week = useMemo(() => weekOf(selected), [selected])
  const from = toDay(week[0]!)
  const to = toDay(week[6]!)
  const { data, isPending } = useQuery(planningQuery(from, to, 'all'))

  // Le sens du glissement suit celui de la lecture : avancer dans la semaine
  // fait entrer la liste par la droite, reculer par la gauche.
  const [direction, setDirection] = useState(1)
  const reduced = useReducedMotion()
  const [filter, setFilter] = useState<PlanningItem['kind'] | null>(null)

  const byDay = useMemo(() => {
    const map = new Map<string, PlanningItem[]>()
    for (const item of data?.items ?? []) map.set(item.day, [...(map.get(item.day) ?? []), item])
    return map
  }, [data])

  const today = toDay(new Date())
  const selectedDay = toDay(selected)
  const events = (byDay.get(selectedDay) ?? []).filter((item) => filter === null || item.kind === filter)

  const shift = reduced ? 0 : 20
  const transition = reduced ? ({ duration: 0 } as const) : ({ type: 'spring', visualDuration: 0.25, bounce: 0 } as const)

  // La bande ne se rejoue que si la semaine change : choisir un autre jour de
  // la meme semaine ne doit pas la faire glisser sous le doigt.
  const weekKey = from

  function selectDay(date: Date) {
    setDirection(date.getTime() < selected.getTime() ? -1 : 1)
    setSelected(date)
  }

  // Changer de filtre ne va ni en avant ni en arriere : la liste fond, la
  // faire glisser suggererait un deplacement dans le temps.
  function selectFilter(kind: PlanningItem['kind'] | null) {
    setDirection(0)
    setFilter(kind)
  }

  // On atterrit sur le premier du mois vise plutot que sur le meme quantieme :
  // le 31 n'existe pas partout.
  function goToMonth(offset: number) {
    setDirection(offset)
    setSelected(new Date(selected.getFullYear(), selected.getMonth() + offset, 1))
  }

  return (
    <aside className={cn('flex flex-col gap-3 bg-white p-4', className)}>
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-[16px] font-medium text-[#111]">Agenda</h2>
        <Link to="/pm/planning" className="text-[12px] text-[#73757c] hover:text-[#111] hover:underline">
          Planning complet
        </Link>
      </div>

      <div className="border-surface-sunken flex flex-col gap-3 border-b pb-4">
        <div className="bg-surface flex items-center justify-between gap-2 rounded-[12px] px-2 py-2.5">
          <MonthArrow direction="previous" onClick={() => goToMonth(-1)} />
          <p className="truncate text-[14px] font-semibold text-[#1f2937] first-letter:uppercase">
            {MONTH_FORMAT.format(selected)}
          </p>
          <MonthArrow direction="next" onClick={() => goToMonth(1)} />
        </div>

        {/* La bande glisse quand on change de semaine ; `overflow-hidden` la
            borne au panneau pendant le trajet. Un point sous un jour dit
            qu'il porte quelque chose, sans avoir a cliquer pour le savoir. */}
        <div className="overflow-hidden">
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={weekKey}
              initial={{ opacity: 0, x: shift * direction }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -shift * direction, transition: { duration: 0.12 } }}
              transition={transition}
              className="grid grid-cols-7 gap-1"
            >
              {week.map((date) => {
                const active = isSameDay(date, selected)
                const busy = (byDay.get(toDay(date))?.length ?? 0) > 0

                return (
                  <div key={date.toISOString()} className="flex flex-col items-center gap-1.5">
                    <p className="text-[12px] leading-[1.4] font-semibold text-[#444]">
                      {WEEKDAY_FORMAT.format(date).replace('.', '')}
                    </p>
                    <button
                      type="button"
                      aria-pressed={active}
                      aria-label={DAY_FORMAT.format(date)}
                      onClick={() => selectDay(date)}
                      className={cn(
                        'flex size-8 cursor-pointer items-center justify-center rounded-full text-center text-[14px] leading-[1.5] font-semibold transition-colors',
                        active
                          ? 'bg-white text-[#ff782b] drop-shadow-[0px_4px_2px_rgba(0,0,0,0.06)]'
                          : toDay(date) === today
                            ? 'text-[#ff782b] hover:bg-[#f0f0f0]'
                            : 'text-[#111] hover:bg-[#f0f0f0]',
                      )}
                    >
                      {date.getDate()}
                    </button>
                    <span
                      aria-hidden
                      className={cn('size-1 rounded-full', busy ? 'bg-[#ff782b]' : 'bg-transparent')}
                    />
                  </div>
                )
              })}
            </motion.div>
          </AnimatePresence>
        </div>

        {/* Les marges negatives rendent au defilement les 16px du panneau : la
            premiere et la derniere pilule ne restent pas coupees en butee. */}
        <div className="-mx-4 flex gap-3 overflow-x-auto px-4 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          {FILTERS.map((entry) => {
            const active = entry.kind === filter

            return (
              <button
                key={entry.label}
                type="button"
                aria-pressed={active}
                onClick={() => selectFilter(entry.kind)}
                className={cn(
                  'shrink-0 cursor-pointer rounded-[12px] border px-4 py-2 text-[12px] font-medium whitespace-nowrap transition-colors',
                  active
                    ? 'border-[#ff782b] bg-[#ff782b] text-white'
                    : 'border-surface-sunken bg-white text-[#111] hover:bg-[#f8f8f8]',
                )}
              >
                {entry.label}
              </button>
            )
          })}
        </div>
      </div>

      <AnimatePresence mode="wait" initial={false}>
        <motion.div
          key={`${selectedDay}-${filter ?? 'tout'}-${isPending ? 'chargement' : 'pret'}`}
          initial={{ opacity: 0, x: shift * direction }}
          animate={{ opacity: 1, x: 0 }}
          exit={{ opacity: 0, x: -shift * direction, transition: { duration: 0.12 } }}
          transition={transition}
          className="flex flex-col gap-5"
        >
          {isPending ? (
            <div className="flex flex-col gap-3">
              <span className="h-12 animate-pulse rounded-[8px] bg-[#f3f4f4]" />
              <span className="h-12 animate-pulse rounded-[8px] bg-[#f3f4f4]" />
            </div>
          ) : events.length === 0 ? (
            <p className="text-[14px] text-[#111]/50">
              {filter === null ? 'Rien de prévu ce jour-là.' : 'Rien de ce type ce jour-là.'}
            </p>
          ) : (
            events.map((item) => <AgendaEntry key={`${item.kind}:${item.id}`} item={item} today={today} />)
          )}
        </motion.div>
      </AnimatePresence>
    </aside>
  )
}

/** Une entree de l'agenda, qui mene la ou l'on agit. */
function AgendaEntry({ item, today }: { item: PlanningItem; today: string }) {
  const { label, color } = KINDS[item.kind]
  const late = !item.done && item.day < today

  const detail =
    item.kind === 'milestone' && item.deliverables_total > 0
      ? `${item.project.name} · ${item.deliverables_validated}/${item.deliverables_total} livrables validés`
      : item.project.name

  const body = (
    <>
      {/* Le filet de la maquette : un trait de 2px coiffe d'un demi-disque,
          dans la teinte du genre d'entree. */}
      <span aria-hidden className="relative w-[10px] shrink-0 self-stretch">
        <span className="absolute top-0 left-0 h-3 w-[6px] rounded-l-full" style={{ backgroundColor: color }} />
        <span className="absolute top-[7px] bottom-0 left-[4px] w-[2px]" style={{ backgroundColor: color }} />
      </span>

      <span className="flex min-w-0 flex-col gap-1.5">
        <span className="flex items-center gap-2">
          <span className="text-[12px] font-medium tracking-[0.48px]" style={{ color }}>
            {label}
          </span>
          {item.done && <span className="text-[11px] text-[#0db471]">Terminé</span>}
          {late && <span className="text-[11px] text-[#e5484d]">En retard</span>}
        </span>
        <span className="flex flex-col gap-0.5">
          <span
            className={cn(
              'text-[14px] font-semibold text-[#111]',
              item.done && 'text-[#111]/50 line-through decoration-[#c4c4c4]',
            )}
          >
            {item.title}
          </span>
          <span className="truncate text-[13px] leading-[1.5] text-[#111]/70">{detail}</span>
        </span>
      </span>
    </>
  )

  const className = '-mx-2 flex w-[calc(100%+1rem)] gap-1 rounded-[8px] px-2 py-1 text-left transition-colors hover:bg-[#f8f8f8]'

  if (item.kind === 'task_due') {
    return (
      <Link to="/pm/projets/$id/taches" params={{ id: item.project.id }} search={{ tache: item.id }} className={className}>
        {body}
      </Link>
    )
  }

  return (
    <Link
      to={item.kind === 'milestone' ? '/pm/projets/$id/jalons' : '/pm/projets/$id'}
      params={{ id: item.project.id }}
      className={className}
    >
      {body}
    </Link>
  )
}
