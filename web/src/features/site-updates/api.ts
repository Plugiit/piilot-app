import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { SiteUpdates } from '@/types/api'

const key = (projectId: string) => ['site-updates', projectId] as const
const tokenKey = ['site-updates', 'token'] as const

export type UpdateEnvironment = 'prod' | 'preprod'

/** Un lancement pas encore termine : la carte se rafraichit toute seule. */
function hasActiveRun(view: SiteUpdates | undefined): boolean {
  return view?.runs.some((r) => r.status === 'queued' || r.status === 'dispatching' || r.status === 'dispatched') ?? false
}

export function siteUpdatesQuery(projectId: string) {
  return queryOptions({
    queryKey: key(projectId),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/projects/{id}/site-updates', { params: { path: { id: projectId } } })),
    refetchInterval: (query) => (hasActiveRun(query.state.data) ? 15_000 : false),
  })
}

export function useSiteUpdatesActions(projectId: string) {
  const queryClient = useQueryClient()
  const path = { params: { path: { id: projectId } } }
  const set = (view: SiteUpdates) => queryClient.setQueryData(key(projectId), view)
  return {
    setTarget: useMutation({
      mutationFn: async (body: { environment: UpdateEnvironment; workflow: string; ref: string; every_days: number | null; enabled: boolean }) =>
        unwrap(await api.PUT('/api/v1/admin/projects/{id}/site-updates', { ...path, body })),
      onSuccess: set,
    }),
    run: useMutation({
      mutationFn: async (environment: UpdateEnvironment) =>
        unwrap(await api.POST('/api/v1/admin/projects/{id}/site-updates/runs', { ...path, body: { environment } })),
      onSuccess: set,
    }),
  }
}

export const actionsTokenQuery = queryOptions({
  queryKey: tokenKey,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/integrations/git/actions-token')),
})

export function useActionsToken() {
  const queryClient = useQueryClient()
  const onSuccess = (status: { configured: boolean; updated_at: string | null }) => {
    queryClient.setQueryData(tokenKey, status)
    void queryClient.invalidateQueries({ queryKey: ['site-updates'], refetchType: 'none' })
  }
  return {
    save: useMutation({ mutationFn: async (token: string) => unwrap(await api.PUT('/api/v1/admin/integrations/git/actions-token', { body: { token } })), onSuccess }),
    clear: useMutation({ mutationFn: async () => unwrap(await api.DELETE('/api/v1/admin/integrations/git/actions-token')), onSuccess }),
  }
}
