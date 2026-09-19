import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'

import { HttpError } from '@/lib/api'

/**
 * Statuts qui justifient une nouvelle tentative.
 *
 * Liste blanche et non « tout ce qui est >= 500 » : la famille 5xx melange des
 * pannes passageres (502 le temps qu'un conteneur redemarre) et des reponses
 * definitives. 501 NOT_IMPLEMENTED en est une — la route existe, le handler
 * n'est pas ecrit, et le reessayer ne peut pas changer le resultat.
 */
const RETRYABLE_STATUSES = new Set([
  500, // erreur interne, potentiellement transitoire
  502, // passerelle en cours de bascule
  503, // service momentanement indisponible
  504, // depassement de delai en amont
])

/**
 * Politique de cache de l'application.
 *
 * `staleTime` a 30 s est le reglage qui donne la sensation d'instantaneite :
 * revenir sur un ecran deja visite dans la minute n'emet aucune requete. Des
 * donnees d'administration ne changent pas assez vite pour que ce delai pose
 * probleme, et toute mutation invalide explicitement ses cles.
 */
export function createQueryClient(onUnauthorized: () => void) {
  // Un 401 peut tomber sur n'importe quelle requete, y compris pendant qu'on
  // remplit un formulaire. La garde de session, elle, vit dans `beforeLoad` et
  // ne s'execute donc qu'a la navigation : une session qui expire sur place
  // n'etait vue par personne, et l'ecran se couvrait de messages « Non
  // authentifie » sans jamais proposer de se reconnecter.
  //
  // Les deux caches sont couverts : une lecture qui expire et un enregistrement
  // qui expire meritent la meme issue.
  function reportUnauthorized(error: unknown) {
    if (error instanceof HttpError && error.isUnauthorized) onUnauthorized()
  }

  return new QueryClient({
    queryCache: new QueryCache({ onError: reportUnauthorized }),
    mutationCache: new MutationCache({ onError: reportUnauthorized }),
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        gcTime: 5 * 60_000,
        refetchOnWindowFocus: false,
        retry(failureCount, error) {
          // Une erreur HTTP dont le statut n'est pas dans la liste blanche est
          // definitive : la reessayer ne fait que retarder l'affichage du
          // message, de 3 secondes avec le backoff par defaut.
          if (error instanceof HttpError) {
            return RETRYABLE_STATUSES.has(error.status) && failureCount < 2
          }

          // Pas de statut : panne reseau ou serveur injoignable. C'est le cas
          // ou reessayer a un sens.
          return failureCount < 2
        },
      },
      mutations: {
        retry: false,
      },
    },
  })
}
