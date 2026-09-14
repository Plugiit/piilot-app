import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'

import { api, apiUrl, unwrap } from '@/lib/api'

export const notificationKeys = {
  all: ['notifications'] as const,
  feed: () => [...notificationKeys.all, 'feed'] as const,
}

/**
 * Panneau de notifications.
 *
 * Une seule requete pour la liste et le compteur : la cloche et le panneau
 * montrent deux faces de la meme reponse, et les separer ferait clignoter l'un
 * pendant que l'autre se recharge.
 */
export const notificationFeedQuery = queryOptions({
  queryKey: notificationKeys.feed(),
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/notifications')),
  // Le flux previent des nouveautes ; ce delai n'est qu'un filet quand il est
  // coupe.
  staleTime: 60_000,
})

/** Marque une notification comme lue. */
export function useMarkNotificationRead() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(
        await api.POST('/api/v1/admin/notifications/{id}/read', {
          params: { path: { id } },
        }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: notificationKeys.all }),
  })
}

/** Vide le compteur. */
export function useMarkAllNotificationsRead() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/notifications/read')),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: notificationKeys.all }),
  })
}

/**
 * Branche l'ecran sur le flux des notifications.
 *
 * `EventSource` plutot qu'une lecture continue ecrite a la main : il se
 * reconnecte tout seul quand la connexion tombe — reveil de veille, changement
 * de reseau — et c'est precisement ce qu'on ne veut pas reecrire.
 *
 * Le message recu ne sert qu'a declencher une relecture : le serveur n'envoie
 * que l'identifiant et le genre, la liste reste la seule source de verite. Deux
 * formats a tenir pour economiser une requete ne vaudrait pas la divergence qui
 * finirait par s'installer entre eux.
 */
export function useNotificationStream() {
  const queryClient = useQueryClient()

  useEffect(() => {
    // `withCredentials` : la session vit dans un cookie httpOnly, et
    // EventSource ne l'envoie pas sans cela des que l'API est sur un autre
    // sous-domaine.
    const source = new EventSource(apiUrl('/api/v1/admin/notifications/stream'), {
      withCredentials: true,
    })

    source.addEventListener('notification', () => {
      void queryClient.invalidateQueries({ queryKey: notificationKeys.all })
    })

    return () => source.close()
  }, [queryClient])
}
