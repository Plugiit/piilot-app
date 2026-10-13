import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { ProjectUptime, UptimeSettings } from '@/types/api'

export const uptimeKeys = {
  modules: ['modules'] as const,
  settings: ['uptime', 'settings'] as const,
  monitors: (q: string, page: number) => ['uptime', 'monitors', q, page] as const,
  allMonitors: ['uptime', 'monitors'] as const,
  incidents: (page: number) => ['uptime', 'incidents', page] as const,
  project: (id: string) => ['uptime', 'project', id] as const,
}

export const modulesQuery = queryOptions({
  queryKey: uptimeKeys.modules,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/modules')),
})

/**
 * Reglages et etat de la releve. Relus toutes les trois secondes tant
 * qu'une releve demandee — un « Tester » — n'a pas eu lieu.
 */
export const uptimeSettingsQuery = queryOptions({
  queryKey: uptimeKeys.settings,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/modules/uptime')),
  refetchInterval: (query) => (query.state.data?.status.poll_pending === true ? 3_000 : 60_000),
})

export function uptimeMonitorsQuery(q: string, page: number, pageSize = 25) {
  return queryOptions({
    queryKey: uptimeKeys.monitors(q, page),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/modules/uptime/monitors', { params: { query: { q, page, page_size: pageSize } } })),
    placeholderData: keepPreviousData,
  })
}

export function uptimeIncidentsQuery(page: number) {
  return queryOptions({
    queryKey: uptimeKeys.incidents(page),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/modules/uptime/incidents', { params: { query: { page } } })),
    placeholderData: keepPreviousData,
  })
}

export function projectUptimeQuery(id: string) {
  return queryOptions({
    queryKey: uptimeKeys.project(id),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/projects/{id}/uptime', { params: { path: { id } } })),
  })
}

export interface UptimeSettingsValues {
  enabled: boolean
  base_url: string
  /** Absente : garde la cle en place. */
  api_key?: string
  poll_minutes: number
}

function useSetSettings() {
  const queryClient = useQueryClient()
  return (settings: UptimeSettings) => {
    queryClient.setQueryData(uptimeKeys.settings, settings)
    void queryClient.invalidateQueries({ queryKey: uptimeKeys.modules })
  }
}

export function useUpdateUptimeSettings() {
  const set = useSetSettings()
  return useMutation({
    mutationFn: async (body: UptimeSettingsValues) => unwrap(await api.PUT('/api/v1/admin/modules/uptime', { body })),
    onSuccess: set,
  })
}

export function useRotateUptimeSecret() {
  const set = useSetSettings()
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/modules/uptime/rotate')),
    onSuccess: set,
  })
}

export function useTestUptime() {
  const set = useSetSettings()
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/modules/uptime/poll')),
    onSuccess: set,
  })
}

export function useSetProjectSites(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (sites: { monitor_id: string; environment: 'prod' | 'preprod' }[]) =>
      unwrap(await api.PUT('/api/v1/admin/projects/{id}/uptime', { params: { path: { id } }, body: { sites } })),
    onSuccess: (view: ProjectUptime) => {
      queryClient.setQueryData(uptimeKeys.project(id), view)
      // La liste des sites porte les projets lies.
      void queryClient.invalidateQueries({ queryKey: uptimeKeys.allMonitors })
    },
  })
}
