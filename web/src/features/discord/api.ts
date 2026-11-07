import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { DiscordStatus } from '@/types/api'

import { linkedSince, type PendingCode } from './format'

const key = ['discord', 'me'] as const

/**
 * La liaison Discord du compte connecte.
 *
 * C'est le bot qui lie le compte, quand le membre tape le code : Piilot ne
 * l'apprend qu'en relisant. Pendant qu'un code attend, la carte relit donc
 * l'etat toutes les quelques secondes, et seulement pendant ce temps-la :
 * jusqu'a ce que le code serve ou expire.
 */
export function discordStatusQuery(pending: PendingCode | null) {
  return queryOptions({
    queryKey: key,
    queryFn: async () => unwrap(await api.GET('/api/v1/auth/me/discord')),
    refetchInterval: (query) =>
      pending !== null && Date.now() < pending.expiresAt.getTime() && !linkedSince(query.state.data, pending) ? 4_000 : false,
  })
}

export function useDiscordActions() {
  const queryClient = useQueryClient()
  return {
    // Tirer un code invalide le precedent : il n'y a rien a mettre en cache,
    // le code ne vit que dans la carte qui l'a demande.
    createCode: useMutation({
      mutationFn: async () => unwrap(await api.POST('/api/v1/auth/me/discord/code')),
    }),
    unlink: useMutation({
      mutationFn: async () => {
        unwrap(await api.DELETE('/api/v1/auth/me/discord'))
      },
      onSuccess: () =>
        queryClient.setQueryData<DiscordStatus>(key, (status) =>
          status === undefined ? status : { ...status, linked: false, discord_user_id: null, linked_at: null },
        ),
    }),
  }
}
