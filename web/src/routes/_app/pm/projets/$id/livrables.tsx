import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { DELIVERABLES_PAGE_SIZE, deliverableListQuery } from '@/features/deliverables/api'
import { DeliverableTable } from '@/features/deliverables/list'
import { HttpError } from '@/lib/api'

/**
 * Les livrables du projet : ce qui a ete depose, ou en est la validation.
 *
 * Le meme tableau que l'ecran global, sans les colonnes projet et client —
 * la fiche les dit deja. Un seul endpoint pour les deux ecrans : celui-ci le
 * filtre sur le projet.
 */
const searchSchema = z.object({
  page: z.number().int().min(1).catch(1),
})

export const Route = createFileRoute('/_app/pm/projets/$id/livrables')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ page: search.page }),
  loader: ({ context, params, deps }) =>
    context.queryClient.query({
      ...deliverableListQuery({ projectId: params.id }, deps.page),
      staleTime: 'static',
    }),
  component: ProjectDeliverablesPage,
})

function ProjectDeliverablesPage() {
  const { id } = Route.useParams()
  const { page } = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data, isError, error } = useQuery(deliverableListQuery({ projectId: id }, page))

  if (isError) {
    return (
      <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
        {error instanceof HttpError ? error.message : 'Chargement impossible'}
      </p>
    )
  }

  const total = data?.total ?? 0
  const current = data?.page ?? page
  const totalPages = Math.max(1, Math.ceil(total / DELIVERABLES_PAGE_SIZE))

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <DeliverableTable
        items={data?.items ?? []}
        showProject={false}
        empty="Aucun livrable sur ce projet. Déposez le premier depuis « Déposer un livrable »."
      />

      <div className="flex flex-wrap items-center justify-between gap-2 p-4">
        <p className="text-[12px] text-[#777]">
          {total} livrable{total > 1 ? 's' : ''} · page {current} sur {totalPages}
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
    </div>
  )
}
