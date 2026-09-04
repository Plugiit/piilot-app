import { queryOptions, keepPreviousData } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'

export interface ProjectListParams {
  page: number
  pageSize: number
  search?: string
  status?: string
}

/**
 * Liste des projets, paginee cote serveur.
 *
 * `keepPreviousData` garde la page precedente affichee pendant le chargement
 * de la suivante : la table ne clignote pas et ne saute pas en hauteur au
 * changement de page.
 */
export function projectListQuery(params: ProjectListParams) {
  return queryOptions({
    queryKey: ['projects', 'list', params],
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/projects', {
          params: {
            query: {
              page: params.page,
              page_size: params.pageSize,
              search: params.search,
              status: params.status,
            },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/** Agregats du tableau de bord, precalcules et caches cote serveur. */
export const dashboardQuery = queryOptions({
  queryKey: ['dashboard'],
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/dashboard')),
})
