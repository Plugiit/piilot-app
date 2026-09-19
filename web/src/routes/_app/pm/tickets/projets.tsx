import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { myTicketsBoardQuery } from '@/features/tickets/api'
import { filtersOf } from '@/routes/_app/pm/tickets'
import { TicketBoard, type BoardColumn } from '@/features/tickets/board'
import { HttpError } from '@/lib/api'

/**
 * Kanban des tickets, par projet.
 *
 * Contrairement au regroupement par statut, les colonnes ne sont pas une liste
 * connue d'avance : seuls les projets qui portent au moins un de vos tickets
 * apparaissent. Monter tous les projets de l'agence donnerait un mur de
 * colonnes vides a traverser pour trouver les trois qui comptent.
 *
 * Elles sont classees par volume decroissant, a nom egal par ordre
 * alphabetique : la charge se lit alors de gauche a droite, sans avoir a
 * comparer des compteurs.
 */
export const Route = createFileRoute('/_app/pm/tickets/projets')({
  loaderDeps: ({ search }) => ({ filters: filtersOf(search) }),
  loader: ({ context, deps }) =>
    context.queryClient.query({ ...myTicketsBoardQuery(deps.filters), staleTime: 'static' }),
  component: TicketsByProjectPage,
})

function TicketsByProjectPage() {
  const search = Route.useSearch()
  const { data, isError, error } = useQuery(myTicketsBoardQuery(filtersOf(search)))

  const tickets = data?.items ?? []

  const byProject = new Map<string, BoardColumn>()

  for (const ticket of tickets) {
    const column = byProject.get(ticket.project.id)

    if (column === undefined) {
      byProject.set(ticket.project.id, {
        key: ticket.project.id,
        label: ticket.project.name,
        tickets: [ticket],
      })
    } else {
      column.tickets.push(ticket)
    }
  }

  const columns = [...byProject.values()].sort(
    (a, b) => b.tickets.length - a.tickets.length || a.label.localeCompare(b.label, 'fr'),
  )

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

      {/* Pas de projet sur les cartes : la colonne le porte deja, et le repeter
          prendrait la place du sujet. */}
      <TicketBoard columns={columns} empty="Aucun ticket ne vous est assigné pour le moment." />
    </div>
  )
}
