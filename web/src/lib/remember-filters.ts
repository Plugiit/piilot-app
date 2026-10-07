import { useLocation, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'

/**
 * Memorise les filtres d'une liste, par ecran, dans le navigateur. A appeler
 * en tete du composant de l'ecran.
 *
 * On revient dix fois par jour sur la meme liste avec les memes filtres :
 * « mes projets en production », « les tickets ouverts de tel projet ». Sans
 * memoire, chaque retour les reposait. Ici, arriver sur l'ecran sans aucun
 * filtre dans l'adresse rejoue les derniers ; en poser ou en retirer un les
 * enregistre. Tout retirer efface la memoire, pour que l'ecran reste vide la
 * fois suivante.
 *
 * `omit` : ce qui n'est pas un filtre — la page, la tache ouverte dans le
 * tiroir — et ne doit ni se rejouer ni compter comme un filtre pose.
 *
 * `defaults` : les valeurs que le routeur pose de lui-meme — un tri par
 * defaut — et qui ne sont donc pas un choix. Une adresse qui ne porte que
 * celles-la est une adresse sans filtre.
 *
 * Par navigateur et par poste, sans rien envoyer au serveur : c'est une
 * commodite, pas une donnee.
 */
export function useRememberFilters(scope: string, omit: string[] = [], defaults: Record<string, unknown> = {}) {
  const location = useLocation()
  const navigate = useNavigate()
  const key = `piilot:filters:${scope}`
  const applied = useRef(false)

  const search = location.search as Record<string, unknown>
  const filters = Object.fromEntries(
    Object.entries(search).filter(
      ([name, value]) => !omit.includes(name) && value !== undefined && value !== '' && defaults[name] !== value,
    ),
  )
  const serialized = JSON.stringify(filters)

  // Premiere arrivee sans rien dans l'adresse : on rejoue les filtres d'avant.
  useEffect(() => {
    if (applied.current) return
    applied.current = true
    if (Object.keys(filters).length > 0) return

    let stored: Record<string, unknown> | null = null
    try {
      const raw = localStorage.getItem(key)
      stored = raw === null ? null : (JSON.parse(raw) as Record<string, unknown>)
    } catch {
      stored = null
    }
    if (stored === null || Object.keys(stored).length === 0) return

    void navigate({ to: location.pathname, search: { ...search, ...stored } as never, replace: true })
    // Uniquement au montage.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Chaque changement de filtre se memorise ; plus aucun filtre, on oublie.
  useEffect(() => {
    if (!applied.current) return
    try {
      if (Object.keys(filters).length === 0) localStorage.removeItem(key)
      else localStorage.setItem(key, serialized)
    } catch {
      // Stockage indisponible : la liste marche sans memoire.
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serialized, key])

}
