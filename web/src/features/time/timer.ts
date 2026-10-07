import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { timeKeys } from '@/features/time/api'
import { api, unwrap } from '@/lib/api'
import type { Timer } from '@/types/api'

export const timerKeys = {
  current: ['time-timer'] as const,
}

/**
 * Le chrono en cours. Relu toutes les minutes et au retour sur l'onglet : il
 * a pu etre lance ou arrete depuis un autre poste.
 */
export const timerQuery = queryOptions({
  queryKey: timerKeys.current,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/time-timer')).timer,
  refetchInterval: 60_000,
  refetchOnWindowFocus: true,
  staleTime: 30_000,
})

function useTimerInvalidation() {
  const queryClient = useQueryClient()

  return (timer: Timer | null) => {
    queryClient.setQueryData(timerKeys.current, timer)
    // Une saisie vient peut-etre d'etre ecrite : feuille, projets, Mon travail.
    void queryClient.invalidateQueries({ queryKey: timeKeys.all })
    void queryClient.invalidateQueries({ queryKey: ['projects'] })
    void queryClient.invalidateQueries({ queryKey: ['my-work'] })
    void queryClient.invalidateQueries({ queryKey: ['dashboard'] })
  }
}

export function useStartTimer() {
  const settle = useTimerInvalidation()

  return useMutation({
    mutationFn: async (values: { project_id: string; task_id?: string | null; note?: string }) =>
      unwrap(await api.POST('/api/v1/admin/time-timer', { body: values })).timer,
    onSuccess: (timer) => settle(timer),
  })
}

export function useStopTimer() {
  const settle = useTimerInvalidation()

  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/time-timer/stop')).entry,
    onSuccess: () => settle(null),
  })
}

export function useDiscardTimer() {
  const settle = useTimerInvalidation()

  return useMutation({
    mutationFn: async () => unwrap(await api.DELETE('/api/v1/admin/time-timer')),
    onSuccess: () => settle(null),
  })
}
