import { CheckListIcon, Menu01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'

import { NotificationBell } from '@/features/notifications/bell'
import { Link, useRouterState } from '@tanstack/react-router'
import { Fragment, useState, type ReactNode } from 'react'

import { useSidebar } from '@/components/layout/admin-shell'
import { AsideContext, type AsideState } from '@/components/layout/aside-context'
import { useActiveModule } from '@/components/layout/modules'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { cn } from '@/lib/utils'

/**
 * Maillon intermediaire du fil d'Ariane.
 *
 * Entre la racine du module et l'ecran courant. Toujours un lien : un maillon
 * sur lequel on ne peut pas revenir n'apprend qu'ou l'on est, alors qu'un fil
 * sert d'abord a remonter.
 */
export interface Crumb {
  label: string
  to: string
  params?: Record<string, string>
  search?: Record<string, unknown>
}

/**
 * En-tete de la frame : fil d'Ariane a gauche, actions transverses a droite.
 *
 * Les deux boutons ne sont pas cables : la maquette les pose, les ecrans
 * qu'ils ouvrent n'existent pas encore.
 */
function FrameHeader({
  title,
  trail,
  aside,
}: {
  title: string
  /** Maillons entre la racine du module et l'ecran courant. */
  trail: Crumb[]
  /** Present quand l'ecran porte un panneau lateral repliable. */
  aside?: AsideState
}) {
  // Le module ouvert forme la racine du fil : il n'a pas a etre repete dans
  // chaque ecran, l'URL le dit deja.
  const activeModule = useActiveModule()
  const { openDrawer } = useSidebar()

  return (
    <header className="flex shrink-0 items-center justify-between gap-2 border-b border-[#ebebeb] p-3">
      {/* Sous 1024px le panneau n'est plus a l'ecran : sans ce bouton, les
          ecrans du module deviendraient injoignables. */}
      <button
        type="button"
        onClick={openDrawer}
        aria-label="Ouvrir la navigation"
        className="border-surface-sunken flex shrink-0 items-center justify-center rounded-[12px] border bg-white p-2.5 lg:hidden"
      >
        <HugeiconsIcon icon={Menu01Icon} size={20} strokeWidth={1.6} className="text-[#111]" />
      </button>

      <div className="flex min-w-0 flex-1 flex-col items-start justify-center sm:h-12">
        <Breadcrumb>
          <BreadcrumbList className="flex-nowrap gap-1.5 text-base sm:gap-1.5">
            {activeModule && (
              <>
                <BreadcrumbItem className="hidden shrink-0 whitespace-nowrap sm:inline-flex">
                  <BreadcrumbLink asChild className="text-[#777] hover:text-[#111]">
                    {/* Sans `exact`, la racine du fil se declarerait page
                        courante sur chacune de ses sous-routes. */}
                    <Link to={activeModule.to} activeOptions={{ exact: true }}>
                      {activeModule.label}
                    </Link>
                  </BreadcrumbLink>
                </BreadcrumbItem>
                <BreadcrumbSeparator className="hidden text-[#c4c4c4] sm:block [&>svg]:size-3" />
              </>
            )}

            {/* Les maillons intermediaires suivent le meme sort que la racine :
                masques sous `sm`, ou un fil de quatre niveaux ne tiendrait pas
                et ou seul l'ecran courant compte encore. */}
            {trail.map((crumb) => (
              <Fragment key={`${crumb.to}-${crumb.label}`}>
                <BreadcrumbItem className="hidden max-w-[220px] shrink-0 sm:inline-flex">
                  <BreadcrumbLink asChild className="text-[#777] hover:text-[#111]">
                    <Link
                      to={crumb.to}
                      params={crumb.params}
                      search={crumb.search}
                      className="truncate"
                    >
                      {crumb.label}
                    </Link>
                  </BreadcrumbLink>
                </BreadcrumbItem>
                <BreadcrumbSeparator className="hidden text-[#c4c4c4] sm:block [&>svg]:size-3" />
              </Fragment>
            ))}

            <BreadcrumbItem className="min-w-0">
              {/* Dernier maillon : l'ecran courant. `BreadcrumbPage` porte
                  aria-current et n'est pas un lien — on ne navigue pas vers la
                  page ou l'on se trouve. */}
              <BreadcrumbPage className="font-heading truncate text-base font-medium text-[#111]">
                {title}
              </BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
      </div>

      <div className="flex shrink-0 items-center gap-3">
        <NotificationBell />

        {/* Ne s'affiche que sur les ecrans qui ont un panneau a replier, et
            qu'a partir de la largeur ou ce panneau est une colonne : plus bas
            il passe sous le contenu, et le replier n'y gagne rien.

            Un bouton absent vaut mieux qu'un bouton qui ne fait rien. */}
        {aside !== undefined && (
          <button
            type="button"
            aria-label={aside.open ? 'Masquer l’agenda' : 'Afficher l’agenda'}
            aria-pressed={aside.open}
            onClick={aside.toggle}
            className={cn(
              'hidden items-center justify-center rounded-[12px] border p-2.5 transition-colors xl:flex',
              aside.open
                ? 'border-[#ff782b] bg-[#ff782b] text-white'
                : 'border-surface-sunken bg-white text-[#111]',
            )}
          >
            <HugeiconsIcon icon={CheckListIcon} size={20} strokeWidth={1.6} />
          </button>
        )}
      </div>
    </header>
  )
}

/**
 * Surface d'une page du back-office.
 *
 * Elle se decolle du haut de la fenetre de 16 pixels et reste collee au bord
 * droit et au bas : d'ou le seul coin arrondi en haut a gauche, le seul qui
 * flotte sur le fond de l'espace. Les bordures droite et basse tombent hors
 * champ, elles ne sont posees que pour rester fidele au fichier de design.
 *
 * La hauteur est bornee et le debordement masque : c'est la zone de contenu,
 * sous l'en-tete, qui defile — sinon l'en-tete partirait avec un tableau long.
 */
/**
 * Etat replie du panneau lateral, retenu le temps de l'onglet.
 *
 * `sessionStorage` et non `localStorage` : replier l'agenda est une decision de
 * seance de travail, pas une preference durable — rouvrir le navigateur le
 * lendemain doit retrouver l'ecran complet. Et rien ne part au serveur : ou
 * l'on a range un panneau ne regarde pas la base.
 *
 * Chaque acces est garde : un navigateur en navigation privee ou une page
 * cloisonnee peut refuser le stockage, et un agenda replie ne vaut pas un
 * ecran blanc.
 */
function readAside(key: string): boolean {
  try {
    return window.sessionStorage.getItem(key) !== 'replie'
  } catch {
    return true
  }
}

function writeAside(key: string, open: boolean) {
  try {
    window.sessionStorage.setItem(key, open ? 'ouvert' : 'replie')
  } catch {
    // Le panneau reste utilisable, il ne se souviendra simplement pas.
  }
}

export function PageFrame({
  title,
  trail = [],
  hasAside = false,
  children,
}: {
  title: string
  /**
   * Maillons intermediaires du fil d'Ariane, de la racine du module vers
   * l'ecran courant. Vide pour un ecran pose directement sous son module.
   */
  trail?: Crumb[]
  /**
   * L'ecran porte un panneau lateral repliable. L'en-tete affiche alors son
   * bouton, et la page lit l'etat par `usePageAside`.
   */
  hasAside?: boolean
  children: ReactNode
}) {
  // La cle porte l'identifiant de route et non l'adresse : deux projets
  // different par leur URL mais partagent le meme ecran, et le panneau n'a pas
  // a se souvenir projet par projet.
  const routeId = useRouterState({ select: (state) => state.matches.at(-1)?.routeId ?? '/' })
  const storageKey = `aside:${routeId}`

  const [open, setOpen] = useState(() => readAside(storageKey))

  const aside: AsideState = {
    open,
    toggle: () => {
      const next = !open

      setOpen(next)
      writeAside(storageKey, next)
    },
  }

  return (
    <div className="md:mt-4 flex min-h-0 flex-1 flex-col overflow-hidden md:rounded-t-[20px] border border-[#ebebeb] bg-white md:rounded-tr-none">
      <FrameHeader
        title={title}
        trail={trail}
        aside={hasAside ? aside : undefined}
      />

      <AsideContext.Provider value={hasAside ? aside : null}>
        <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
      </AsideContext.Provider>
    </div>
  )
}
