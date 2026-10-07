import { useEffect, useSyncExternalStore } from 'react'

/**
 * Etat de la palette Cmd+K, partage par la coquille (qui l'ouvre depuis le
 * champ du panneau) et le composant lui-meme (ouvert au clavier).
 */
let open = false
const listeners = new Set<() => void>()

function emit() {
  listeners.forEach((listener) => listener())
}

export function openPalette() {
  if (open) return
  open = true
  emit()
}

export function setPaletteOpen(next: boolean) {
  if (open === next) return
  open = next
  emit()
}

export function usePaletteOpen(): boolean {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    () => open,
  )
}

/**
 * Intention de creation, deposee par la palette avant de naviguer.
 *
 * Une commande « Nouveau projet » mene a la liste des projets puis ouvre sa
 * fenetre de creation. La fenetre n'existe pas encore au moment de la
 * commande : l'intention l'attend, et la fenetre la consomme a son montage.
 * Une intention qui ne trouve personne s'efface a la navigation suivante.
 */
export type CreateKind = 'project' | 'task' | 'client' | 'contact' | 'ticket' | 'interaction'

let pending: CreateKind | null = null
const intentListeners = new Set<() => void>()

export function requestCreate(kind: CreateKind) {
  pending = kind
  intentListeners.forEach((listener) => listener())
}

export function clearCreateIntent() {
  pending = null
}

/**
 * Ouvre la fenetre quand l'intention la designe. `onOpen` est appele au plus
 * une fois par intention.
 */
export function useCreateIntent(kind: CreateKind, onOpen: () => void) {
  const current = useSyncExternalStore(
    (listener) => {
      intentListeners.add(listener)
      return () => intentListeners.delete(listener)
    },
    () => pending,
  )

  useEffect(() => {
    if (current !== kind) return
    pending = null
    onOpen()
    // `onOpen` est un setState : stable, pas besoin de le suivre.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current, kind])
}
