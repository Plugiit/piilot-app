import { QueryClientProvider } from '@tanstack/react-query'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { toast } from 'sonner'

import { createQueryClient } from '@/lib/query-client'
import { routeTree } from './routeTree.gen'

import './index.css'

/**
 * Une seule redirection par expiration.
 *
 * Un ecran emet plusieurs requetes : quand la session expire, elles echouent
 * toutes en meme temps. Sans ce verrou, on empilerait autant de messages et de
 * navigations qu'il y avait d'appels en vol. Il se leve a la connexion
 * suivante, quand `/login` rend la main.
 */
let redirecting = false

const queryClient = createQueryClient(() => {
  // Deja sur la connexion : il n'y a rien a quitter, et y renvoyer bouclerait.
  if (redirecting || router.state.location.pathname === '/login') return

  redirecting = true

  // `redirect` ramene l'utilisateur ou il etait une fois reconnecte, comme le
  // fait deja la garde de session.
  const from = router.state.location.href

  toast.error('Votre session a expiré. Reconnectez-vous pour continuer.')

  void router.navigate({ to: '/login', search: { redirect: from } }).then(() => {
    // Le cache porte les donnees de la session expiree. Il est vide apres la
    // navigation et non pendant : le purger depuis le gestionnaire d'erreur du
    // cache reviendrait a le modifier au milieu de son propre parcours.
    queryClient.clear()
    redirecting = false
  })
})

const router = createRouter({
  routeTree,
  context: { queryClient },
  // Le prefetch se declenche au survol ; 50 ms de delai evitent de precharger
  // toutes les routes qu'un curseur traverse en passant.
  defaultPreload: 'intent',
  defaultPreloadDelay: 50,
  // Les loaders relisent le cache TanStack Query, qui porte deja sa propre
  // politique de fraicheur : desactiver celle du routeur evite deux caches
  // concurrents avec des durees differentes.
  defaultPreloadStaleTime: 0,
  scrollRestoration: true,
})

// Typage global du routeur : `<Link to="...">` et `Route.useSearch()` sont
// verifies a la compilation.
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

const rootElement = document.getElementById('root')

if (!rootElement) {
  throw new Error("L'element racine #root est introuvable")
}

createRoot(rootElement).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
