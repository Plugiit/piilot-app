import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { toast } from 'sonner'

import { clientBoardQuery, useMoveClientStatus } from '@/features/clients/api'
import { ClientBoard } from '@/features/clients/board'
import { CLIENT_STATUS } from '@/features/clients/format'
import { HttpError } from '@/lib/api'

import { paramsOf } from '../clients'

/**
 * Vue pipeline de l'ecran « Clients ».
 *
 * Le kanban ignore le filtre de statut et celui du portail : ranger par etape
 * est justement ce qu'il fait, et masquer des colonnes reviendrait a cacher
 * l'information qu'on vient lire. Il garde la recherche et le chargé de compte.
 */
export const Route = createFileRoute('/_app/crm/clients/kanban')({
  loaderDeps: ({ search }) => {
    const { search: query, managerId } = paramsOf(search)

    return { search: query, managerId }
  },
  loader: ({ context, deps }) =>
    context.queryClient.query({ ...clientBoardQuery(deps), staleTime: 'static' }),
  component: ClientKanbanPage,
})

function ClientKanbanPage() {
  const search = Route.useSearch()
  const { search: query, managerId } = paramsOf(search)

  const { data: board } = useQuery(clientBoardQuery({ search: query, managerId }))
  const move = useMoveClientStatus()

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {board?.truncated === true && (
        <p className="border-b border-[#e8e8e9] px-4 py-2 text-[12px] text-[#73757c]">
          Toutes les cartes ne sont pas affichées. Affinez la recherche pour voir le reste.
        </p>
      )}

      <ClientBoard
        clients={board?.items ?? []}
        onMove={(client, status) =>
          move.mutate(
            { id: client.id, status },
            {
              onSuccess: () => {
                toast.success(`« ${client.name} » → ${CLIENT_STATUS[status].label}`)
              },
              onError: (error) => {
                toast.error(
                  error instanceof HttpError ? error.message : 'Déplacement impossible',
                )
              },
            },
          )
        }
      />
    </div>
  )
}
