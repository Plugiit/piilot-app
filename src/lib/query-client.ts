import { QueryClient } from '@tanstack/react-query'

import { HttpError } from '@/lib/api'

/**
 * Politique de cache de l'admin.
 *
 * `staleTime` a 30 s est le reglage qui donne la sensation d'instantaneite :
 * revenir sur un ecran deja visite dans la minute n'emet aucune requete. Des
 * donnees d'administration ne changent pas assez vite pour que ce delai pose
 * probleme, et toute mutation invalide explicitement ses cles.
 */
export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        gcTime: 5 * 60_000,
        refetchOnWindowFocus: false,
        retry(failureCount, error) {
          // Reessayer un 401 ou un 404 ne peut pas reussir et retarde
          // l'affichage de l'erreur.
          if (error instanceof HttpError && error.status < 500) {
            return false
          }
          return failureCount < 2
        },
      },
      mutations: {
        retry: false,
      },
    },
  })
}
