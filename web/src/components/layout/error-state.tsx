import type { ErrorComponentProps } from '@tanstack/react-router'
import { AlertTriangle } from 'lucide-react'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { HttpError } from '@/lib/api'

/**
 * Etat d'erreur d'une vue.
 *
 * Monte comme `errorComponent` des routes qui portent un loader, et non
 * seulement a la racine : l'erreur remplace le composant de la route ou elle
 * est declaree, donc l'attacher a la feuille la contient dans l'`Outlet` du
 * shell — la navigation reste a l'ecran et reste cliquable. A la racine, elle
 * effacerait toute la page.
 *
 * Les endpoints metier repondent encore 501, donc ce cas n'a rien
 * d'hypothetique aujourd'hui.
 */
export function ErrorState({ error, reset }: ErrorComponentProps) {
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
