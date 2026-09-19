import { Notification03Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { TASK_STATUS } from '@/features/projects/format'
import { Avatars } from '@/features/projects/ui'
import {
  notificationFeedQuery,
  useMarkAllNotificationsRead,
  useMarkNotificationRead,
} from '@/features/notifications/api'
import { cn } from '@/lib/utils'
import type { AppNotification, TaskStatus } from '@/types/api'

const WHEN = new Intl.RelativeTimeFormat('fr-FR', { numeric: 'auto' })

/** Duree ecoulee, en mots. */
function since(iso: string): string {
  const seconds = Math.round((Date.parse(iso) - Date.now()) / 1000)
  const minutes = Math.round(seconds / 60)

  if (minutes > -1) return "à l'instant"
  if (minutes > -60) return WHEN.format(minutes, 'minute')

  const hours = Math.round(minutes / 60)
  if (hours > -24) return WHEN.format(hours, 'hour')

  return WHEN.format(Math.round(hours / 24), 'day')
}

function statusLabel(value: unknown): string {
  return typeof value === 'string' && value in TASK_STATUS
    ? TASK_STATUS[value as TaskStatus].label
    : '—'
}

/**
 * La phrase d'une notification.
 *
 * Ecrite ici et non par le serveur : celui-ci dit ce qui s'est passe et avec
 * quoi, l'ecran le raconte dans sa langue. Meme partage que le journal d'une
 * tache, et il permet de changer une tournure sans migration.
 */
function sentence(item: AppNotification): ReactNode {
  const who = item.actor === null ? 'Quelqu’un' : item.actor.firstname
  const title = typeof item.payload.title === 'string' ? item.payload.title : 'une tâche'

  switch (item.kind) {
    case 'task_assigned':
      return `${who} vous a assigné « ${title} »`
    case 'task_unassigned':
      return `${who} vous a retiré de « ${title} »`
    case 'task_created':
      return `${who} a créé « ${title} »`
    case 'task_status_changed':
      return `${who} a passé « ${title} » en ${statusLabel(item.payload.to)}`
    case 'task_due_changed':
      return `${who} a changé l’échéance de « ${title} »`
    case 'task_commented':
      return `${who} a commenté « ${title} »`
    case 'project_created':
      return `${who} a créé le projet « ${title} »`
  }
}

/**
 * Cloche des notifications.
 *
 * Le compteur porte sur les non-lues, pas sur le total : ce qu'on veut savoir
 * d'un coup d'oeil, c'est ce qui reste a regarder.
 */
export function NotificationBell() {
  const { data: feed } = useQuery(notificationFeedQuery)
  const markAll = useMarkAllNotificationsRead()
  const markOne = useMarkNotificationRead()

  const items = feed?.items ?? []
  const unread = feed?.unread ?? 0

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={unread === 0 ? 'Notifications' : `Notifications, ${unread} non lues`}
        className="border-surface-sunken relative flex cursor-pointer items-center justify-center rounded-[12px] border bg-white p-2.5 outline-none transition-colors hover:bg-[#f8f8f8]"
      >
        <HugeiconsIcon
          icon={Notification03Icon}
          size={20}
          strokeWidth={1.6}
          className="text-[#111]"
        />

        {/* Le point de la maquette, pose sur l'angle du glyphe et cercle de
            blanc pour se detacher du trait de la cloche. Il ne s'allume que
            s'il y a quelque chose a lire : le compte exact se lit dans le
            panneau, la cloche dit seulement qu'il faut l'ouvrir. */}
        {unread > 0 && (
          <span
            aria-hidden
            className="absolute top-[9px] right-[10px] size-1.5 rounded-full bg-brand ring-2 ring-white"
          />
        )}
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" side="bottom" className="w-[360px] p-0">
        <header className="flex items-center justify-between gap-2 border-b border-[#e8e8e9] px-3 py-2.5">
          <span className="text-[14px] font-medium text-[#1b1b1b]">Notifications</span>

          {unread > 0 && (
            <button
              type="button"
              onClick={() => markAll.mutate()}
              className="cursor-pointer text-[12px] text-brand transition-opacity hover:opacity-80"
            >
              Tout marquer comme lu
            </button>
          )}
        </header>

        <div className="flex max-h-[420px] flex-col overflow-y-auto">
          {items.length === 0 && (
            <p className="px-3 py-8 text-center text-[13px] text-[#73757c]">
              Rien pour l’instant.
            </p>
          )}

          {items.map((item) => (
            <NotificationRow
              key={item.id}
              item={item}
              onOpen={() => {
                if (item.read_at === null) markOne.mutate(item.id)
              }}
            />
          ))}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function NotificationRow({ item, onOpen }: { item: AppNotification; onOpen: () => void }) {
  const unread = item.read_at === null

  const body = (
    <>
      {item.actor === null ? (
        <span aria-hidden className="size-7 shrink-0 rounded-full bg-[#e8e8e9]" />
      ) : (
        <Avatars people={[item.actor]} max={1} size={28} />
      )}

      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-[13px] leading-[1.45] break-words text-[#1b1b1b]">
          {sentence(item)}
        </span>
        <span className="text-[11px] text-[#a2a3a7]">{since(item.created_at)}</span>
      </span>

      {unread && <span aria-hidden className="mt-1.5 size-2 shrink-0 rounded-full bg-brand" />}
    </>
  )

  const className = cn(
    'flex w-full items-start gap-2.5 border-b border-[#f3f4f4] px-3 py-2.5 text-left transition-colors last:border-b-0',
    unread ? 'bg-[#fff8f4] hover:bg-[#fff2ea]' : 'hover:bg-[#f8f8f8]',
  )

  // Une notification qui ne designe plus rien reste lisible : son projet a pu
  // etre supprime entre-temps, et un lien mort vaut moins qu'un texte simple.
  if (item.project_id === null) {
    return (
      <button type="button" onClick={onOpen} className={cn(className, 'cursor-pointer')}>
        {body}
      </button>
    )
  }

  return (
    <Link
      to="/pm/projets/$id/taches"
      params={{ id: item.project_id }}
      search={{ vue: 'kanban', tache: item.task_id ?? undefined }}
      onClick={onOpen}
      className={className}
    >
      {body}
    </Link>
  )
}
