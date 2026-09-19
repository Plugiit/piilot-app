import { createContext, useContext } from 'react'

/**
 * Etat du panneau lateral d'un ecran.
 *
 * Le bouton qui le replie vit dans l'en-tete, porte par `PageFrame` ; le
 * panneau lui-meme vit dans la page, qui seule sait ou le poser et comment il
 * se comporte aux differentes largeurs. Ce contexte est le fil entre les deux,
 * et evite d'avoir a remonter le panneau entier dans la frame pour un bouton.
 *
 * Dans un fichier a part : un module qui exporte autre chose que des
 * composants perd le rechargement a chaud.
 */
export interface AsideState {
  open: boolean
  toggle: () => void
}

export const AsideContext = createContext<AsideState | null>(null)

/**
 * Lit l'etat du panneau lateral.
 *
 * Renvoie `null` hors d'une page qui en declare un : une page sans panneau ne
 * doit pas avoir a en simuler un pour s'afficher.
 */
export function usePageAside(): AsideState | null {
  return useContext(AsideContext)
}
