import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { GitSync } from '@/types/api'

const key = (projectId: string) => ['git-sync', projectId] as const

/**
 * Ou en est la synchronisation. Relue toutes les trois secondes tant qu'une
 * synchronisation demandee n'a pas eu lieu.
 */
export function gitSyncQuery(projectId: string) {
  return queryOptions({
    queryKey: key(projectId),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/projects/{id}/git/sync', { params: { path: { id: projectId } } })),
    refetchInterval: (query) => (query.state.data?.pending === true ? 3_000 : false),
  })
}

export function useRequestGitSync(projectId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/projects/{id}/git/sync', { params: { path: { id: projectId } } })),
    onSuccess: (view: GitSync) => queryClient.setQueryData(key(projectId), view),
  })
}
