import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { DeliverableStatus } from '@/types/api'

/** Ce que la barre d'outils peut demander. */
export interface DeliverableFilters {
  search?: string
  status?: DeliverableStatus
  projectId?: string
}

export const deliverableKeys = {
  all: ['deliverables'] as const,
  list: (f: DeliverableFilters, page: number) => [...deliverableKeys.all, 'list', f, page] as const,
}

/** Vingt-cinq lignes par page, comme la valeur par defaut du serveur. */
export const DELIVERABLES_PAGE_SIZE = 25

/**
 * Les livrables de l'agence, tous projets confondus.
 *
 * C'est la file d'attente : ce qui attend une reponse du client, et depuis
 * quand. `keepPreviousData` garde la page precedente pendant le chargement de
 * la suivante, pour que le tableau ne clignote pas.
 */
export function deliverableListQuery(f: DeliverableFilters, page: number) {
  return queryOptions({
    queryKey: deliverableKeys.list(f, page),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/deliverables', {
          params: {
            query: {
              search: f.search,
              status: f.status,
              project_id: f.projectId,
              page,
              page_size: DELIVERABLES_PAGE_SIZE,
            },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

export interface CreateDeliverableValues {
  title: string
  description: string
  url: string
}

/** Depose un livrable et sa premiere version sur un projet. */
export function useCreateDeliverable(projectId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: CreateDeliverableValues) =>
      unwrap(
        await api.POST('/api/v1/admin/projects/{id}/deliverables', {
          params: { path: { id: projectId } },
          body: values,
        }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: deliverableKeys.all })
    },
  })
}

/** Soumet une nouvelle version : la v2 apres des retours. */
export function useSubmitVersion(deliverableId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (url: string) =>
      unwrap(
        await api.POST('/api/v1/admin/deliverables/{id}/versions', {
          params: { path: { id: deliverableId } },
          body: { url },
        }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: deliverableKeys.all })
    },
  })
}

export interface DecisionValues {
  decision: 'valide' | 'retours'
  feedback: string
}

/**
 * Enregistre la reponse du client sur la version courante.
 *
 * Depuis le back-office, c'est l'agence qui la saisit : une validation recue au
 * telephone doit pouvoir etre tracee. Le portail fera le meme appel au nom du
 * client, et le role du compte dira lequel des deux a tranche.
 */
export function useDecideDeliverable(deliverableId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: DecisionValues) =>
      unwrap(
        await api.POST('/api/v1/admin/deliverables/{id}/decision', {
          params: { path: { id: deliverableId } },
          body: values,
        }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: deliverableKeys.all })
    },
  })
}
