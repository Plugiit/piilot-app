import {
  Alert02Icon,
  Calendar03Icon,
  Flag02Icon,
  FolderOpenIcon,
  RefreshIcon,
  TextAlignLeftIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { StatusPill } from '@/features/projects/ui'
import {
  TICKET_PRIORITY,
  TICKET_STATUS,
  TICKET_TRACKER,
  formatDateTime,
} from '@/features/tickets/format'
import { cn } from '@/lib/utils'
import type { Ticket } from '@/types/api'

/**
 * Vue liste des tickets.
 *
 * Meme gabarit que les tableaux des contacts, des clients et des taches :
 * construit en flex et non en `<table>`, pour que les cellules s'etirent avec
 * la fenetre, avec un defilement horizontal quand la somme des minimums ne
 * tient plus.
 */
interface Column {
  key: string
  label: string
  icon?: IconSvgElement
  width: string
}

const COLUMNS: Column[] = [
  // Pas d'icone ici : le « # » est deja le symbole de la colonne, et les deux
  // ensemble faisaient un dieze en double.
  { key: 'numero', label: '#', width: 'w-[76px] shrink-0' },
  { key: 'project', label: 'Projet', icon: FolderOpenIcon, width: 'min-w-[170px] flex-[1_1_170px]' },
  { key: 'tracker', label: 'Tracker', icon: Alert02Icon, width: 'w-[130px] shrink-0' },
  { key: 'status', label: 'Statut', icon: RefreshIcon, width: 'w-[150px] shrink-0' },
  { key: 'priority', label: 'Priorité', icon: Flag02Icon, width: 'w-[115px] shrink-0' },
  {
    key: 'subject',
    label: 'Sujet',
    icon: TextAlignLeftIcon,
    width: 'min-w-[240px] flex-[2_1_240px]',
  },
  { key: 'created', label: 'Créé le', icon: Calendar03Icon, width: 'w-[150px] shrink-0' },
  { key: 'updated', label: 'Mis à jour le', icon: Calendar03Icon, width: 'w-[150px] shrink-0' },
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

export function TicketTable({ tickets, empty }: { tickets: Ticket[]; empty: ReactNode }) {
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

        {tickets.length === 0 && (
          <p className="border-b border-[#e8e8e9] px-2.5 py-8 text-center text-[14px] text-[#73757c]">
            {empty}
          </p>
        )}

        {tickets.map((ticket) => {
          const tracker = TICKET_TRACKER[ticket.tracker]!
          const status = TICKET_STATUS[ticket.status]!
          const priority = TICKET_PRIORITY[ticket.priority]!

          return (
            <div key={ticket.id} className="flex items-stretch bg-white">
              {/* Chasse fixe conservee : c'est une colonne qu'on parcourt du
                  regard, et des chiffres de largeur variable la feraient
                  onduler d'une ligne a l'autre. */}
              <Cell width={COLUMNS[0]!.width}>
                <Link
                  to="/pm/tickets/$id"
                  params={{ id: ticket.id }}
                  className="text-[14px] text-[#73757c] tabular-nums hover:text-[#1b1b1b] hover:underline"
                >
                  {ticket.numero}
                </Link>
              </Cell>

              <Cell width={COLUMNS[1]!.width}>
                <Link
                  to="/pm/projets/$id"
                  params={{ id: ticket.project.id }}
                  className="truncate text-[14px] text-[#1b1b1b] hover:underline"
                >
                  {ticket.project.name}
                </Link>
              </Cell>

              <Cell width={COLUMNS[2]!.width}>
                <StatusPill label={tracker.label} color={tracker.pill.text} pill={tracker.pill} />
              </Cell>

              <Cell width={COLUMNS[3]!.width}>
                <StatusPill label={status.label} color={status.pill.text} pill={status.pill} />
              </Cell>

              <Cell width={COLUMNS[4]!.width}>
                <StatusPill label={priority.label} color={priority.pill.text} pill={priority.pill} />
              </Cell>

              <Cell width={COLUMNS[5]!.width}>
                {/* `title` : le sujet est tronque des que la colonne se serre,
                    et c'est la seule cellule dont le texte entier compte. */}
                <Link
                  to="/pm/tickets/$id"
                  params={{ id: ticket.id }}
                  className="truncate text-[14px] text-[#1b1b1b] hover:underline"
                  title={ticket.subject}
                >
                  {ticket.subject}
                </Link>
              </Cell>

              <Cell width={COLUMNS[6]!.width}>
                <span className="text-[13px] text-[#73757c] tabular-nums">
                  {formatDateTime(ticket.created_at)}
                </span>
              </Cell>

              <Cell width={COLUMNS[7]!.width}>
                <span className="text-[13px] text-[#73757c] tabular-nums">
                  {formatDateTime(ticket.updated_at)}
                </span>
              </Cell>
            </div>
          )
        })}
      </div>
    </div>
  )
}
