import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { ReplyTemplate, TicketDetail } from '@/types/api'

export const replyTemplateKeys = {
  all: ['reply-templates'] as const,
}

/** Les reponses types : quelques dizaines, chargees une fois. */
export const replyTemplatesQuery = queryOptions({
  queryKey: replyTemplateKeys.all,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/ticket-reply-templates')),
  staleTime: 5 * 60_000,
})

export function useSaveReplyTemplate() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (values: { id?: string; title: string; body: string }) =>
      values.id === undefined
        ? unwrap(await api.POST('/api/v1/admin/ticket-reply-templates', { body: { title: values.title, body: values.body } }))
        : unwrap(
            await api.PATCH('/api/v1/admin/ticket-reply-templates/{id}', {
              params: { path: { id: values.id } },
              body: { title: values.title, body: values.body },
            }),
          ),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: replyTemplateKeys.all }),
  })
}

export function useDeleteReplyTemplate() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      const { error } = await api.DELETE('/api/v1/admin/ticket-reply-templates/{id}', { params: { path: { id } } })
      if (error !== undefined) throw error
    },
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: replyTemplateKeys.all }),
  })
}

/**
 * Remplit les variables d'une reponse type avec ce que le ticket sait :
 * {prenom} de qui a ouvert la demande, {numero}, {sujet}, {projet}.
 */
export function fillTemplate(template: ReplyTemplate, ticket: TicketDetail): string {
  const firstname =
    ticket.reporter?.firstname ?? ticket.requester?.name.trim().split(/\s+/)[0] ?? ''
  return template.body
    .replaceAll('{prenom}', firstname)
    .replaceAll('{numero}', `#${ticket.numero}`)
    .replaceAll('{sujet}', ticket.subject)
    .replaceAll('{projet}', ticket.project.name)
    // « Bonjour {prenom}, » sans prenom ne laisse pas d'espace orpheline.
    .replace(/ +,/g, ',')
}
