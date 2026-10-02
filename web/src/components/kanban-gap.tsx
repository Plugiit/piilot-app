import { motion, useReducedMotion } from 'framer-motion'
import { useEffect, useState } from 'react'

/**
 * Ressort des colonnes de kanban : l'emplacement qui s'ouvre sous la carte
 * portee, et les cartes voisines qui se poussent pour lui faire place.
 *
 * Un ressort sans rebond plutot qu'une duree : une colonne qui s'allonge de
 * 80px ou de 200px garde la meme allure, et ne depasse pas sa hauteur finale
 * — un rebond ferait sautiller toute la colonne a chaque survol.
 */
export function useColumnTransition() {
  const reduced = useReducedMotion()

  return reduced ? { duration: 0 } : ({ type: 'spring', visualDuration: 0.28, bounce: 0 } as const)
}

/**
 * Emplacement de depot en bas d'une colonne.
 *
 * Il prend la hauteur de la carte portee : la colonne s'allonge exactement de
 * ce qu'elle va recevoir, et la carte, une fois deposee, prend la place sans
 * que rien ne bouge. Toujours monte, a hauteur nulle quand il est ferme :
 * c'est ce qui laisse Framer interpoler l'ouverture comme la fermeture.
 *
 * `instant` le referme sans animation au moment ou la carte arrive. L'animer
 * a cet instant ferait grandir la colonne de la carte puis retrecir de
 * l'emplacement : un a-coup de la hauteur d'une carte.
 */
export function DropGap({
  open,
  height,
  instant,
  spacing,
  className,
}: {
  open: boolean
  height: number
  instant: boolean
  /**
   * Ecart de la liste qui le porte (`gap` du flex), en pixels. Ferme,
   * l'emplacement le reprend par une marge negative : sans elle, chaque
   * colonne garderait cet ecart vide sous sa derniere carte.
   */
  spacing: number
  className?: string
}) {
  const transition = useColumnTransition()

  return (
    <motion.div
      aria-hidden
      initial={false}
      animate={{
        height: open ? height : 0,
        marginTop: open ? 0 : -spacing,
        opacity: open ? 1 : 0,
      }}
      transition={instant ? { duration: 0 } : transition}
      className="shrink-0 overflow-hidden"
    >
      <div
        className={
          className ??
          'border-brand/40 bg-brand/5 h-full rounded-[10px] border-2 border-dashed'
        }
      />
    </motion.div>
  )
}

/**
 * Carte deposee, en attente d'arriver dans sa nouvelle colonne.
 *
 * Entre le relachement et l'arrivee de la carte, il y a la mise a jour du
 * cache — immediate quand elle est optimiste, un aller-retour au serveur
 * sinon. Pendant ce temps, l'emplacement doit rester ouvert : referme tout de
 * suite, la colonne se tasserait puis se rallongerait a l'arrivee.
 *
 * `arrived` dit si la carte est la. L'arrivee est retenue une fois pour toutes
 * (`landed`) : si la carte repart ensuite par le menu, l'emplacement ne doit
 * pas se rouvrir pour l'attendre. Le delai de secours le referme si elle
 * n'arrive jamais — l'ecriture a echoue et la carte est revenue a sa place.
 */
export function useLanding<Column>(arrived: (landing: { id: string; column: Column }) => boolean) {
  const [landing, setLanding] = useState<{
    id: string
    column: Column
    height: number
    landed: boolean
  } | null>(null)

  // Retenu pendant le rendu plutot que dans un effet : c'est le rendu meme ou
  // la carte apparait qui doit refermer l'emplacement, sans attendre le tour
  // suivant.
  if (landing !== null && !landing.landed && arrived(landing)) {
    setLanding({ ...landing, landed: true })
  }

  const pending = landing !== null && !landing.landed

  useEffect(() => {
    if (!pending) return

    const timer = window.setTimeout(() => setLanding(null), 3000)

    return () => window.clearTimeout(timer)
  }, [pending])

  return {
    /** Colonne qui attend une carte, tant qu'elle n'est pas arrivee. */
    awaiting: pending ? landing.column : null,
    /** Colonne ou la carte vient d'arriver : son emplacement se ferme sans animation. */
    landedIn: landing?.landed === true ? landing.column : null,
    height: landing?.height ?? 0,
    land: (id: string, column: Column, height: number) =>
      setLanding({ id, column, height, landed: false }),
    /** Une nouvelle carte est saisie : l'arrivee precedente est oubliee. */
    reset: () => setLanding(null),
  }
}
