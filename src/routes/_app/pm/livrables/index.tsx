import { Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  DELIVERABLES_PAGE_SIZE,
  deliverableListQuery,
  type DeliverableFilters,
} from '@/features/deliverables/api'
import { DELIVERABLE_STATUS, DELIVERABLE_STATUS_ORDER } from '@/features/deliverables/format'
import { DeliverableTable } from '@/features/deliverables/list'
import { NewDeliverableDialog } from '@/features/deliverables/new-deliverable-dialog'
import { projectListQuery } from '@/features/projects/api'
import { HttpError } from '@/lib/api'
import { useSearchField } from '@/lib/search-field'
import type { DeliverableStatus } from '@/types/api'

/**
 * Ecran « Livrables » du module.
 *
 * Il traverse les projets, et c'est sa raison d'etre : la fiche d'un projet dit
 * ou en sont ses livrables a lui, elle ne dit pas ce qui attend une reponse
 * client en ce moment. C'est cette file-la qu'on relance le lundi matin.
 *
 * Les filtres vivent dans l'adresse : « ce qui attend chez ce client » est une
 * vue qu'on partage et ou le bouton Retour ramene.
 */
const searchSchema = z.object({
  page: z.number().int().min(1).catch(1),
  search: z.string().optional(),
  status: z.enum(['brouillon', 'en_attente', 'retours', 'valide']).optional().catch(undefined),
  projet: z.string().optional(),
})

type LivrablesSearch = z.infer<typeof searchSchema>

function filtersOf(search: LivrablesSearch): DeliverableFilters {
  return {
    search: search.search?.trim() === '' ? undefined : search.search,
    status: search.status,
    projectId: search.projet,
  }
}

export const Route = createFileRoute('/_app/pm/livrables/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ filters: filtersOf(search), page: search.page }),
  loader: ({ context, deps }) =>
    context.queryClient.query({
      ...deliverableListQuery(deps.filters, deps.page),
      staleTime: 'static',
    }),
  component: LivrablesPage,
})

function LivrablesPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data: projects } = useQuery(
    projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }),
  )

  const { data, isError, error } = useQuery(deliverableListQuery(filtersOf(search), search.page))

  // Tout changement de filtre ramene page 1 : rester en page 3 d'une liste qui
  // vient de se reduire n'afficherait rien.
  function setFilter(patch: Partial<Omit<LivrablesSearch, 'page'>>) {
    void navigate({ search: (prev) => ({ ...prev, ...patch, page: 1 }), replace: true })
  }

  const [draft, setDraft] = useSearchField(search.search ?? '', (value) =>
    setFilter({ search: value === '' ? undefined : value }),
  )

  const statusOptions: Option[] = DELIVERABLE_STATUS_ORDER.map((status) => ({
    value: status,
    label: DELIVERABLE_STATUS[status]!.label,
    color: DELIVERABLE_STATUS[status]!.pill.text,
  }))

  const projectOptions: Option[] = (projects?.items ?? []).map((project) => ({
    value: project.id,
    label: project.name,
  }))

  const total = data?.total ?? 0
  const current = data?.page ?? search.page
  const totalPages = Math.max(1, Math.ceil(total / DELIVERABLES_PAGE_SIZE))

  return (
    <PageFrame title="Livrables">
      <div className="flex min-h-full flex-col">
        <div className="flex flex-wrap items-center gap-2 p-4">
          <div className="relative min-w-[200px] flex-1">
            <HugeiconsIcon
              icon={Search01Icon}
              size={16}
              strokeWidth={1.6}
              className="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-[#8d8d8d]"
            />
            <Input
              type="search"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder="Rechercher un livrable"
              aria-label="Rechercher un livrable"
              className="h-9 pl-8 text-[13px]"
            />
          </div>

          <FilterMenu
            name="État"
            all="Tous les états"
            value={search.status}
            options={statusOptions}
            onChange={(value) => setFilter({ status: value as DeliverableStatus | undefined })}
          />

          <FilterMenu
            name="Projet"
            all="Tous les projets"
            value={search.projet}
            options={projectOptions}
            onChange={(value) => setFilter({ projet: value })}
          />

          <NewDeliverableDialog />
        </div>

        {isError ? (
          <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
            {error instanceof HttpError ? error.message : 'Chargement impossible'}
          </p>
        ) : (
          <>
            <DeliverableTable
              items={data?.items ?? []}
              empty="Aucun livrable. Déposez le premier depuis « Déposer un livrable »."
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
    </PageFrame>
  )
}
