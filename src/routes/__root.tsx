import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import { Toaster } from 'sonner'

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
      <p className="text-6xl font-semibold text-ink">404</p>
      <p className="text-ink-muted text-sm">Cette page n'existe pas.</p>
    </div>
  )
}
