import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { uptimeKeys } from '@/features/uptime/api'
import { api, unwrap } from '@/lib/api'
import type { CoolifySettings, ProjectCoolify } from '@/types/api'

export const coolifyKeys = {
  settings: ['coolify', 'settings'] as const,
  applications: ['coolify', 'applications'] as const,
  project: (id: string) => ['coolify', 'project', id] as const,
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

/** Les applications relevees : sert au choix d'une application pour un projet. */
export const coolifyApplicationsQuery = queryOptions({
  queryKey: coolifyKeys.applications,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/modules/coolify/applications')),
})

export function projectCoolifyQuery(id: string) {
  return queryOptions({
    queryKey: coolifyKeys.project(id),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/projects/{id}/coolify', { params: { path: { id } } })),
  })
}

export function useSetProjectCoolify(id: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (applications: { application_uuid: string; environment: 'prod' | 'preprod' }[]) =>
      unwrap(await api.PUT('/api/v1/admin/projects/{id}/coolify', { params: { path: { id } }, body: { applications } })),
    onSuccess: (view: ProjectCoolify) => {
      queryClient.setQueryData(coolifyKeys.project(id), view)
      // La relève demandée remplira les déploiements : l'écran du module la suit.
      void queryClient.invalidateQueries({ queryKey: coolifyKeys.settings })
    },
  })
}
