import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { myTicketsBoardQuery } from '@/features/tickets/api'
import { filtersOf } from '@/routes/_app/pm/tickets'
import { TicketBoard, type BoardColumn } from '@/features/tickets/board'
import { TICKET_STATUS, TICKET_STATUS_ORDER } from '@/features/tickets/format'
import { HttpError } from '@/lib/api'

/**
 * Kanban des tickets, par statut.
 *
 * Les sept colonnes sont toujours la, meme vides : ce sont les etapes du cycle,
 * pas un resume des donnees. En masquer une parce qu'elle est vide ferait
 * disparaitre « En relecture » les jours ou rien n'y est, et le tableau
 * changerait de forme d'une heure a l'autre.
 */
export const Route = createFileRoute('/_app/pm/tickets/statuts')({
  loaderDeps: ({ search }) => ({ filters: filtersOf(search) }),
  loader: ({ context, deps }) =>
    context.queryClient.query({ ...myTicketsBoardQuery(deps.filters), staleTime: 'static' }),
  component: TicketsByStatusPage,
})

function TicketsByStatusPage() {
  const search = Route.useSearch()
  const { data, isError, error } = useQuery(myTicketsBoardQuery(filtersOf(search)))

  const tickets = data?.items ?? []

  const columns: BoardColumn[] = TICKET_STATUS_ORDER.map((status) => ({
    key: status,
    label: TICKET_STATUS[status]!.label,
    color: TICKET_STATUS[status]!.pill.text,
    tickets: tickets.filter((ticket) => ticket.status === status),
  }))

  if (isError) {
    return (
      <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
        {error instanceof HttpError ? error.message : 'Chargement impossible'}
      </p>
    )
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {data?.truncated === true && (
        <p className="px-4 pt-4 text-[12px] text-[#73757c]">
          Seules les 300 premières cartes sont affichées. Le tableau les montre toutes.
        </p>
      )}

      {/* Le projet figure sur les cartes : ici c'est le statut qui fait la
          colonne, et sans lui on ne saurait pas de quoi parle le ticket. */}
      <TicketBoard
        columns={columns}
        showProject
        empty="Aucun ticket ne vous est assigné pour le moment."
      />
    </div>
  )
}
