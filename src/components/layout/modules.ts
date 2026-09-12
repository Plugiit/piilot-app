import {
  Building03Icon,
  Calendar03Icon,
  CheckmarkSquare02Icon,
  Clock01Icon,
  Contact01Icon,
  DeliveryBox01Icon,
  Folder01Icon,
  Home03Icon,
  LayoutTable01Icon,
  Message01Icon,
  Tag01Icon,
  Ticket02Icon,
  UserMultipleIcon,
} from '@hugeicons/core-free-icons'
import type { IconSvgElement } from '@hugeicons/react'
import { useRouterState } from '@tanstack/react-router'

/** Feuille d'un menu deroulant : toujours une destination. */
export interface MenuLeaf {
  label: string
  to: string
}

/**
 * Entree de menu : soit elle mene quelque part (`to`), soit elle deplie des
 * sous-entrees (`children`). Jamais les deux — un parent cliquable qui deplie
 * aussi laisse l'utilisateur sans moyen de faire l'un sans l'autre.
 */
export type MenuItem = { icon: IconSvgElement; label: string } & (
  { to: string; children?: never } | { to?: never; children: MenuLeaf[] }
)

export interface MenuGroup {
  label: string
  items: MenuItem[]
}

export interface AppModule {
  icon: IconSvgElement
  /** Nom court, celui du rail et de la racine du fil d'Ariane. */
  label: string
  to: string
  /** Navigation du panneau, propre au module. */
  menu: MenuGroup[]
}

/**
 * Modules de l'application et leur navigation.
 *
 * Une seule liste, lue par le rail (ses marques), le panneau (son titre et son
 * menu) et le fil d'Ariane (sa racine) : les trois ne peuvent pas diverger.
 *
 * Les ecrans marques « 2e lot » ont leur entree mais pas encore leur contenu :
 * un modele de projet ne vaut que quand on sait a quoi ressemble un projet, et
 * le catalogue de services attend d'etre cadre — il reste volontairement a
 * l'ecart de tout montant, la facturation n'est pas de ce projet.
 */
export const MODULES: AppModule[] = [
  {
    icon: Folder01Icon,
    label: 'Gestion de projet',
    to: '/pm',
    menu: [
      {
        label: 'Pilotage',
        items: [
          { icon: Home03Icon, label: 'Tableau de bord', to: '/pm' },
          { icon: Folder01Icon, label: 'Projets', to: '/pm/projets' },
          { icon: Calendar03Icon, label: 'Planning', to: '/pm/planning' },
        ],
      },
      {
        label: 'Production',
        items: [
          { icon: CheckmarkSquare02Icon, label: 'Tâches', to: '/pm/taches' },
          {
            icon: Clock01Icon,
            label: 'Suivi du temps',
            // Saisir son temps et l'analyser sont deux ecrans distincts : un
            // formulaire, puis un tableau d'agregats.
            children: [
              { label: 'Saisie', to: '/pm/temps/saisie' },
              { label: 'Rapports', to: '/pm/temps/rapports' },
            ],
          },
          { icon: DeliveryBox01Icon, label: 'Livrables', to: '/pm/livrables' },
          { icon: Ticket02Icon, label: 'Tickets', to: '/pm/tickets' },
        ],
      },
      {
        label: 'Référentiels',
        items: [
          { icon: LayoutTable01Icon, label: 'Modèles de projet', to: '/pm/modeles' },
          { icon: Tag01Icon, label: 'Services', to: '/pm/services' },
        ],
      },
    ],
  },
  {
    icon: UserMultipleIcon,
    label: 'CRM',
    to: '/crm',
    menu: [
      {
        label: 'Général',
        items: [
          { icon: Building03Icon, label: 'Clients', to: '/crm/clients' },
          { icon: Contact01Icon, label: 'Contacts', to: '/crm/contacts' },
          { icon: Message01Icon, label: 'Interactions', to: '/crm/interactions' },
        ],
      },
    ],
  },
]

/**
 * Module ouvert, deduit de l'URL.
 *
 * Les sous-routes gardent leur module actif : /pm/xyz reste Project Management.
 */
export function useActiveModule(): AppModule | undefined {
  const pathname = useRouterState({ select: (state) => state.location.pathname })

  return MODULES.find((module) => pathname === module.to || pathname.startsWith(`${module.to}/`))
}

/** Destinations d'une entree : la sienne, ou celles de ses sous-entrees. */
export function destinationsOf(item: MenuItem): string[] {
  return item.children ? item.children.map((leaf) => leaf.to) : [item.to]
}

/** Vrai quand `to` designe cette adresse ou l'une de ses sous-pages. */
function covers(to: string, pathname: string): boolean {
  return pathname === to || pathname.startsWith(`${to}/`)
}

/**
 * Destination du menu a laquelle appartient l'adresse courante.
 *
 * Une entree reste allumee sur ses sous-pages : la fiche d'un projet est
 * encore « Projets ». Mais plusieurs entrees peuvent prefixer la meme adresse
 * — /pm/projets/42 est couvert par « Tableau de bord » (/pm) autant que par
 * « Projets » (/pm/projets). C'est la plus longue qui gagne, sans quoi le
 * tableau de bord resterait actif partout dans son module.
 *
 * Rendre la destination plutot qu'un booleen par entree laisse un seul
 * vainqueur : deux entrees ne peuvent pas s'allumer ensemble.
 */
export function activeDestination(
  module: AppModule | undefined,
  pathname: string,
): string | undefined {
  if (module === undefined) return undefined

  let best: string | undefined

  for (const group of module.menu) {
    for (const item of group.items) {
      for (const to of destinationsOf(item)) {
        if (covers(to, pathname) && (best === undefined || to.length > best.length)) {
          best = to
        }
      }
    }
  }

  return best
}
