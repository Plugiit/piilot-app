import { Link } from '@tanstack/react-router'

import { StatusPill } from '@/features/projects/ui'
import { TICKET_PRIORITY, TICKET_TRACKER, formatDateTime } from '@/features/tickets/format'
import type { Ticket } from '@/types/api'

/**
 * Une colonne du kanban : ce qui la nomme, et ce qu'elle porte.
 *
 * Les deux vues — par projet, par statut — n'en different que par la facon de
 * repartir les memes cartes. Elles construisent donc leurs colonnes chacune de
 * leur cote et partagent ce composant, plutot que d'exister en deux exemplaires
 * qui divergeraient a la premiere retouche.
 */
export interface BoardColumn {
  key: string
  label: string
  /** Pastille de tete de colonne. Absente quand la colonne ne porte pas de teinte. */
  color?: string
  tickets: Ticket[]
}

/** Carte d'un ticket : le sujet mene a sa fiche. */
function TicketCard({ ticket }: { ticket: Ticket }) {
  const tracker = TICKET_TRACKER[ticket.tracker]!
  const priority = TICKET_PRIORITY[ticket.priority]!

  return (
    <article className="flex flex-col gap-2 rounded-[10px] bg-white p-3 transition-shadow hover:shadow-[0_1px_4px_0_rgb(16_24_40/0.10)]">
      <div className="flex items-center justify-between gap-2">
        <span className="text-[12px] text-[#a2a3a7] tabular-nums">#{ticket.numero}</span>
        <StatusPill label={priority.label} color={priority.pill.text} pill={priority.pill} />
      </div>

      <Link
        to="/pm/tickets/$id"
        params={{ id: ticket.id }}
        className="line-clamp-3 text-[14px] leading-[1.4] text-[#1b1b1b] hover:underline"
      >
        {ticket.subject}
      </Link>

      <div className="flex flex-wrap items-center justify-between gap-2 pt-1">
        <StatusPill label={tracker.label} color={tracker.pill.text} pill={tracker.pill} />

        {/* Le projet ne figure pas sur les cartes de la vue « par projet » : la
            colonne le dit deja, et le repeter sur chaque carte prendrait la
            place du sujet. C'est l'appelant qui decide, via `showProject`. */}
        <span className="text-[11px] text-[#a2a3a7] tabular-nums">
          {formatDateTime(ticket.updated_at)}
        </span>
      </div>
    </article>
  )
}

/** Carte augmentee du projet, pour la vue groupee par statut. */
function TicketCardWithProject({ ticket }: { ticket: Ticket }) {
  return (
    <div className="flex flex-col gap-1.5 rounded-[10px] bg-white">
      <TicketCard ticket={ticket} />

      <Link
        to="/pm/projets/$id"
        params={{ id: ticket.project.id }}
        className="truncate px-3 pb-2.5 text-[12px] text-[#73757c] hover:underline"
      >
        {ticket.project.name}
      </Link>
    </div>
  )
}

export function TicketBoard({
  columns,
  showProject = false,
  empty,
}: {
  columns: BoardColumn[]
  /** Ajoute le projet sur chaque carte. Inutile quand c'est lui qui fait la colonne. */
  showProject?: boolean
  empty: string
}) {
  if (columns.length === 0) {
    return <p className="p-8 text-center text-[14px] text-[#73757c]">{empty}</p>
  }

  return (
    <div className="flex min-h-0 flex-1 gap-3 overflow-x-auto p-4">
      {columns.map((column) => (
        <section
          key={column.key}
          className="flex w-[290px] shrink-0 flex-col gap-2 rounded-[12px] bg-[#f3f4f4] p-2"
        >
          <header className="flex items-center gap-2 px-2 py-1.5">
            {column.color !== undefined && (
              <span
                aria-hidden
                className="size-2 shrink-0 rounded-full"
                style={{ background: column.color }}
              />
            )}
            <h2 className="font-heading min-w-0 flex-1 truncate text-[14px] font-medium text-[#1b1b1b]">
              {column.label}
            </h2>
            <span className="text-[12px] text-[#73757c] tabular-nums">{column.tickets.length}</span>
          </header>

          <div className="flex flex-col gap-2">
            {column.tickets.length === 0 ? (
              <p className="px-2 py-6 text-center text-[13px] text-[#a2a3a7]">Aucun ticket</p>
            ) : (
              column.tickets.map((ticket) =>
                showProject ? (
                  <TicketCardWithProject key={ticket.id} ticket={ticket} />
                ) : (
                  <TicketCard key={ticket.id} ticket={ticket} />
                ),
              )
            )}
          </div>
        </section>
      ))}
    </div>
  )
}
