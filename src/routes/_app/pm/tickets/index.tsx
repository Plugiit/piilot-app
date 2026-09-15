import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { TICKETS_PAGE_SIZE, myTicketsQuery } from '@/features/tickets/api'
import { filtersOf } from '@/routes/_app/pm/tickets'
import { TicketTable } from '@/features/tickets/list'
import { HttpError } from '@/lib/api'

/**
 * Vue tableau des tickets.
 *
 * L'ecran ne montre que les tickets confies au compte connecte. L'assigne n'est
 * pas un parametre d'adresse : le serveur le lit dans la session, sinon changer
 * l'URL suffirait a lire la file de quelqu'un d'autre.
 *
 * `page` est valide par la route parente, que les trois vues partagent.
 */
export const Route = createFileRoute('/_app/pm/tickets/')({
  // Toute la barre d'outils entre dans les dependances : changer un filtre
  // doit recharger, changer de page aussi.
  loaderDeps: ({ search }) => ({ filters: filtersOf(search), page: search.page }),
  loader: ({ context, deps }) =>
    context.queryClient.query({
      ...myTicketsQuery(deps.filters, deps.page),
      staleTime: 'static',
    }),
  component: TicketsPage,
})

function TicketsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data, isError, error } = useQuery(myTicketsQuery(filtersOf(search), search.page))

  const tickets = data?.items ?? []
  const total = data?.total ?? 0
  const current = data?.page ?? search.page
  const totalPages = Math.max(1, Math.ceil(total / TICKETS_PAGE_SIZE))

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {isError && (
          <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
            {error instanceof HttpError ? error.message : 'Chargement impossible'}
          </p>
        )}

        {!isError && (
          <>
            <TicketTable
              tickets={tickets}
              empty="Aucun ticket ne vous est assigné pour le moment."
            />

            <div className="flex flex-wrap items-center justify-between gap-2 p-4">
              <p className="text-[12px] text-[#777]">
                {total} ticket{total > 1 ? 's' : ''} · page {current} sur {totalPages}
              </p>

              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={current <= 1}
                  onClick={() =>
                    void navigate({ search: (prev) => ({ ...prev, page: current - 1 }) })
                  }
                >
                  Précédent
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={current >= totalPages}
                  onClick={() =>
                    void navigate({ search: (prev) => ({ ...prev, page: current + 1 }) })
                  }
                >
                  Suivant
                </Button>
              </div>
            </div>
          </>
        )}
    </div>
  )
}
