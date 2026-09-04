import { QueryClientProvider } from '@tanstack/react-query'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { createQueryClient } from '@/lib/query-client'
import { routeTree } from './routeTree.gen'

import './index.css'

const queryClient = createQueryClient()

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
