import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'

export const timeKeys = {
  all: ['time-entries'] as const,
  sheet: (from: string, to: string) => [...timeKeys.all, from, to] as const,
}

/**
 * Ma feuille de temps sur une plage de jours.
 *
 * Aucun identifiant de compte n'est envoye : le serveur le lit dans la session.
 * Le passer en parametre laisserait demander le pointage de quelqu'un d'autre
 * en changeant l'adresse.
 */
export function timeSheetQuery(from: string, to: string) {
  return queryOptions({
    queryKey: timeKeys.sheet(from, to),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/time-entries', { params: { query: { from, to } } })),
  })
}

/** Ce que le formulaire envoie. */
export interface TimeEntryValues {
  project_id: string
  task_id?: string | null
  service_id?: string | null
  spent_on: string
  minutes: number
  note?: string
}

export function useCreateTimeEntry() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: TimeEntryValues) =>
      unwrap(await api.POST('/api/v1/admin/time-entries', { body: values })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: timeKeys.all })
      // Les heures consommees d'un projet changent avec la saisie : les ecrans
      // qui les affichent doivent les relire.
      void queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })
}

export function useUpdateTimeEntry(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: TimeEntryValues) =>
      unwrap(
        await api.PATCH('/api/v1/admin/time-entries/{id}', {
          params: { path: { id } },
          body: values,
        }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: timeKeys.all })
      void queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })
}

export function useDeleteTimeEntry(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () =>
      unwrap(await api.DELETE('/api/v1/admin/time-entries/{id}', { params: { path: { id } } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: timeKeys.all })
      void queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })
}
