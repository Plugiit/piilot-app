import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { ProjectMaintenance } from '@/types/api'

export const maintenanceKeys = {
  all: ['maintenance'] as const,
  plans: ['maintenance', 'plans'] as const,
  project: (id: string) => ['maintenance', 'project', id] as const,
  interventions: (month: string, project: string | null, page: number) => ['maintenance', 'interventions', month, project, page] as const,
  portal: (id: string, page: number) => ['maintenance', 'portal', id, page] as const,
}

export const maintenancePlansQuery = queryOptions({
  queryKey: maintenanceKeys.plans,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/maintenance-plans')),
})

export interface PlanValues {
  name: string
  included_minutes: number
  sections: ('availability' | 'audience' | 'interventions' | 'deployments' | 'updates' | 'backups' | 'domain')[]
  active: boolean
}

export function useSavePlan(id: string | null) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: PlanValues) =>
      id === null
        ? unwrap(await api.POST('/api/v1/admin/maintenance-plans', { body }))
        : unwrap(await api.PUT('/api/v1/admin/maintenance-plans/{id}', { params: { path: { id } }, body })),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: maintenanceKeys.all }),
  })
}

export function useDeletePlan() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.DELETE('/api/v1/admin/maintenance-plans/{id}', { params: { path: { id } } })),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: maintenanceKeys.all }),
  })
}

export function projectMaintenanceQuery(id: string) {
  return queryOptions({
    queryKey: maintenanceKeys.project(id),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/projects/{id}/maintenance', { params: { path: { id } } })),
  })
}

export function useSetProjectMaintenance(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: { plan_id: string | null; month: string }) =>
      unwrap(await api.PUT('/api/v1/admin/projects/{id}/maintenance', { params: { path: { id } }, body })),
    onSuccess: (view: ProjectMaintenance) => {
      queryClient.setQueryData(maintenanceKeys.project(id), view)
      void queryClient.invalidateQueries({ queryKey: ['maintenance', 'interventions'] })
    },
  })
}

export function interventionsQuery(month: string, project: string | null, page: number) {
  return queryOptions({
    queryKey: maintenanceKeys.interventions(month, project, page),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/interventions', {
          params: { query: { month, page, ...(project !== null ? { project } : {}) } },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

export function portalInterventionsQuery(id: string, page: number) {
  return queryOptions({
    queryKey: maintenanceKeys.portal(id, page),
    queryFn: async () => unwrap(await api.GET('/api/v1/client/projects/{id}/interventions', { params: { path: { id }, query: { page } } })),
    placeholderData: keepPreviousData,
  })
}
