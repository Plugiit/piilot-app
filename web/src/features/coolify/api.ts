import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { uptimeKeys } from '@/features/uptime/api'
import { api, unwrap } from '@/lib/api'
import type { CoolifySettings } from '@/types/api'

export const coolifyKeys = {
  settings: ['coolify', 'settings'] as const,
}

/**
 * Reglages et etat de la releve. Relus toutes les trois secondes tant
 * qu'une releve demandee — un « Tester » — n'a pas eu lieu.
 */
export const coolifySettingsQuery = queryOptions({
  queryKey: coolifyKeys.settings,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/modules/coolify')),
  refetchInterval: (query) => (query.state.data?.status.poll_pending === true ? 3_000 : 60_000),
})

export interface CoolifySettingsValues {
  enabled: boolean
  base_url: string
  /** Absent : garde le jeton en place. */
  api_token?: string
  poll_minutes: number
}

function useSetSettings() {
  const queryClient = useQueryClient()
  return (settings: CoolifySettings) => {
    queryClient.setQueryData(coolifyKeys.settings, settings)
    // La carte de la page des modules porte l'etat.
    void queryClient.invalidateQueries({ queryKey: uptimeKeys.modules })
  }
}

export function useUpdateCoolifySettings() {
  const set = useSetSettings()
  return useMutation({
    mutationFn: async (body: CoolifySettingsValues) => unwrap(await api.PUT('/api/v1/admin/modules/coolify', { body })),
    onSuccess: set,
  })
}

export function useTestCoolify() {
  const set = useSetSettings()
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/modules/coolify/poll')),
    onSuccess: set,
  })
}
