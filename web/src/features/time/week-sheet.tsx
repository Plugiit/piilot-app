import { ArrowReloadHorizontalIcon, PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { projectListQuery } from '@/features/projects/api'
import { taskBoardQuery } from '@/features/tasks/api'
import {
  timeSheetQuery,
  useCreateTimeEntry,
  useDeleteTimeEntry,
  useUpdateTimeEntry,
} from '@/features/time/api'
import { formatDuration, parseDuration } from '@/features/time/format'
import {
  buildWeekRows,
  dayTotals,
  rowKey,
  weekDays,
  type WeekRef,
  type WeekRow,
} from '@/features/time/week'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { TimeEntry } from '@/types/api'

const HEAD = new Intl.DateTimeFormat('fr-FR', { weekday: 'short', day: 'numeric' })

/** Valeur du choix « aucune tache » : Radix refuse la chaine vide. */
const AUCUNE = 'aucune'

/**
 * Feuille de la semaine : une rangee par projet et par tache, une cellule par
 * jour.
 *
 * Pour qui pointe le vendredi ce qu'il a fait de la semaine, plutot que chaque
 * soir. Une cellule vide ou tenant une seule saisie se modifie sur place ; une
 * cellule qui en regroupe plusieurs se lit seulement — les fondre en une
 * effacerait leurs notes — et mene a la journee pour les reprendre une a une.
 */
export function WeekSheet({ monday, onOpenDay }: { monday: string; onOpenDay: (day: string) => void }) {
  const days = weekDays(monday)
  const sunday = days[6]!
  const { data, isError, error } = useQuery(timeSheetQuery(monday, sunday))

  // Rangees ouvertes a la main, en attente d'une premiere saisie. Elles
  // s'effacent d'elles-memes : des qu'une saisie existe, la rangee vient des
  // donnees.
  const [extra, setExtra] = useState<{ project: WeekRef; task: WeekRef | null }[]>([])

  if (isError) {
    return (
      <p className="rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
        {error instanceof HttpError ? error.message : 'Chargement impossible'}
      </p>
    )
  }

  const rows = buildWeekRows(data?.items ?? [], days, extra)
  const totals = dayTotals(rows)
  const week = totals.reduce((a, b) => a + b, 0)
  const today = days.find((day) => day === new Date().toLocaleDateString('sv-SE'))

  const columns = 'minmax(220px,1fr) repeat(7, minmax(64px,84px)) 76px'

  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-x-auto rounded-[12px] border border-[#e8e8e9] bg-white">
        <div className="min-w-max" role="table" aria-label="Feuille de temps de la semaine">
          <div
            role="row"
            className="grid items-center border-b border-[#e8e8e9] bg-[#f3f4f4] px-3 py-2 text-[12px] text-[#73757c]"
            style={{ gridTemplateColumns: columns }}
          >
            <span role="columnheader">Projet et tâche</span>
            {days.map((day) => (
              <button
                key={day}
                type="button"
                role="columnheader"
                onClick={() => onOpenDay(day)}
                title="Ouvrir la journée"
                className={cn(
                  'cursor-pointer text-center capitalize transition-colors hover:text-[#1b1b1b]',
                  day === today && 'font-medium text-brand',
                )}
              >
                {HEAD.format(new Date(`${day}T12:00:00`))}
              </button>
            ))}
            <span role="columnheader" className="text-right">
              Total
            </span>
          </div>

          {rows.length === 0 && (
            <p className="px-3 py-8 text-center text-[14px] text-[#73757c]">
              Rien de pointé cette semaine. Ajoutez une ligne pour commencer.
            </p>
          )}

          {rows.map((row) => (
            <div
              key={row.key}
              role="row"
              className="grid items-center border-b border-[#f0f0f1] px-3 py-1.5 last:border-b-0"
              style={{ gridTemplateColumns: columns }}
            >
              <span role="rowheader" className="flex min-w-0 flex-col pr-2">
                <span className="truncate text-[14px] text-[#1b1b1b]">{row.project.name}</span>
                <span className="truncate text-[12px] text-[#8d8d8d]">{row.task?.name ?? 'Sans tâche'}</span>
              </span>

              {days.map((day, index) => (
                <WeekCell
                  key={`${day}:${row.cells[index]?.map((e) => `${e.id}:${e.minutes}`).join(',')}`}
                  row={row}
                  day={day}
                  entries={row.cells[index] ?? []}
                  onOpenDay={() => onOpenDay(day)}
                />
              ))}

              <span role="cell" className="text-right text-[14px] font-medium text-[#1b1b1b] tabular-nums">
                {row.total === 0 ? '—' : formatDuration(row.total)}
              </span>
            </div>
          ))}

          <div
            role="row"
            className="grid items-center border-t border-[#e8e8e9] bg-[#fafafa] px-3 py-2 text-[13px] tabular-nums"
            style={{ gridTemplateColumns: columns }}
          >
            <span role="rowheader" className="text-[#73757c]">
              Total du jour
            </span>
            {totals.map((minutes, index) => (
              <span key={days[index]} role="cell" className="text-center text-[#4b4b4f]">
                {minutes === 0 ? '—' : formatDuration(minutes)}
              </span>
            ))}
            <span role="cell" className="text-right text-[15px] font-semibold text-[#1b1b1b]">
              {formatDuration(week)}
            </span>
          </div>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <AddRow
          existing={new Set(rows.map((row) => row.key))}
          onAdd={(row) => setExtra((prev) => [...prev, row])}
        />
        <CopyPreviousWeek
          monday={monday}
          existing={new Set(rows.map((row) => row.key))}
          onRows={(added) => setExtra((prev) => [...prev, ...added])}
        />
      </div>

      <p className="text-[12px] text-[#8d8d8d]">
        Tapez une durée (90, 1h30 ou 1,5) puis Entrée ou Tab. Videz une cellule pour retirer sa saisie.
      </p>
    </div>
  )
}

/**
 * Une cellule de la grille.
 *
 * Elle enregistre en quittant le champ, sans bouton : sur une grille, Tab
 * d'une case a l'autre est le geste naturel, et confirmer chaque case le
 * casserait.
 */
function WeekCell({
  row,
  day,
  entries,
  onOpenDay,
}: {
  row: WeekRow
  day: string
  entries: TimeEntry[]
  onOpenDay: () => void
}) {
  const [only] = entries
  const initial = only === undefined ? '' : formatDuration(only.minutes)
  const [value, setValue] = useState(initial)
  const [invalid, setInvalid] = useState(false)

  const create = useCreateTimeEntry()
  const update = useUpdateTimeEntry(only?.id ?? '')
  const remove = useDeleteTimeEntry(only?.id ?? '')
  const pending = create.isPending || update.isPending || remove.isPending

  if (entries.length > 1) {
    const minutes = entries.reduce((sum, entry) => sum + entry.minutes, 0)

    return (
      <button
        type="button"
        role="cell"
        onClick={onOpenDay}
        title={`${entries.length} saisies ce jour-là — les modifier depuis la journée`}
        className="mx-1 flex h-8 cursor-pointer items-center justify-center rounded-[8px] bg-[#f3f4f4] text-[13px] text-[#1b1b1b] tabular-nums transition-colors hover:bg-[#ebebeb]"
      >
        {formatDuration(minutes)}
        <span className="ml-1 text-[10px] text-[#8d8d8d]">×{entries.length}</span>
      </button>
    )
  }

  const onError = (error: Error) => {
    setValue(initial)
    toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible')
  }

  function commit() {
    const raw = value.trim()
    if (raw === initial) return

    if (raw === '' || raw === '0') {
      setInvalid(false)
      if (only !== undefined) remove.mutate(undefined, { onError })
      return
    }

    const minutes = parseDuration(raw)
    if (minutes === null || minutes <= 0 || minutes > 24 * 60) {
      setInvalid(true)
      return
    }
    setInvalid(false)

    if (only === undefined) {
      create.mutate(
        {
          project_id: row.project.id,
          task_id: row.task?.id ?? null,
          service_id: row.service?.id ?? null,
          spent_on: day,
          minutes,
        },
        { onError },
      )
      return
    }

    // Seule la duree change : la tache, le service et la note de la saisie
    // restent les siens.
    update.mutate(
      {
        project_id: only.project.id,
        task_id: only.task?.id ?? null,
        service_id: only.service?.id ?? null,
        spent_on: only.spent_on,
        minutes,
        note: only.note,
      },
      { onError },
    )
  }

  return (
    <span role="cell" className="px-1">
      <input
        value={value}
        onChange={(event) => setValue(event.target.value)}
        onBlur={commit}
        onKeyDown={(event) => {
          if (event.key === 'Enter') event.currentTarget.blur()
          if (event.key === 'Escape') {
            setValue(initial)
            setInvalid(false)
          }
        }}
        disabled={pending}
        inputMode="decimal"
        aria-label={`${row.project.name}${row.task ? `, ${row.task.name}` : ''}, ${day}`}
        aria-invalid={invalid}
        placeholder="·"
        title={only?.note || undefined}
        className={cn(
          'h-8 w-full rounded-[8px] border border-transparent bg-transparent text-center text-[13px] text-[#1b1b1b] tabular-nums outline-none transition-colors placeholder:text-[#d0d1d3] hover:border-[#e8e8e9] focus:border-[#d0d1d3] focus:bg-white',
          only !== undefined && 'bg-[#f8f8f8]',
          invalid && 'border-[#e5484d] focus:border-[#e5484d]',
          pending && 'opacity-60',
        )}
      />
    </span>
  )
}

/**
 * Reprend les rangees de la semaine precedente — les projets et les taches,
 * pas les heures : une semaine ressemble a la precedente, et les ouvrir une a
 * une etait le geste le plus repete du lundi matin.
 */
function CopyPreviousWeek({
  monday,
  existing,
  onRows,
}: {
  monday: string
  existing: Set<string>
  onRows: (rows: { project: WeekRef; task: WeekRef | null }[]) => void
}) {
  const queryClient = useQueryClient()
  const [pending, setPending] = useState(false)

  async function copy() {
    setPending(true)
    try {
      const previous = weekDays(shiftDays(monday, -7))
      const sheet = await queryClient.fetchQuery(timeSheetQuery(previous[0]!, previous[6]!))
      const seen = new Set(existing)
      const rows: { project: WeekRef; task: WeekRef | null }[] = []
      for (const entry of sheet.items) {
        const key = rowKey(entry.project.id, entry.task?.id)
        if (seen.has(key)) continue
        seen.add(key)
        rows.push({ project: entry.project, task: entry.task })
      }
      if (rows.length === 0) {
        toast.info('Rien de nouveau à reprendre : les lignes de la semaine dernière sont déjà là.')
      } else {
        onRows(rows)
        toast.success(`${rows.length} ligne${rows.length > 1 ? 's' : ''} reprise${rows.length > 1 ? 's' : ''} de la semaine dernière`)
      }
    } catch (error) {
      toast.error(error instanceof HttpError ? error.message : 'Lecture impossible')
    } finally {
      setPending(false)
    }
  }

  return (
    <Button variant="ghost" size="sm" className="gap-1.5" disabled={pending} onClick={() => void copy()}>
      <HugeiconsIcon icon={ArrowReloadHorizontalIcon} size={14} strokeWidth={2} />
      Reprendre la semaine précédente
    </Button>
  )
}

function shiftDays(day: string, delta: number): string {
  const date = new Date(`${day}T12:00:00`)
  date.setDate(date.getDate() + delta)
  return date.toLocaleDateString('sv-SE')
}

/** Ouvre une rangee pour un projet, et une tache si l'on veut. */
function AddRow({
  existing,
  onAdd,
}: {
  existing: Set<string>
  onAdd: (row: { project: WeekRef; task: WeekRef | null }) => void
}) {
  const [open, setOpen] = useState(false)
  const [projectId, setProjectId] = useState('')
  const [taskId, setTaskId] = useState(AUCUNE)

  const { data: projects } = useQuery({
    ...projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }),
    enabled: open,
  })
  const { data: board } = useQuery({ ...taskBoardQuery(projectId), enabled: open && projectId !== '' })

  if (!open) {
    return (
      <Button variant="outline" size="sm" className="gap-1.5 self-start" onClick={() => setOpen(true)}>
        <HugeiconsIcon icon={PlusSignIcon} size={14} strokeWidth={2} />
        Ajouter une ligne
      </Button>
    )
  }

  const project = projects?.items.find((p) => p.id === projectId)
  const task = board?.items.find((t) => t.id === taskId)
  const duplicate = projectId !== '' && existing.has(rowKey(projectId, task?.id))

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-[12px] border border-[#e8e8e9] bg-white p-2">
      <Select value={projectId} onValueChange={(value) => { setProjectId(value); setTaskId(AUCUNE) }}>
        <SelectTrigger className="h-9 min-w-[200px] text-[13px]">
          <SelectValue placeholder="Projet" />
        </SelectTrigger>
        <SelectContent>
          {(projects?.items ?? []).map((p) => (
            <SelectItem key={p.id} value={p.id}>
              {p.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select value={taskId} onValueChange={setTaskId} disabled={projectId === ''}>
        <SelectTrigger className="h-9 min-w-[200px] text-[13px]">
          <SelectValue placeholder="Tâche" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={AUCUNE}>
            <span className="text-[#73757c]">Sans tâche</span>
          </SelectItem>
          {(board?.items ?? []).map((t) => (
            <SelectItem key={t.id} value={t.id}>
              {t.title}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Button
        size="sm"
        disabled={project === undefined || duplicate}
        onClick={() => {
          if (project === undefined) return
          onAdd({
            project: { id: project.id, name: project.name },
            task: task === undefined ? null : { id: task.id, name: task.title },
          })
          setProjectId('')
          setTaskId(AUCUNE)
          setOpen(false)
        }}
      >
        Ajouter
      </Button>
      <Button size="sm" variant="ghost" onClick={() => setOpen(false)}>
        Annuler
      </Button>

      {duplicate && <span className="text-[12px] text-[#73757c]">Cette ligne est déjà dans la semaine.</span>}
    </div>
  )
}
