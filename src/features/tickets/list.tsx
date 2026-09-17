import {
  Alert02Icon,
  Calendar03Icon,
  Flag02Icon,
  FolderOpenIcon,
  RefreshIcon,
  TextAlignLeftIcon,
  UserIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { Avatars, StatusPill } from '@/features/projects/ui'
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
 *
 * Chaque colonne porte son rendu plutot que d'etre repeinte a la main dans
 * l'ordre : les deux jeux ci-dessous n'ont alors qu'a se composer, la ou des
 * cellules rangees par position obligeaient a decaler tout le corps du tableau
 * des qu'une colonne changeait de place.
 */
interface Column {
  key: string
  label: string
  icon?: IconSvgElement
  width: string
  cell: (ticket: Ticket) => ReactNode
}

// Pas d'icone sur le numero : le « # » est deja le symbole de la colonne, et
// les deux ensemble faisaient un dieze en double.
//
// Chasse fixe conservee : c'est une colonne qu'on parcourt du regard, et des
// chiffres de largeur variable la feraient onduler d'une ligne a l'autre.
const NUMERO: Column = {
  key: 'numero',
  label: '#',
  width: 'w-[76px] shrink-0',
  cell: (ticket) => (
    <Link
      to="/pm/tickets/$id"
      params={{ id: ticket.id }}
      className="text-[14px] text-[#73757c] tabular-nums hover:text-[#1b1b1b] hover:underline"
    >
      {ticket.numero}
    </Link>
  ),
}

const PROJECT: Column = {
  key: 'project',
  label: 'Projet',
  icon: FolderOpenIcon,
  width: 'min-w-[170px] flex-[1_1_170px]',
  cell: (ticket) => (
    <Link
      to="/pm/projets/$id"
      params={{ id: ticket.project.id }}
      className="truncate text-[14px] text-[#1b1b1b] hover:underline"
    >
      {ticket.project.name}
    </Link>
  ),
}

// Un ticket sans destinataire est un etat normal, pas une donnee manquante :
// la cellule dit « A prendre » plutot que de rester vide, qui se lirait comme
// une valeur qui n'a pas charge.
const ASSIGNEE: Column = {
  key: 'assignee',
  label: 'Assigné à',
  icon: UserIcon,
  width: 'min-w-[170px] flex-[1_1_170px]',
  cell: (ticket) => {
    if (ticket.assignee === null) {
      return <span className="text-[14px] text-[#a2a3a7]">À prendre</span>
    }

    const nom = `${ticket.assignee.firstname} ${ticket.assignee.lastname}`.trim() || 'Sans nom'

    return (
      <>
        <Avatars people={[ticket.assignee]} max={1} size={24} />
        <span className="truncate text-[14px] text-[#1b1b1b]" title={nom}>
          {nom}
        </span>
      </>
    )
  },
}

const TRACKER: Column = {
  key: 'tracker',
  label: 'Tracker',
  icon: Alert02Icon,
  width: 'w-[130px] shrink-0',
  cell: (ticket) => {
    const tracker = TICKET_TRACKER[ticket.tracker]!

    return <StatusPill label={tracker.label} color={tracker.pill.text} pill={tracker.pill} />
  },
}

const STATUS: Column = {
  key: 'status',
  label: 'Statut',
  icon: RefreshIcon,
  width: 'w-[150px] shrink-0',
  cell: (ticket) => {
    const status = TICKET_STATUS[ticket.status]!

    return <StatusPill label={status.label} color={status.pill.text} pill={status.pill} />
  },
}

const PRIORITY: Column = {
  key: 'priority',
  label: 'Priorité',
  icon: Flag02Icon,
  width: 'w-[115px] shrink-0',
  cell: (ticket) => {
    const priority = TICKET_PRIORITY[ticket.priority]!

    return <StatusPill label={priority.label} color={priority.pill.text} pill={priority.pill} />
  },
}

const SUBJECT: Column = {
  key: 'subject',
  label: 'Sujet',
  icon: TextAlignLeftIcon,
  width: 'min-w-[240px] flex-[2_1_240px]',
  // `title` : le sujet est tronque des que la colonne se serre, et c'est la
  // seule cellule dont le texte entier compte.
  cell: (ticket) => (
    <Link
      to="/pm/tickets/$id"
      params={{ id: ticket.id }}
      className="truncate text-[14px] text-[#1b1b1b] hover:underline"
      title={ticket.subject}
    >
      {ticket.subject}
    </Link>
  ),
}

const CREATED: Column = {
  key: 'created',
  label: 'Créé le',
  icon: Calendar03Icon,
  width: 'w-[150px] shrink-0',
  cell: (ticket) => (
    <span className="text-[13px] text-[#73757c] tabular-nums">
      {formatDateTime(ticket.created_at)}
    </span>
  ),
}

const UPDATED: Column = {
  key: 'updated',
  label: 'Mis à jour le',
  icon: Calendar03Icon,
  width: 'w-[150px] shrink-0',
  cell: (ticket) => (
    <span className="text-[13px] text-[#73757c] tabular-nums">
      {formatDateTime(ticket.updated_at)}
    </span>
  ),
}

/**
 * Colonnes de l'ecran « Tickets » du module, qui traverse les projets : c'est
 * le projet qui y distingue une ligne de la suivante.
 */
const TICKET_COLUMNS: Column[] = [
  NUMERO,
  PROJECT,
  TRACKER,
  STATUS,
  PRIORITY,
  SUBJECT,
  CREATED,
  UPDATED,
]

/**
 * Colonnes de l'onglet « Tickets » d'un projet.
 *
 * Le projet cede sa place a l'assigne : repete a l'identique sur chaque ligne
 * il n'apprenait rien, quand « qui s'en occupe » est justement la question
 * qu'on se pose devant un projet.
 */
const PROJECT_TICKET_COLUMNS: Column[] = [
  NUMERO,
  ASSIGNEE,
  TRACKER,
  STATUS,
  PRIORITY,
  SUBJECT,
  CREATED,
  UPDATED,
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

interface TableProps {
  tickets: Ticket[]
  empty: ReactNode
}

/** Le tableau tel que l'ecran « Tickets » du module l'affiche. */
export function TicketTable(props: TableProps) {
  return <Table {...props} columns={TICKET_COLUMNS} />
}

/** Le meme tableau dans la fiche d'un projet, ou l'assigne remplace le projet. */
export function ProjectTicketTable(props: TableProps) {
  return <Table {...props} columns={PROJECT_TICKET_COLUMNS} />
}

function Table({ tickets, empty, columns }: TableProps & { columns: Column[] }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-x-auto">
      <div className="flex min-w-max flex-col">
        <div className="flex items-stretch">
          {columns.map((column) => (
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

        {tickets.map((ticket) => (
          <div key={ticket.id} className="flex items-stretch bg-white">
            {columns.map((column) => (
              <Cell key={column.key} width={column.width}>
                {column.cell(ticket)}
              </Cell>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
