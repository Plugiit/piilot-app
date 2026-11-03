import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { SiteSources } from '@/types/api'

const key = (projectId: string) => ['site-sources', projectId] as const

export function siteSourcesQuery(projectId: string) {
  return queryOptions({
    queryKey: key(projectId),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/projects/{id}/site-sources', { params: { path: { id: projectId } } })),
  })
}

/** Les gestes de la carte : chacun rend la carte a jour. */
export function useSiteSourcesActions(projectId: string) {
  const queryClient = useQueryClient()
  const path = { params: { path: { id: projectId } } }
  const set = (view: SiteSources) => queryClient.setQueryData(key(projectId), view)
  return {
    rotate: useMutation({
      mutationFn: async () => unwrap(await api.POST('/api/v1/admin/projects/{id}/site-sources/backup-token', path)),
      onSuccess: () => void queryClient.invalidateQueries({ queryKey: key(projectId) }),
    }),
    update: useMutation({
      mutationFn: async (body: { expected_hours: number; last_restore_test_on: string | null }) =>
        unwrap(await api.PUT('/api/v1/admin/projects/{id}/site-sources/backup', { ...path, body })),
      onSuccess: set,
    }),
    remove: useMutation({
      mutationFn: async () => unwrap(await api.DELETE('/api/v1/admin/projects/{id}/site-sources/backup', path)),
      onSuccess: set,
    }),
    addDomain: useMutation({
      mutationFn: async (domain: string) => unwrap(await api.POST('/api/v1/admin/projects/{id}/site-sources/domains', { ...path, body: { domain } })),
      onSuccess: set,
    }),
    removeDomain: useMutation({
      mutationFn: async (domainId: string) =>
        unwrap(await api.DELETE('/api/v1/admin/projects/{id}/site-sources/domains/{domainId}', { params: { path: { id: projectId, domainId } } })),
      onSuccess: set,
    }),
  }
}
