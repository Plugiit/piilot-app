import { useEffect, useRef } from 'react'

/**
 * Le focus d'une confirmation en ligne : il entre dans le panneau a l'ouverture
 * et revient aux gestes a la fermeture. Sans ca, le bouton clique disparait et
 * le clavier repart du haut de la page.
 */
export function useConfirmFocus(open: boolean) {
  const panelRef = useRef<HTMLDivElement>(null)
  const actionsRef = useRef<HTMLDivElement>(null)
  const wasOpen = useRef(false)

  useEffect(() => {
    if (open) panelRef.current?.focus()
    else if (wasOpen.current) actionsRef.current?.querySelector('button')?.focus()
    wasOpen.current = open
  }, [open])

  return { panelRef, actionsRef }
}
