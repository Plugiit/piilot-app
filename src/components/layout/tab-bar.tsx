import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { Link, useMatchRoute } from '@tanstack/react-router'
import { motion } from 'framer-motion'

import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'

export interface Tab {
  to: string
  label: string
  icon: IconSvgElement
}

/**
 * Barre d'onglets d'un ecran.
 *
 * Des liens et non un etat local : un onglet est une adresse. On partage « le
 * kanban du portail client », on y revient par le bouton Retour, on la met en
 * favori — trois choses qu'un `useState` ne sait pas faire.
 *
 * Le filet actif est un noeud unique porte par `layoutId` : Framer le glisse
 * d'un onglet a l'autre au lieu de l'effacer ici pour le repeindre la. D'ou le
 * `layoutId` demande a l'appelant : deux barres qui partageraient le meme nom
 * verraient leur filet sauter de l'une a l'autre.
 */
export function TabBar({
  tabs,
  params = {},
  layoutId,
  keepSearch = false,
}: {
  tabs: Tab[]
  /** Vide par defaut : tous les ecrans a onglets n'ont pas de parametre d'adresse. */
  params?: Record<string, string>
  layoutId: string
  /**
   * Reporte les parametres de recherche d'un onglet a l'autre.
   *
   * Sur un ecran filtre, changer de vue ne doit pas remettre la barre d'outils
   * a zero — on regarde le meme jeu de donnees autrement. Optionnel parce que
   * les routes qui ne declarent pas ces parametres les refuseraient.
   */
  keepSearch?: boolean
}) {
  const matchRoute = useMatchRoute()
  const transition = useSlideTransition()

  return (
    <nav className="flex items-center">
      {tabs.map((tab) => {
        const active = matchRoute({ to: tab.to, params }) !== false

        return (
          <Link
            key={tab.to}
            to={tab.to}
            params={params}
            search={keepSearch ? (prev) => prev : undefined}
            className="relative flex shrink-0 items-center gap-2 px-3.5 py-2"
          >
            <HugeiconsIcon
              icon={tab.icon}
              size={20}
              strokeWidth={1.6}
              className={cn('shrink-0 transition-colors', active ? 'text-brand' : 'text-[#73757c]')}
            />
            <span
              className={cn(
                'text-[16px] whitespace-nowrap transition-colors',
                active ? 'font-medium text-brand' : 'text-[#73757c]',
              )}
            >
              {tab.label}
            </span>

            {active && (
              <motion.span
                aria-hidden
                layoutId={layoutId}
                transition={transition}
                className="absolute right-0 -bottom-px left-0 h-[2px] bg-brand"
              />
            )}
          </Link>
        )
      })}
    </nav>
  )
}
