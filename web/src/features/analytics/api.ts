import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { projectKeys } from '@/features/projects/api'
import { api, unwrap } from '@/lib/api'
import type { AnalyticsSettings, ProjectAnalytics } from '@/types/api'

export const analyticsKeys = {
  settings: ['analytics', 'settings'] as const,
  websites: (q: string, page: number) => ['analytics', 'websites', q, page] as const,
  allWebsites: ['analytics', 'websites'] as const,
  project: (id: string) => ['analytics', 'project', id] as const,
  statistics: (id: string, period: string, from?: string, to?: string) => ['analytics', 'statistics', id, period, from ?? null, to ?? null] as const,
  active: (id: string) => ['analytics', 'active', id] as const,
}

/**
 * Reglages et etat du module. Relus toutes les trois secondes tant qu'une
 * verification demandee n'a pas eu lieu.
 */
export const analyticsSettingsQuery = queryOptions({
  queryKey: analyticsKeys.settings,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/modules/analytics')),
  refetchInterval: (query) => (query.state.data?.status.check_pending === true ? 3_000 : 60_000),
})

export function analyticsWebsitesQuery(q: string, page: number, pageSize = 25) {
  return queryOptions({
    queryKey: analyticsKeys.websites(q, page),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/modules/analytics/websites', { params: { query: { q, page, page_size: pageSize } } })),
    placeholderData: keepPreviousData,
  })
}

export function projectAnalyticsQuery(id: string) {
  return queryOptions({
    queryKey: analyticsKeys.project(id),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/projects/{id}/analytics', { params: { path: { id } } })),
  })
}

export interface AnalyticsSettingsValues {
  enabled: boolean
  base_url: string
  auth_mode: 'login' | 'api_key'
  username: string
  /** Absent : garde le mot de passe en place. */
  password?: string
  /** Absente : garde la cle en place. */
  api_key?: string
  api_version: 0 | 2 | 3
  cache_minutes: number
}

function useSetSettings() {
  const queryClient = useQueryClient()
  return (settings: AnalyticsSettings) => {
    queryClient.setQueryData(analyticsKeys.settings, settings)
    void queryClient.invalidateQueries({ queryKey: ['modules'] })
  }
}

export function useUpdateAnalyticsSettings() {
  const set = useSetSettings()
  return useMutation({
    mutationFn: async (body: AnalyticsSettingsValues) => unwrap(await api.PUT('/api/v1/admin/modules/analytics', { body })),
    onSuccess: set,
  })
}

export function useCheckAnalytics() {
  const set = useSetSettings()
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/modules/analytics/check')),
    onSuccess: set,
  })
}

export function useSetProjectWebsites(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (sites: { website_id: string; environment: 'prod' | 'preprod' }[]) =>
      unwrap(await api.PUT('/api/v1/admin/projects/{id}/analytics', { params: { path: { id } }, body: { sites } })),
    onSuccess: (view: ProjectAnalytics) => {
      queryClient.setQueryData(analyticsKeys.project(id), view)
      void queryClient.invalidateQueries({ queryKey: analyticsKeys.allWebsites })
      // La fiche du projet dit s'il manque ses statistiques.
      void queryClient.invalidateQueries({ queryKey: projectKeys.detail(id) })
    },
  })
}

export type PeriodKey = '7d' | '30d' | 'month' | 'prev_month' | '12m' | 'custom'

/**
 * La page « Statistiques » du portail. Une vue en preparation se relit
 * toutes les deux secondes ; prete, toutes les cinq minutes.
 */
export function statisticsQuery(id: string, period: PeriodKey, from?: string, to?: string) {
  return queryOptions({
    queryKey: analyticsKeys.statistics(id, period, from, to),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/client/projects/{id}/statistics', {
          params: { path: { id }, query: { period, ...(period === 'custom' ? { from, to } : {}) } },
        }),
      ),
    placeholderData: keepPreviousData,
    refetchInterval: (query) => (query.state.data?.status === 'pending' ? 2_000 : 5 * 60_000),
    retry: false,
  })
}

/** Les visiteurs du moment, relus toutes les trente secondes. */
export function activeVisitorsQuery(id: string) {
  return queryOptions({
    queryKey: analyticsKeys.active(id),
    queryFn: async () => unwrap(await api.GET('/api/v1/client/projects/{id}/statistics/active', { params: { path: { id } } })),
    refetchInterval: 30_000,
    retry: false,
  })
}
