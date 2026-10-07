import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, apiUrl, postFile, unwrap } from '@/lib/api'
import type { PortalFile } from '@/types/api'

/**
 * Le portail lit `/api/v1/client/*` : des endpoints a lui, isoles cote serveur
 * par le client de l'appelant. Aucun identifiant de client ne part d'ici.
 */
export const portalKeys = {
  all: ['portal'] as const,
  projects: () => [...portalKeys.all, 'projects'] as const,
  project: (id: string) => [...portalKeys.all, 'project', id] as const,
  deliverable: (id: string) => [...portalKeys.all, 'deliverable', id] as const,
  tickets: (state: 'open' | 'closed', page: number) => [...portalKeys.all, 'tickets', state, page] as const,
  ticket: (id: string) => [...portalKeys.all, 'ticket', id] as const,
  review: (id: string, token: string) => [...portalKeys.all, 'review', id, token] as const,
}

/**
 * La page de reponse ouverte depuis l'e-mail : sans session, le jeton signe
 * du lien fait foi. Rien n'est decide en la lisant.
 */
export function portalReviewQuery(id: string, token: string) {
  return queryOptions({
    queryKey: portalKeys.review(id, token),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/public/deliverables/{id}/review', { params: { path: { id }, query: { token } } })),
  })
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

/** Les demandes du client, ouvertes ou closes. */
export function portalTicketsQuery(state: 'open' | 'closed', page: number) {
  return queryOptions({
    queryKey: portalKeys.tickets(state, page),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/client/tickets', { params: { query: { state, page } } })),
  })
}

export function portalTicketQuery(id: string) {
  return queryOptions({
    queryKey: portalKeys.ticket(id),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/client/tickets/{id}', { params: { path: { id } } })),
  })
}

export interface NewTicketValues {
  project_id: string
  tracker: 'anomalie' | 'evolution' | 'assistance'
  priority: 'low' | 'normal' | 'high'
  subject: string
  description: string
}

/**
 * Depose une demande, puis ses pieces jointes une a une. Une piece jointe qui
 * echoue ne fait pas tomber la demande : elle est deja partie, et le client
 * le saura par le message d'erreur.
 */
export function useCreateTicket() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ values, files }: { values: NewTicketValues; files: File[] }) => {
      const ticket = unwrap(await api.POST('/api/v1/client/tickets', { body: values }))
      const failed: string[] = []
      for (const file of files) {
        try {
          await postFile<PortalFile>(`/api/v1/client/tickets/${ticket.id}/files`, 'file', file)
        } catch {
          failed.push(file.name)
        }
      }
      return { ticket, failed }
    },
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: [...portalKeys.all, 'tickets'] }),
  })
}

export function useReplyTicket(ticketId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (body: string) =>
      unwrap(
        await api.POST('/api/v1/client/tickets/{id}/messages', { params: { path: { id: ticketId } }, body: { body } }),
      ),
    onSuccess: (ticket) => {
      queryClient.setQueryData(portalKeys.ticket(ticketId), ticket)
      void queryClient.invalidateQueries({ queryKey: [...portalKeys.all, 'tickets'] })
    },
  })
}

export function useAttachToTicket(ticketId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (file: File) => postFile<PortalFile>(`/api/v1/client/tickets/${ticketId}/files`, 'file', file),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: portalKeys.ticket(ticketId) }),
  })
}
