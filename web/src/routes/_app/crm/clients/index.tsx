import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { Button } from '@/components/ui/button'
import { CLIENTS_PAGE_SIZE, clientListQuery } from '@/features/clients/api'
import { ClientTable } from '@/features/clients/list'

import { paramsOf } from '../clients'

/**
 * Vue liste de l'ecran « Clients ».
 *
 * La barre d'outils et les onglets vivent dans le chassis : cette vue ne porte
 * que le tableau et sa pagination — le kanban, lui, n'en a pas.
 */
export const Route = createFileRoute('/_app/crm/clients/')({
  loaderDeps: ({ search }) => paramsOf(search),
  loader: ({ context, deps }) =>
    context.queryClient.query({ ...clientListQuery(deps), staleTime: 'static' }),
  component: ClientListPage,
})

function ClientListPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data: page } = useQuery(clientListQuery(paramsOf(search)))

  const total = page?.total ?? 0
  const current = page?.page ?? 1
  const totalPages = Math.max(1, Math.ceil(total / CLIENTS_PAGE_SIZE))

  const filtered =
    (search.search ?? '') !== '' ||
    search.statut !== undefined ||
    search.charge !== undefined ||
    search.portail !== undefined

  return (
    <>
      <ClientTable
        clients={page?.items ?? []}
        empty={filtered ? 'Aucun client ne correspond.' : 'Aucun client pour le moment.'}
      />

      <div className="flex flex-wrap items-center justify-between gap-2 p-4">
        <p className="text-[12px] text-[#777]">
          {total} client{total > 1 ? 's' : ''} · page {current} sur {totalPages}
        </p>

        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={current <= 1}
            onClick={() => void navigate({ search: (prev) => ({ ...prev, page: current - 1 }) })}
          >
            Précédent
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={current >= totalPages}
            onClick={() => void navigate({ search: (prev) => ({ ...prev, page: current + 1 }) })}
          >
            Suivant
          </Button>
        </div>
      </div>
    </>
  )
}
