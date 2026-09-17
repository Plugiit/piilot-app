import {
  Building03Icon,
  Calendar03Icon,
  FolderOpenIcon,
  LinkSquare02Icon,
  RefreshIcon,
  TextAlignLeftIcon,
  UserIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { Avatars, StatusPill } from '@/features/projects/ui'
import { DELIVERABLE_STATUS, attenteDepuis } from '@/features/deliverables/format'
import { DeliverableRowActions } from '@/features/deliverables/row-actions'
import { formatDateTime } from '@/features/tickets/format'
import { cn } from '@/lib/utils'
import type { Deliverable } from '@/types/api'

/**
 * Vue liste des livrables.
 *
 * Meme gabarit que les tableaux des tickets et des taches : construit en flex
 * et non en `<table>`, avec un defilement horizontal quand la somme des
 * minimums ne tient plus.
 */
interface Column {
  key: string
  label: string
  icon?: IconSvgElement
  width: string
  cell: (item: Deliverable) => ReactNode
}

const COLUMNS: Column[] = [
  {
    key: 'title',
    label: 'Livrable',
    icon: TextAlignLeftIcon,
    width: 'min-w-[220px] flex-[2_1_220px]',
    // Le titre mene au livrable lui-meme : c'est le seul geste qu'on fait avec
    // une ligne avant de repondre. Un livrable sans version n'a rien a ouvrir.
    cell: (item) =>
      item.version === null || item.version.url === '' ? (
        <span className="truncate text-[14px] text-[#1b1b1b]" title={item.title}>
          {item.title}
        </span>
      ) : (
        <a
          href={item.version.url}
          target="_blank"
          rel="noreferrer"
          title={item.version.url}
          className="flex min-w-0 items-center gap-1.5 text-[14px] text-[#1b1b1b] hover:underline"
        >
          <span className="truncate">{item.title}</span>
          <HugeiconsIcon
            icon={LinkSquare02Icon}
            size={14}
            strokeWidth={1.8}
            className="shrink-0 text-[#a2a3a7]"
          />
        </a>
      ),
  },
  {
    key: 'project',
    label: 'Projet',
    icon: FolderOpenIcon,
    width: 'min-w-[150px] flex-[1_1_150px]',
    cell: (item) => (
      <Link
        to="/pm/projets/$id"
        params={{ id: item.project.id }}
        className="truncate text-[14px] text-[#1b1b1b] hover:underline"
      >
        {item.project.name}
      </Link>
    ),
  },
  {
    key: 'client',
    label: 'Client',
    icon: Building03Icon,
    width: 'min-w-[140px] flex-[1_1_140px]',
    cell: (item) => (
      <Link
        to="/crm/clients/$id"
        params={{ id: item.client.id }}
        className="truncate text-[14px] text-[#73757c] hover:underline"
      >
        {item.client.name}
      </Link>
    ),
  },
  {
    key: 'status',
    label: 'État',
    icon: RefreshIcon,
    width: 'w-[130px] shrink-0',
    // Le retour du client sert d'infobulle : c'est lui qui dit quoi corriger,
    // et la colonne n'a pas la place de le porter en entier.
    cell: (item) => {
      const status = DELIVERABLE_STATUS[item.status]!
      const feedback = item.version?.feedback ?? ''

      return (
        <span title={feedback === '' ? undefined : feedback}>
          <StatusPill label={status.label} color={status.pill.text} pill={status.pill} />
        </span>
      )
    },
  },
  {
    key: 'version',
    label: 'Version',
    width: 'w-[80px] shrink-0',
    cell: (item) => (
      <span className="text-[14px] text-[#73757c] tabular-nums">
        {item.version === null ? '—' : `v${item.version.numero}`}
      </span>
    ),
  },
  {
    key: 'submitted',
    label: 'Soumis le',
    icon: Calendar03Icon,
    width: 'w-[165px] shrink-0',
    // L'anciennete ne s'affiche que sur ce qui attend : sur un livrable
    // tranche, elle ne dit plus rien d'actionnable.
    cell: (item) => {
      if (item.version === null) return <span className="text-[13px] text-[#a2a3a7]">—</span>

      const depuis = item.status === 'en_attente' ? attenteDepuis(item.version.submitted_at) : ''

      return (
        <span className="flex min-w-0 items-center gap-1.5">
          <span className="text-[13px] text-[#73757c] tabular-nums">
            {formatDateTime(item.version.submitted_at)}
          </span>
          {depuis !== '' && (
            <span
              title={`En attente depuis ${depuis}`}
              className="shrink-0 rounded-full bg-[#fff1d4] px-1.5 text-[11px] font-medium text-[#9a6a00]"
            >
              {depuis}
            </span>
          )}
        </span>
      )
    },
  },
  {
    key: 'submitter',
    label: 'Soumis par',
    icon: UserIcon,
    width: 'w-[150px] shrink-0',
    cell: (item) => {
      if (item.version?.submitter == null) {
        return <span className="text-[14px] text-[#a2a3a7]">—</span>
      }

      const nom =
        `${item.version.submitter.firstname} ${item.version.submitter.lastname}`.trim() ||
        'Sans nom'

      return (
        <>
          <Avatars people={[item.version.submitter]} max={1} size={24} />
          <span className="truncate text-[14px] text-[#1b1b1b]" title={nom}>
            {nom}
          </span>
        </>
      )
    },
  },
  {
    key: 'actions',
    label: '',
    width: 'w-[56px] shrink-0 justify-end',
    cell: (item) => <DeliverableRowActions item={item} />,
  },
]

/** Une cellule : meme gabarit dans l'en-tete et dans les rangees. */
function Cell({
  width,
  className,
  children,
}: {
  width: string
  className?: string
  children: ReactNode
}) {
  return (
    <div
      className={cn(
        'flex items-center gap-2 border-b border-[#e8e8e9] px-2.5 py-3',
        width,
        className,
      )}
    >
      {children}
    </div>
  )
}

export function DeliverableTable({
  items,
  empty,
}: {
  items: Deliverable[]
  empty: ReactNode
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-x-auto">
      <div className="flex min-w-max flex-col">
        <div className="flex items-stretch">
          {COLUMNS.map((column) => (
            <Cell key={column.key} width={column.width} className="bg-[#f3f4f4]">
              {column.icon !== undefined && (
                <HugeiconsIcon
                  icon={column.icon}
                  size={20}
                  strokeWidth={1.6}
                  className="shrink-0 text-[#73757c]"
                />
              )}
              <span className="text-[14px] text-[#73757c]">{column.label}</span>
            </Cell>
          ))}
        </div>

        {items.length === 0 && (
          <p className="border-b border-[#e8e8e9] px-2.5 py-8 text-center text-[14px] text-[#73757c]">
            {empty}
          </p>
        )}

        {items.map((item) => (
          <div key={item.id} className="flex items-stretch bg-white">
            {COLUMNS.map((column) => (
              <Cell key={column.key} width={column.width}>
                {column.cell(item)}
              </Cell>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
