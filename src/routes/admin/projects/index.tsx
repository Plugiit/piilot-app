import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { PageHeader } from '@/components/layout/page-header'
import { Button } from '@/components/ui/button'
import { projectListQuery } from '@/features/projects/api'

/**
 * Les filtres vivent dans l'URL, pas dans un useState : la vue est
 * partageable, le bouton Retour fonctionne, et la cle de cache TanStack Query
 * derive directement de ces parametres.
 */
const searchSchema = z.object({
  page: z.number().int().min(1).catch(1),
  pageSize: z.number().int().min(10).max(100).catch(25),
  search: z.string().optional(),
  status: z.string().optional(),
})

export const Route = createFileRoute('/admin/projects/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => search,
  loader: ({ context, deps }) =>
    context.queryClient.query({ ...projectListQuery(deps), staleTime: 'static' }),
  component: ProjectsPage,
})

function ProjectsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const { data, isPending } = useQuery(projectListQuery(search))

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.page_size)) : 1

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader
        title="Projets"
        subtitle={data ? `${data.total} projet${data.total > 1 ? 's' : ''}` : undefined}
      />

      <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-6 sm:px-6">
        <div className="overflow-hidden rounded-xl border border-border bg-surface-raised">
          <table className="w-full text-sm">
            <thead className="border-b border-border bg-surface-sunken text-left text-xs uppercase text-ink-muted">
              <tr>
                <th className="px-4 py-2.5 font-medium">Projet</th>
                <th className="px-4 py-2.5 font-medium">Client</th>
                <th className="px-4 py-2.5 font-medium">Statut</th>
                <th className="px-4 py-2.5 text-right font-medium">Avancement</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {isPending && (
                <tr>
                  <td colSpan={4} className="px-4 py-8 text-center text-ink-muted">
                    Chargement…
                  </td>
                </tr>
              )}

              {data?.items.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-4 py-8 text-center text-ink-muted">
                    Aucun projet
                  </td>
                </tr>
              )}

              {data?.items.map((project) => (
                <tr key={project.id} className="hover:bg-surface-sunken">
                  <td className="px-4 py-2.5 font-medium text-ink">{project.name}</td>
                  <td className="px-4 py-2.5 text-ink-soft">{project.client_name ?? '—'}</td>
                  <td className="px-4 py-2.5 text-ink-soft">{project.status}</td>
                  <td className="px-4 py-2.5 text-right tabular-nums text-ink-soft">
                    {project.completion}%
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="mt-4 flex items-center justify-between">
          <p className="text-xs text-ink-muted">
            Page {search.page} sur {totalPages}
          </p>

          <div className="flex gap-2">
            <Button
              variant="outline"
              disabled={search.page <= 1}
              onClick={() => void navigate({ search: (prev) => ({ ...prev, page: prev.page - 1 }) })}
            >
              Précédent
            </Button>
            <Button
              variant="outline"
              disabled={search.page >= totalPages}
              onClick={() => void navigate({ search: (prev) => ({ ...prev, page: prev.page + 1 }) })}
            >
              Suivant
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
