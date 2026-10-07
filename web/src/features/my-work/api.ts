import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'

export const myWorkKeys = {
  all: ['my-work'] as const,
}

/**
 * Page « Mon travail ».
 *
 * Aucun identifiant envoye : le serveur lit la personne dans la session, et la
 * page ne montre jamais que son propre travail.
 */
export const myWorkQuery = queryOptions({
  queryKey: myWorkKeys.all,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/me/work')),
  // Relue a chaque retour sur la page : elle reprend des taches, des tickets
  // et des livrables que d'autres ecrans modifient, et tenir la liste de tout
  // ce qui doit l'invalider serait plus fragile qu'un appel borne de plus.
  // Le cache s'affiche pendant ce temps : la page ne clignote pas.
  staleTime: 0,
})

/**
 * Changer le statut d'une tache depuis « Mon travail ».
 *
 * La page est relue, et avec elle tout ce qui montre des taches : la liste du
 * module, le tableau du projet et ses compteurs.
 */
export function useWorkTaskMove() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: 'todo' | 'progress' | 'review' | 'done' }) =>
      unwrap(await api.POST('/api/v1/admin/tasks/{id}/move', { params: { path: { id } }, body: { status } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: myWorkKeys.all })
      void queryClient.invalidateQueries({ queryKey: ['tasks'] })
      void queryClient.invalidateQueries({ queryKey: ['projects'] })
    },
  })
}
