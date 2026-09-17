import { ArrowLeft02Icon, ArrowRight02Icon, Delete02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { toast } from 'sonner'
import { z } from 'zod'

import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { useDeleteTimeEntry, timeSheetQuery } from '@/features/time/api'
import { TimeEntryForm } from '@/features/time/entry-form'
import { formatDuration } from '@/features/time/format'
import { HttpError } from '@/lib/api'
import type { TimeEntry } from '@/types/api'

/**
 * Saisie du temps.
 *
 * Un jour a la fois : on pointe le soir ce qu'on vient de faire, et une semaine
 * entiere a l'ecran donnerait a chercher la bonne ligne plutot qu'a saisir. La
 * semaine se lit dans les rapports.
 *
 * Le jour vit dans l'adresse : « ce que j'ai pointe mardi » se partage, et le
 * bouton Retour y ramene.
 */
const searchSchema = z.object({
  jour: z.string().optional(),
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

const LONG_DAY = new Intl.DateTimeFormat('fr-FR', {
  weekday: 'long',
  day: 'numeric',
  month: 'long',
})

export const Route = createFileRoute('/_app/pm/temps/saisie/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ jour: search.jour ?? today() }),
  loader: ({ context, deps }) =>
    context.queryClient.query({
      ...timeSheetQuery(deps.jour, deps.jour),
      staleTime: 'static',
    }),
  component: SaisiePage,
})

function SaisiePage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const jour = search.jour ?? today()
  const { data, isError, error } = useQuery(timeSheetQuery(jour, jour))

  const items = data?.items ?? []
  const total = data?.total_minutes ?? 0

  function goTo(next: string) {
    void navigate({ search: { jour: next === today() ? undefined : next } })
  }

  return (
    <PageFrame title="Saisie du temps">
      <div className="flex min-h-full flex-col gap-4 p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon-sm"
              aria-label="Jour précédent"
              onClick={() => goTo(shiftDay(jour, -1))}
            >
              <HugeiconsIcon icon={ArrowLeft02Icon} size={16} strokeWidth={1.8} />
            </Button>

            <p className="min-w-[190px] text-center text-[15px] font-medium text-[#1b1b1b] first-letter:uppercase">
              {LONG_DAY.format(new Date(`${jour}T12:00:00`))}
            </p>

            <Button
              variant="outline"
              size="icon-sm"
              aria-label="Jour suivant"
              onClick={() => goTo(shiftDay(jour, 1))}
            >
              <HugeiconsIcon icon={ArrowRight02Icon} size={16} strokeWidth={1.8} />
            </Button>

            {jour !== today() && (
              <Button variant="ghost" size="sm" onClick={() => goTo(today())}>
                Aujourd’hui
              </Button>
            )}
          </div>

          {/* Le total du jour, en gros : c'est la seule chose qu'on verifie
              avant de fermer l'ecran. */}
          <p className="text-[20px] font-medium text-[#1b1b1b] tabular-nums">
            {formatDuration(total)}
          </p>
        </div>

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
      </div>
    </PageFrame>
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
