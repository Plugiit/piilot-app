import { useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'

import { HttpError } from '@/lib/api'
import type { RgpdErasureItem } from '@/types/api'

import { useCancelErasure, useRequestExport } from './api'
import { rgpdErrorMessage, type SubjectType } from './labels'

export interface RgpdSubject {
  type: SubjectType
  id: string
  /** Nom à retaper pour confirmer : celui que l'écran affiche pour le sujet. */
  name: string
}

export function reportError(error: unknown, fallback: string) {
  toast.error(error instanceof HttpError ? rgpdErrorMessage(error.code, error.message) : fallback)
}

/** Demande d'export : l'archive se construit en tâche de fond, la liste des exports la montre. */
export function useExportAction(subject: RgpdSubject) {
  const request = useRequestExport()
  const navigate = useNavigate()

  return {
    pending: request.isPending,
    run: () =>
      request.mutate(subject, {
        onSuccess: () =>
          toast.success('Export demandé. L’archive sera prête dans une minute environ.', {
            action: {
              label: 'Voir',
              onClick: () => void navigate({ to: '/parametres/rgpd', search: { tab: 'exports', page: 1 } }),
            },
          }),
        onError: (e) => reportError(e, 'Export impossible'),
      }),
  }
}

/** Annule un effacement planifié, avec le retour d'usage. */
export function useCancelAction() {
  const cancel = useCancelErasure()

  return {
    pending: cancel.isPending,
    run: (erasure: RgpdErasureItem) =>
      cancel.mutate(erasure.id, {
        onSuccess: () => toast.success('Suppression annulée'),
        onError: (e) => toast.error(e instanceof HttpError ? rgpdErrorMessage(e.code, e.message) : 'Annulation impossible'),
      }),
  }
}

