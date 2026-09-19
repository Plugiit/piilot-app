import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'

import { ErrorState } from '@/components/layout/error-state'
import { Toaster } from '@/components/ui/sonner'

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
  component: RootLayout,
  // Filet ultime : ce qui echoue hors d'une vue (garde de route, session).
  // Les vues, elles, declarent le leur pour rester dans leur shell.
  errorComponent: ErrorState,
  notFoundComponent: NotFound,
})

function RootLayout() {
  return (
    <>
      <Outlet />
      <Toaster position="bottom-right" richColors closeButton />
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
