import { useSyncExternalStore } from 'react'

import { HttpError } from '@/lib/api'

/**
 * Etat « application en maintenance », partage par toute l'application.
 *
 * La passerelle repond 503 MAINTENANCE quand aucune instance du serveur ne
 * repond depuis plusieurs secondes. Il suffit qu'une requete le recoive pour
 * que l'ecran de maintenance couvre la page : les autres echoueraient de la
 * meme facon, chacune avec son message.
 */
let active = false
const listeners = new Set<() => void>()

export function reportMaintenance(error: unknown) {
  if (active || !(error instanceof HttpError) || error.code !== 'MAINTENANCE') return

  active = true
  listeners.forEach((listener) => listener())
}

export function useMaintenance(): boolean {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    () => active,
  )
}
