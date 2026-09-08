import { useReducedMotion } from 'framer-motion'

/**
 * Transition des pastilles qui glissent d'un onglet a l'autre.
 *
 * Partagee par le rail de modules, les onglets du graphique de charge et les
 * listes de l'agenda : trois endroits ou un ressort different se remarquerait
 * aussitot, l'oeil comparant deux mouvements voisins bien mieux que deux
 * couleurs.
 *
 * Un ressort plutot qu'une duree : la pastille garde la meme allure quelle que
 * soit la distance parcourue, d'un voisin immediat aux deux bouts de la
 * colonne. `bounce: 0` lui evite de depasser sa cible.
 *
 * `duration: 0` plutot que pas d'animation du tout quand le systeme demande
 * a reduire les mouvements : Framer Motion applique alors l'etat final sans
 * l'interpoler, et rien n'a besoin d'etre ecrit en double.
 */
export function useSlideTransition() {
  const reduced = useReducedMotion()

  return reduced ? { duration: 0 } : ({ type: 'spring', visualDuration: 0.25, bounce: 0 } as const)
}
