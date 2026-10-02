import { queryOptions } from '@tanstack/react-query'

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
