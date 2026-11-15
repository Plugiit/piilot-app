import { keepPreviousData, queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api, apiUrl, unwrap } from '@/lib/api'
import { clientKeys } from '@/features/clients/api'
import type { RgpdErasureItem } from '@/types/api'

import { isExportActive, type ErasureStatus, type SubjectType } from './labels'

/** Intervalle de sondage tant qu'une archive se construit. */
const POLL_MS = 3000

export const rgpdKeys = {
  all: ['rgpd'] as const,
  exports: (page: number) => ['rgpd', 'exports', page] as const,
  erasures: (status: ErasureStatus | undefined, page: number) => ['rgpd', 'erasures', status ?? 'all', page] as const,
  scheduled: (subjectType: SubjectType, subjectId: string) => ['rgpd', 'erasures', 'scheduled', subjectType, subjectId] as const,
  settings: ['rgpd', 'settings'] as const,
}

export function exportsQuery(page: number) {
  return queryOptions({
    queryKey: rgpdKeys.exports(page),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/rgpd/exports', { params: { query: { page } } })),
    placeholderData: keepPreviousData,
    // Une archive demandée passe seule de « En attente » à « Prêt ».
    refetchInterval: (query) => (query.state.data?.items.some((i) => isExportActive(i.status)) ? POLL_MS : false),
  })
}

export function erasuresQuery(status: ErasureStatus | undefined, page: number) {
  return queryOptions({
    queryKey: rgpdKeys.erasures(status, page),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/rgpd/erasures', { params: { query: { status, page } } })),
    placeholderData: keepPreviousData,
  })
}

/** Délai de rétractation en vigueur : il ne change pas sans redémarrage. */
export const settingsQuery = queryOptions({
  queryKey: rgpdKeys.settings,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/rgpd/settings')),
  staleTime: 5 * 60_000,
})

/**
 * L'effacement planifié d'un sujet, demandé à l'API par `subject_type` et
 * `subject_id` : la réponse ne dépend pas du nombre d'effacements en cours.
 * Désactivé sans `rgpd.manage` : l'API répondrait 403.
 */
export function scheduledErasureQuery(subjectType: SubjectType, subjectId: string, enabled: boolean) {
  return queryOptions({
    queryKey: rgpdKeys.scheduled(subjectType, subjectId),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/rgpd/erasures', {
          params: { query: { status: 'scheduled', subject_type: subjectType, subject_id: subjectId, page: 1 } },
        }),
      ),
    enabled,
  })
}

/** L'effacement planifié d'un sujet, ou undefined. */
export function useScheduledErasure(subjectType: SubjectType, subjectId: string, enabled: boolean): RgpdErasureItem | undefined {
  const { data } = useQuery(scheduledErasureQuery(subjectType, subjectId, enabled))

  return data?.items[0]
}

export function useRequestExport() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (subject: { type: SubjectType; id: string }) =>
      unwrap(await api.POST('/api/v1/admin/rgpd/exports', { body: { subject_type: subject.type, subject_id: subject.id } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['rgpd', 'exports'] })
    },
  })
}

export function useRequestErasure() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (subject: { type: SubjectType; id: string }) =>
      unwrap(await api.POST('/api/v1/admin/rgpd/erasures', { body: { subject_type: subject.type, subject_id: subject.id } })),
    onSuccess: () => {
      // Les comptes du sujet sont désactivés tout de suite : leurs écrans changent aussi.
      void queryClient.invalidateQueries({ queryKey: rgpdKeys.all })
      void queryClient.invalidateQueries({ queryKey: ['accounts'] })
      void queryClient.invalidateQueries({ queryKey: clientKeys.all })
    },
  })
}

export function useCancelErasure() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.POST('/api/v1/admin/rgpd/erasures/{id}/cancel', { params: { path: { id } } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: rgpdKeys.all })
      void queryClient.invalidateQueries({ queryKey: ['accounts'] })
      void queryClient.invalidateQueries({ queryKey: clientKeys.all })
    },
  })
}

/** Adresse de l'archive d'un export prêt : un lien, le navigateur télécharge. */
export function exportDownloadUrl(id: string): string {
  return apiUrl(`/api/v1/admin/rgpd/exports/${id}/download`)
}
