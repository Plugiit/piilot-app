import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { ClaudeAccount } from '@/types/api'

const key = ['claude', 'me'] as const

/**
 * La carte « Claude » du compte : l'adresse du serveur MCP a saisir dans
 * Claude, et les connexions en cours.
 */
export const claudeQuery = queryOptions({
  queryKey: key,
  queryFn: async () => unwrap(await api.GET('/api/v1/auth/me/claude')),
})

/** Coupe une connexion : Claude est refuse des sa requete suivante. */
export function useDisconnectClaude() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      unwrap(await api.DELETE('/api/v1/auth/me/claude/connections/{id}', { params: { path: { id } } }))
    },
    onSuccess: (_, id) =>
      queryClient.setQueryData<ClaudeAccount>(key, (account) =>
        account === undefined ? account : { ...account, connections: account.connections.filter((c) => c.id !== id) },
      ),
  })
}

/**
 * Une demande de connexion en attente du consentement. Elle ne se relit pas
 * d'elle-meme : une demande ne change pas, elle expire.
 */
export function consentQuery(id: string) {
  return queryOptions({
    queryKey: ['claude', 'consent', id],
    queryFn: async () => unwrap(await api.GET('/api/v1/auth/oauth/requests/{id}', { params: { path: { id } } })),
    retry: false,
    staleTime: Infinity,
  })
}

/** Accepte ou refuse la demande ; rend l'adresse ou renvoyer le navigateur. */
export function useDecide(id: string) {
  return useMutation({
    mutationFn: async (decision: { approve: boolean; scopes: Array<'piilot:read' | 'piilot:write'> }) =>
      unwrap(
        await api.POST('/api/v1/auth/oauth/requests/{id}/decision', {
          params: { path: { id } },
          body: decision,
        }),
      ).redirect_to,
  })
}
