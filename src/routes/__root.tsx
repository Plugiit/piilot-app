import type { QueryClient } from '@tanstack/react-query'
import {
  createRootRouteWithContext,
  Outlet,
  type ErrorComponentProps,
} from '@tanstack/react-router'
import { AlertTriangle } from 'lucide-react'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Toaster } from '@/components/ui/sonner'
import { HttpError } from '@/lib/api'

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
  errorComponent: ErrorPage,
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

/**
 * Filet global : sans lui, l'echec d'un loader remonte jusqu'a React et laisse
 * un ecran blanc. Les endpoints metier repondent encore 501, donc ce cas n'a
 * rien d'hypothetique aujourd'hui.
 */
function ErrorPage({ error, reset }: ErrorComponentProps) {
  const isNotImplemented = error instanceof HttpError && error.code === 'NOT_IMPLEMENTED'

  return (
    <div className="flex h-full items-center justify-center p-6">
      <Alert variant="destructive" className="max-w-md">
        <AlertTriangle />
        <AlertTitle>
          {isNotImplemented ? 'Écran pas encore disponible' : 'Une erreur est survenue'}
        </AlertTitle>
        <AlertDescription className="space-y-3">
          <p>
            {isNotImplemented
              ? "Cet écran attend un endpoint que l'API n'expose pas encore."
              : error.message}
          </p>
          <Button variant="outline" size="sm" onClick={reset}>
            Réessayer
          </Button>
        </AlertDescription>
      </Alert>
    </div>
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
