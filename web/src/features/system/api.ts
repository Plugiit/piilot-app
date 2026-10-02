import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { UpdateStatus } from '@/types/api'

export const systemKeys = {
  update: ['system', 'update'] as const,
  liveVersion: ['system', 'live-version'] as const,
}

/**
 * Etat de la mise a jour, pour les admins.
 *
 * Releve toutes les cinq secondes pendant une mise a jour, pour suivre le
 * deploiement ; toutes les dix minutes sinon, ce qui suffit a voir apparaitre
 * une version que la tache de fond ne verifie que toutes les six heures.
 */
export const updateStatusQuery = queryOptions({
  queryKey: systemKeys.update,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/system/update')),
  refetchInterval: (query) => (query.state.data?.in_progress === true ? 5_000 : 10 * 60_000),
  // Pendant le redeploiement, le serveur disparait quelques instants : une
  // erreur passagere ne doit pas faire disparaitre l'etat affiche.
  retry: 3,
})

/** Lance la mise a jour vers la derniere version. */
export function useRequestUpdate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/system/update')),
    onSuccess: (status: UpdateStatus) => {
      queryClient.setQueryData(systemKeys.update, status)
    },
  })
}

/**
 * Version du serveur, lue sur la sonde de vivacite.
 *
 * Relue au retour sur l'onglet et toutes les deux minutes : c'est ce qui dit a
 * un onglet reste ouvert qu'une nouvelle version a ete deployee pendant ce
 * temps.
 */
export const liveVersionQuery = queryOptions({
  queryKey: systemKeys.liveVersion,
  queryFn: async () => unwrap(await api.GET('/health/live')).version,
  refetchInterval: 2 * 60_000,
  refetchOnWindowFocus: true,
  staleTime: 30_000,
  retry: false,
})
