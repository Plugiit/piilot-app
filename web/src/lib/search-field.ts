import { useEffect, useState } from 'react'

/**
 * Champ de recherche relie a l'adresse, mais pas a chaque frappe.
 *
 * Les quatre barres de recherche du back-office ecrivaient dans l'URL depuis
 * `onChange`. Taper « Maison Aubert » posait donc treize entrees d'historique
 * et treize appels serveur, et il fallait ensuite treize pressions sur Retour
 * pour quitter l'ecran. Sur « Tâches », c'etait pire : la route y attend sa
 * donnee dans un loader indexe sur les filtres, donc chaque caractere bloquait
 * la navigation sur un aller-retour reseau.
 *
 * Ce hook separe les deux rythmes : `draft` suit la frappe et rend le champ
 * immediat, `commit` n'est appele qu'apres une pause. Les appelants posent
 * `replace: true` dans leur navigation, pour que la recherche remplace son
 * entree au lieu d'en empiler une nouvelle.
 *
 * @param committed Valeur portee par l'adresse.
 * @param commit    Ecrit la valeur dans l'adresse. Peut etre recree a chaque
 *                  rendu : l'effet ne depend pas de son identite.
 */
export function useSearchField(
  committed: string,
  commit: (value: string) => void,
  delay = 300,
): [string, (value: string) => void] {
  const [draft, setDraft] = useState(committed)

  // L'adresse a pu changer ailleurs : retour arriere, filtre efface depuis un
  // menu, arrivee sur l'ecran avec une recherche deja posee. Le champ suit,
  // sinon il afficherait une recherche qui n'est plus celle de la liste.
  // C'est le motif React d'ajustement d'etat pendant le rendu, et non un
  // effet : un effet provoquerait un second rendu visible.
  const [seen, setSeen] = useState(committed)

  if (committed !== seen) {
    setSeen(committed)
    setDraft(committed)
  }

  useEffect(() => {
    // Rien a ecrire : la frappe est revenue d'elle-meme sur la valeur de
    // l'adresse, ou c'est l'adresse qui vient de fixer la frappe.
    if (draft === committed) return

    const timer = window.setTimeout(() => commit(draft), delay)

    return () => window.clearTimeout(timer)
    // `commit` est volontairement hors des dependances : les appelants le
    // recreent a chaque rendu, et l'y mettre relancerait le minuteur sans fin.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draft, committed, delay])

  return [draft, setDraft]
}
