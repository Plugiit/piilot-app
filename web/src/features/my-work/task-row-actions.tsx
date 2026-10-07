import { Clock01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { toast } from 'sonner'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useWorkTaskMove } from '@/features/my-work/api'
import { TASK_STATUS, TASK_STATUS_ORDER } from '@/features/projects/format'
import { StatusPill } from '@/features/projects/ui'
import { useCreateTimeEntry } from '@/features/time/api'
import { formatDuration, parseDuration } from '@/features/time/format'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { TaskStatus, WorkTask } from '@/types/api'

/**
 * Les gestes d'une tache depuis « Mon travail », sans l'ouvrir : la cocher,
 * changer son statut, poser le temps du jour. Ce sont les trois choses qu'on
 * fait dix fois par jour ; les faire d'ici epargne autant d'allers-retours.
 */

/** Case a cocher : terminer, ou rouvrir. */
export function WorkTaskCheckbox({ task }: { task: WorkTask }) {
  const move = useWorkTaskMove()
  const done = (task.status as string) === 'done'

  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={done}
      aria-label={done ? `Rouvrir ${task.title}` : `Terminer ${task.title}`}
      disabled={move.isPending}
      onClick={() =>
        move.mutate(
          { id: task.id, status: done ? 'todo' : 'done' },
          {
            onSuccess: () => {
              if (!done) toast.success('Tâche terminée')
            },
            onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Changement impossible'),
          },
        )
      }
      className={cn(
        'flex size-5 shrink-0 cursor-pointer items-center justify-center rounded-[6px] border transition-colors',
        done ? 'border-brand bg-brand text-white' : 'border-[#e8e8e9] bg-white hover:border-[#c9cacc]',
      )}
    >
      {done && (
        <svg viewBox="0 0 20 20" className="size-3" fill="none" aria-hidden>
          <path d="M4 10.5 8 14.5 16 6" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      )}
    </button>
  )
}

/** Pastille de statut qui se change d'un clic. */
export function WorkTaskStatus({ task, className }: { task: WorkTask; className?: string }) {
  const move = useWorkTaskMove()
  const status = TASK_STATUS[task.status]

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button type="button" aria-label={`Statut : ${status.label}, changer`} className={cn('cursor-pointer rounded-full', className)}>
          <StatusPill label={status.label} color={status.color} pill={status.pill} />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        {TASK_STATUS_ORDER.map((value: TaskStatus) => (
          <DropdownMenuItem
            key={value}
            disabled={value === task.status || move.isPending}
            onSelect={() =>
              move.mutate(
                { id: task.id, status: value },
                { onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Changement impossible') },
              )
            }
          >
            <span className="size-2 shrink-0 rounded-full" style={{ background: TASK_STATUS[value].color }} />
            {TASK_STATUS[value].label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** Poser du temps sur la tache, pour aujourd'hui : « 1h30 », Entree. */
export function WorkTaskTime({ task, today }: { task: WorkTask; today: string }) {
  const [open, setOpen] = useState(false)
  const [raw, setRaw] = useState('')
  const create = useCreateTimeEntry()
  const minutes = parseDuration(raw)

  function submit() {
    if (minutes === null || minutes <= 0) return

    create.mutate(
      { project_id: task.project.id, task_id: task.id, spent_on: today, minutes },
      {
        onSuccess: () => {
          toast.success(`${formatDuration(minutes)} saisie sur « ${task.title} »`)
          setRaw('')
          setOpen(false)
        },
        onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Saisie impossible'),
      },
    )
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={`Saisir du temps sur ${task.title}`}
          title="Saisir le temps du jour"
          className="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-[6px] text-[#8d8d8d] transition-colors hover:bg-[#f3f4f4] hover:text-[#1b1b1b]"
        >
          <HugeiconsIcon icon={Clock01Icon} size={16} strokeWidth={1.8} />
        </button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[220px] p-2">
        <p className="px-1 pb-1.5 text-[12px] text-[#73757c]">Temps passé aujourd’hui</p>
        <div className="flex items-center gap-1.5">
          <input
            autoFocus
            value={raw}
            onChange={(event) => setRaw(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault()
                submit()
              }
            }}
            placeholder="1h30"
            aria-label="Durée"
            className="h-8 min-w-0 flex-1 rounded-[6px] border border-input px-2 text-[14px] tabular-nums focus:border-ring focus:outline-none"
          />
          <button
            type="button"
            disabled={minutes === null || minutes <= 0 || create.isPending}
            onClick={submit}
            className="h-8 shrink-0 cursor-pointer rounded-[6px] bg-brand px-2.5 text-[13px] font-medium text-white disabled:cursor-default disabled:opacity-40"
          >
            OK
          </button>
        </div>
        {raw !== '' && minutes === null && <p className="px-1 pt-1 text-[11px] text-[#e5484d]">Ex. : 45m, 1h30, 2h</p>}
      </PopoverContent>
    </Popover>
  )
}
