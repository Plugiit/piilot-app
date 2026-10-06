import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet, redirect, useRouterState } from '@tanstack/react-router'
import { useEffect } from 'react'

import { ErrorState } from '@/components/layout/error-state'
import { Toaster } from '@/components/ui/sonner'
import { VersionBanner } from '@/features/system/version-banner'
import { crossSpaceUrl } from '@/lib/spaces'

/**
 * Le QueryClient est injecte dans le contexte du routeur : les loaders de
 * route peuvent precharger leurs donnees (`queryClient.query`) avant que le
 * composant ne soit rendu, ce qui supprime le flash de skeleton sur une
 * navigation prefetchee.
 */
export interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  // Quand chaque espace a son domaine, une navigation qui sort de l'espace
  // courant — un lien vers les parametres depuis l'equipe, la deconnexion —
  // repart sur le domaine de destination. Le serveur fait de meme pour une
  // adresse ouverte directement. En mono-domaine, rien ne se passe.
  beforeLoad: ({ location }) => {
    const target = crossSpaceUrl(location.pathname, location.searchStr)
    if (target !== null) throw redirect({ href: target })
  },
  component: RootLayout,
  // Filet ultime : ce qui echoue hors d'une vue (garde de route, session).
  // Les vues, elles, declarent le leur pour rester dans leur shell.
  errorComponent: ErrorState,
  notFoundComponent: NotFound,
})

function RootLayout() {
  // Filet de la garde ci-dessus : une redirection lancee par une garde enfant
  // — vers la connexion quand la session manque, apres une deconnexion — ne
  // repasse pas par le `beforeLoad` racine. Chaque adresse atteinte est donc
  // reverifiee ici, et le domaine change s'il le faut.
  const location = useRouterState({ select: (state) => state.location })
  useEffect(() => {
    const target = crossSpaceUrl(location.pathname, location.searchStr)
    if (target !== null) window.location.replace(target)
  }, [location.pathname, location.searchStr])

  return (
    <>
      <Outlet />
      <Toaster position="bottom-right" richColors closeButton />
      {/* A la racine : un onglet reste ouvert apres un deploiement doit etre
          prevenu ou qu'il soit, connexion comprise. */}
      <VersionBanner />
    </>
  )
}

function NotFound() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2">
      <p className="text-6xl font-semibold">404</p>
      <p className="text-muted-foreground text-sm">Cette page n'existe pas.</p>
    </div>
  )
}
