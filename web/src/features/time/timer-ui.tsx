import { PlayIcon, StopIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { formatDuration } from '@/features/time/format'
import { timerQuery, useDiscardTimer, useStartTimer, useStopTimer } from '@/features/time/timer'
import { HttpError } from '@/lib/api'
import { can, sessionQuery } from '@/lib/auth'
import { cn } from '@/lib/utils'

/** « 0:12:34 » depuis un instant de depart, remis a jour chaque seconde. */
function useElapsed(startedAt: string | undefined): string {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (startedAt === undefined) return
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [startedAt])

  if (startedAt === undefined) return ''
  const seconds = Math.max(0, Math.floor((now - new Date(startedAt).getTime()) / 1000))
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

/**
 * Le chrono qui tourne, dans l'en-tete de chaque ecran : on sait toujours sur
 * quoi le temps court, et on l'arrete d'ou l'on est. Rien quand rien ne tourne.
 */
export function TimerIndicator() {
  const { data: session } = useQuery(sessionQuery)
  const allowed = can(session, 'time.write')
  const { data: timer } = useQuery({ ...timerQuery, enabled: allowed })
  const stop = useStopTimer()
  const discard = useDiscardTimer()
  const elapsed = useElapsed(timer?.started_at)

  if (!allowed || timer == null) return null

  const label = timer.task === null ? timer.project.name : timer.task.name

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`Chrono en cours sur ${label}, ${elapsed}`}
          className="flex h-9 cursor-pointer items-center gap-2 rounded-[10px] border border-[#ffd9c2] bg-[#fff2ea] px-2.5 text-[13px] text-[#b84a0c] transition-colors hover:bg-[#ffe9db]"
        >
          <span aria-hidden className="size-1.5 animate-pulse rounded-full bg-brand motion-reduce:animate-none" />
          <span className="font-medium tabular-nums">{elapsed}</span>
          <span className="hidden max-w-[180px] truncate sm:inline">{label}</span>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <div className="px-2 py-1.5 text-[12px] text-[#73757c]">
          <p className="truncate font-medium text-[#1b1b1b]">{label}</p>
          {timer.task !== null && <p className="truncate">{timer.project.name}</p>}
        </div>
        <DropdownMenuItem
          disabled={stop.isPending}
          onSelect={() =>
            stop.mutate(undefined, {
              onSuccess: (entry) =>
                toast.success(entry == null ? 'Chrono arrêté' : `${formatDuration(entry.minutes)} saisie sur « ${label} »`),
              onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Arrêt impossible'),
            })
          }
        >
          <HugeiconsIcon icon={StopIcon} size={16} strokeWidth={1.8} />
          Arrêter et enregistrer
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/pm/temps/saisie">Ouvrir la feuille de temps</Link>
        </DropdownMenuItem>
        <DropdownMenuItem
          variant="destructive"
          disabled={discard.isPending}
          onSelect={() => discard.mutate(undefined, { onSuccess: () => toast.success('Chrono abandonné') })}
        >
          Abandonner sans enregistrer
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/**
 * Bouton « Démarrer » sur une tache ou un ticket. Quand le chrono tourne deja
 * sur cette chose, il propose de l'arreter ; sur une autre, de basculer — le
 * temps de l'autre est enregistre au passage.
 */
export function TimerButton({
  projectId,
  taskId = null,
  note = '',
  size = 'sm',
  className,
}: {
  projectId: string
  taskId?: string | null
  note?: string
  size?: 'sm' | 'xs'
  className?: string
}) {
  const { data: session } = useQuery(sessionQuery)
  const allowed = can(session, 'time.write')
  const { data: timer } = useQuery({ ...timerQuery, enabled: allowed })
  const start = useStartTimer()
  const stop = useStopTimer()
  const elapsed = useElapsed(timer?.started_at)

  if (!allowed) return null

  const runningHere =
    timer != null && timer.project.id === projectId && (timer.task?.id ?? null) === taskId
  const pending = start.isPending || stop.isPending
  const fail = (error: unknown) => toast.error(error instanceof HttpError ? error.message : 'Chrono impossible')

  if (runningHere) {
    return (
      <button
        type="button"
        disabled={pending}
        onClick={() =>
          stop.mutate(undefined, {
            onSuccess: (entry) => toast.success(entry == null ? 'Chrono arrêté' : `${formatDuration(entry.minutes)} enregistrée`),
            onError: fail,
          })
        }
        className={cn(
          'flex cursor-pointer items-center gap-1.5 rounded-[8px] border border-[#ffd9c2] bg-[#fff2ea] font-medium text-[#b84a0c] transition-colors hover:bg-[#ffe9db]',
          size === 'sm' ? 'h-8 px-2.5 text-[13px]' : 'h-7 px-2 text-[12px]',
          className,
        )}
      >
        <HugeiconsIcon icon={StopIcon} size={14} strokeWidth={2} />
        <span className="tabular-nums">{elapsed}</span>
      </button>
    )
  }

  return (
    <button
      type="button"
      disabled={pending}
      title={timer == null ? 'Démarrer le chrono' : 'Basculer le chrono ici (le temps en cours est enregistré)'}
      onClick={() => start.mutate({ project_id: projectId, task_id: taskId, note }, { onError: fail })}
      className={cn(
        'flex cursor-pointer items-center gap-1.5 rounded-[8px] border border-[#e8e8e9] bg-white text-[#1b1b1b] transition-colors hover:border-[#c9cacc] hover:bg-[#fafafa]',
        size === 'sm' ? 'h-8 px-2.5 text-[13px]' : 'h-7 px-2 text-[12px]',
        className,
      )}
    >
      <HugeiconsIcon icon={PlayIcon} size={14} strokeWidth={2} className="text-brand" />
      {timer == null ? 'Démarrer' : 'Basculer ici'}
    </button>
  )
}
