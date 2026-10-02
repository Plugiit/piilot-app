import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { InteractionForm, InteractionList } from '@/features/interactions/journal'
import { clientListQuery } from '@/features/projects/api'
import { can } from '@/lib/auth'

/**
 * Interactions : le journal de la relation, tous clients confondus.
 *
 * Les saisies (notes, appels, rendez-vous, e-mails) et les evenements qui
 * s'inscrivent seuls (projet cree, livrable valide, ticket ouvert) se melent
 * dans l'ordre ou ils sont arrives : c'est ainsi qu'on relit une relation. Le
 * filtre par client mene au meme fil que la fiche du client.
 */
const searchSchema = z.object({
  client: z.string().optional(),
  source: z.enum(['manual', 'events']).optional().catch(undefined),
})

export const Route = createFileRoute('/_app/crm/interactions/')({
  validateSearch: searchSchema,
  component: InteractionsPage,
})

const SOURCES: Option[] = [
  { value: 'manual', label: 'Saisies' },
  { value: 'events', label: 'Événements automatiques' },
]

function InteractionsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const { user } = Route.useRouteContext()
  const { data: clients } = useQuery(clientListQuery())

  const clientOptions: Option[] = (clients?.items ?? []).map((client) => ({
    value: client.id,
    label: client.name,
  }))

  function setFilter(patch: { client?: string; source?: 'manual' | 'events' }) {
    void navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true })
  }

  return (
    <PageFrame title="Interactions">
      <div className="flex min-h-full flex-col gap-4 p-4">
        <div className="flex flex-wrap items-center gap-2">
          <FilterMenu
            name="Client"
            all="Tous les clients"
            value={search.client}
            options={clientOptions}
            onChange={(value) => setFilter({ client: value })}
          />
          <FilterMenu
            name="Source"
            all="Saisies et événements"
            value={search.source}
            options={SOURCES}
            onChange={(value) => setFilter({ source: value as 'manual' | 'events' | undefined })}
          />
        </div>

        <div className="grid max-w-[960px] gap-4">
          {/* On note depuis l'ecran global une fois le client choisi : une
              interaction appartient toujours a un client. */}
          {search.client !== undefined && can(user, 'clients.write') && (
            <section className="rounded-[12px] border border-[#e8e8e9] bg-white p-3">
              <InteractionForm clientId={search.client} />
            </section>
          )}

          <section className="rounded-[12px] border border-[#e8e8e9] bg-white px-3 py-1">
            <InteractionList clientId={search.client} source={search.source} showClient={search.client === undefined} />
          </section>
        </div>
      </div>
    </PageFrame>
  )
}
