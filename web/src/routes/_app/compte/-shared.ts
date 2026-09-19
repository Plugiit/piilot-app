import { toast } from 'sonner'

import { HttpError } from '@/lib/api'

/** Signale l'echec d'une ecriture. Le succes, lui, se voit a l'ecran. */
export function reportError(error: unknown) {
  toast.error(error instanceof HttpError ? error.message : 'Une erreur est survenue')
}
