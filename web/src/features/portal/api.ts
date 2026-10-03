import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, apiUrl, unwrap } from '@/lib/api'

/**
 * Le portail lit `/api/v1/client/*` : des endpoints a lui, isoles cote serveur
 * par le client de l'appelant. Aucun identifiant de client ne part d'ici.
 */
export const portalKeys = {
  all: ['portal'] as const,
  projects: () => [...portalKeys.all, 'projects'] as const,
  project: (id: string) => [...portalKeys.all, 'project', id] as const,
  deliverable: (id: string) => [...portalKeys.all, 'deliverable', id] as const,
}

export const portalProjectsQuery = queryOptions({
  queryKey: portalKeys.projects(),
  queryFn: async () => unwrap(await api.GET('/api/v1/client/projects')),
})

export function portalProjectQuery(id: string) {
  return queryOptions({
    queryKey: portalKeys.project(id),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/client/projects/{id}', { params: { path: { id } } })),
  })
}

export function portalDeliverableQuery(id: string) {
  return queryOptions({
    queryKey: portalKeys.deliverable(id),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/client/deliverables/{id}', { params: { path: { id } } })),
  })
}

/** Valide la version courante, ou demande des retours. */
export function useDecide(deliverableId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: { decision: 'valide' | 'retours'; feedback: string }) =>
      unwrap(
        await api.POST('/api/v1/client/deliverables/{id}/decision', {
          params: { path: { id: deliverableId } },
          body: values,
        }),
      ),
    onSuccess: (deliverable) => {
      queryClient.setQueryData(portalKeys.deliverable(deliverableId), deliverable)
      // Le compteur « a valider » des projets bouge avec la reponse.
      void queryClient.invalidateQueries({ queryKey: portalKeys.projects() })
      void queryClient.invalidateQueries({ queryKey: portalKeys.project(deliverable.project.id) })
    },
  })
}

/** Adresse de telechargement d'un fichier du portail. */
export function portalFileUrl(id: string): string {
  return apiUrl(`/api/v1/client/files/${id}`)
}
