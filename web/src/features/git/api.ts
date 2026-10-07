import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'

export const gitKeys = {
  settings: ['git', 'settings'] as const,
}

/** Adresse du webhook et son secret, pour l'ecran de reglage. */
export const gitSettingsQuery = queryOptions({
  queryKey: gitKeys.settings,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/integrations/git')),
})

/** Tire un nouveau secret : les webhooks configures avec l'ancien cessent d'etre acceptes. */
export function useRotateGitSecret() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/integrations/git/rotate')),
    onSuccess: (settings) => queryClient.setQueryData(gitKeys.settings, settings),
  })
}
