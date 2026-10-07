import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { UpdateStatus } from '@/types/api'

export const systemKeys = {
  update: ['system', 'update'] as const,
  liveVersion: ['system', 'live-version'] as const,
  backups: ['system', 'backups'] as const,
}

/** Derniere sauvegarde reussie et journal, pour Paramètres > Sauvegardes. */
export const backupStatusQuery = queryOptions({
  queryKey: systemKeys.backups,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/system/backups')),
  refetchInterval: 60_000,
})

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
  // Toutes les trois secondes le temps qu'une verification demandee aboutisse,
  // cinq pendant une installation, deux minutes sinon : une version detectee
  // par la tache de fond apparait vite, et l'etat se lit en base, sans appel
  // a GitHub.
  refetchInterval: (query) =>
    query.state.data?.check_pending === true ? 3_000 : query.state.data?.in_progress === true ? 5_000 : 2 * 60_000,
  // Pendant le redeploiement, le serveur disparait quelques instants : une
  // erreur passagere ne doit pas faire disparaitre l'etat affiche.
  retry: 3,
})

/**
 * Demande une verification immediate des versions, puis attend son resultat.
 *
 * La verification part de la tache de fond, dans les quinze secondes : on relit l'etat
 * jusqu'a ce qu'elle soit faite, pour dire au clic « a jour » ou « disponible ».
 */
export function useCheckForUpdate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (): Promise<UpdateStatus> => {
      let status = unwrap(await api.POST('/api/v1/admin/system/update/check'))
      const deadline = Date.now() + 90_000

      while (status.check_pending && Date.now() < deadline) {
        await new Promise((resolve) => setTimeout(resolve, 3_000))
        status = unwrap(await api.GET('/api/v1/admin/system/update'))
      }

      return status
    },
    onSuccess: (status) => queryClient.setQueryData(systemKeys.update, status),
  })
}

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
