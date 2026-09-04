import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { ErrorState } from '@/components/layout/error-state'
import { PageFrame } from '@/components/layout/page-frame'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
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

export const Route = createFileRoute('/_app/admin/projects/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => search,
  loader: ({ context, deps }) =>
    context.queryClient.query({ ...projectListQuery(deps), staleTime: 'static' }),
  errorComponent: (props) => (
    <PageFrame title="Projets">
      <ErrorState {...props} />
    </PageFrame>
  ),
  component: ProjectsPage,
})

function ProjectsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const { data, isPending } = useQuery(projectListQuery(search))

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.page_size)) : 1

  return (
    <PageFrame
      title="Projets"
      description={data ? `${data.total} projet${data.total > 1 ? 's' : ''}` : undefined}
    >
      <div className="px-4 pb-6 sm:px-6">
        <div className="mt-4 overflow-hidden rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Projet</TableHead>
                <TableHead>Client</TableHead>
                <TableHead>Statut</TableHead>
                <TableHead className="text-right">Avancement</TableHead>
              </TableRow>
            </TableHeader>

            <TableBody>
              {isPending &&
                // Des lignes squelettes plutot qu'un message : le tableau garde
                // sa hauteur et ses colonnes, donc rien ne saute a l'arrivee
                // des donnees.
                Array.from({ length: 5 }, (_, i) => (
                  <TableRow key={i}>
                    <TableCell>
                      <Skeleton className="h-4 w-40" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-4 w-28" />
                    </TableCell>
                    <TableCell>
                      <Skeleton className="h-5 w-16 rounded-full" />
                    </TableCell>
                    <TableCell className="flex justify-end">
                      <Skeleton className="h-4 w-10" />
                    </TableCell>
                  </TableRow>
                ))}

              {!isPending && data?.items.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className="text-muted-foreground h-24 text-center">
                    Aucun projet
                  </TableCell>
                </TableRow>
              )}

              {data?.items.map((project) => (
                <TableRow key={project.id}>
                  <TableCell className="font-medium">{project.name}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {project.client_name ?? '—'}
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">{project.status}</Badge>
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{project.completion}%</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>

        <div className="mt-4 flex items-center justify-between">
          <p className="text-muted-foreground text-xs">
            Page {search.page} sur {totalPages}
          </p>

          <div className="flex gap-2">
            <Button
              variant="outline"
              disabled={search.page <= 1}
              onClick={() =>
                void navigate({ search: (prev) => ({ ...prev, page: prev.page - 1 }) })
              }
            >
              Précédent
            </Button>
            <Button
              variant="outline"
              disabled={search.page >= totalPages}
              onClick={() =>
                void navigate({ search: (prev) => ({ ...prev, page: prev.page + 1 }) })
              }
            >
              Suivant
            </Button>
          </div>
        </div>
      </div>
    </PageFrame>
  )
}
