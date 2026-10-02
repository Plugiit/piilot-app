import { infiniteQueryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { InteractionKind } from '@/types/api'

export const interactionKeys = {
  all: ['interactions'] as const,
  feed: (clientId: string | undefined, source: string | undefined) =>
    [...interactionKeys.all, clientId ?? '', source ?? ''] as const,
}

/**
 * Le journal, page apres page.
 *
 * Par curseur et non par numero de page : la liste s'allonge par le haut, et
 * un decalage ferait reapparaitre une ligne deja lue.
 */
export function interactionFeedQuery(clientId?: string, source?: string) {
  return infiniteQueryOptions({
    queryKey: interactionKeys.feed(clientId, source),
    queryFn: async ({ pageParam }) =>
      unwrap(
        await api.GET('/api/v1/admin/crm/interactions', {
          params: { query: { client_id: clientId, source, before: pageParam ?? undefined } },
        }),
      ),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.before,
  })
}

export interface InteractionValues {
  kind: Extract<InteractionKind, 'note' | 'call' | 'meeting' | 'email'>
  body: string
  occurred_at?: string | null
  project_id?: string | null
}

export function useCreateInteraction() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ clientId, values }: { clientId: string; values: InteractionValues }) =>
      unwrap(
        await api.POST('/api/v1/admin/crm/clients/{id}/interactions', {
          params: { path: { id: clientId } },
          body: values,
        }),
      ),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: interactionKeys.all }),
  })
}

export function useDeleteInteraction() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.DELETE('/api/v1/admin/crm/interactions/{id}', { params: { path: { id } } })),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: interactionKeys.all }),
  })
}
