import { MoreHorizontalIcon, Notification03Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { useActiveModule } from '@/components/layout/modules'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'

/**
 * En-tete de la frame : fil d'Ariane a gauche, actions transverses a droite.
 *
 * Les deux boutons ne sont pas cables : la maquette les pose, les ecrans
 * qu'ils ouvrent n'existent pas encore.
 */
function FrameHeader({ title, description }: { title: string; description?: ReactNode }) {
  // Le module ouvert forme la racine du fil : il n'a pas a etre repete dans
  // chaque ecran, l'URL le dit deja.
  const activeModule = useActiveModule()

  return (
    <header className="flex shrink-0 items-center justify-between border-b border-[#ebebeb] p-3">
      <div className="flex h-12 min-w-0 flex-col items-start">
        <Breadcrumb>
          <BreadcrumbList className="flex-nowrap gap-1.5 sm:gap-1.5">
            {activeModule && (
              <>
                <BreadcrumbItem>
                  <BreadcrumbLink asChild className="text-[#777] hover:text-[#111]">
                    {/* Sans `exact`, la racine du fil se declarerait page
                        courante sur chacune de ses sous-routes. */}
                    <Link to={activeModule.to} activeOptions={{ exact: true }}>
                      {activeModule.label}
                    </Link>
                  </BreadcrumbLink>
                </BreadcrumbItem>
                <BreadcrumbSeparator className="text-[#c4c4c4] [&>svg]:size-3" />
              </>
            )}

            <BreadcrumbItem className="min-w-0">
              {/* Dernier maillon : l'ecran courant. `BreadcrumbPage` porte
                  aria-current et n'est pas un lien — on ne navigue pas vers la
                  page ou l'on se trouve. */}
              <BreadcrumbPage className="font-heading truncate text-lg font-medium text-[#111]">
                {title}
              </BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>

        {description && <p className="truncate text-xs leading-[1.5] text-[#777]">{description}</p>}
      </div>

      <div className="flex shrink-0 items-center gap-3">
        <button
          type="button"
          aria-label="Notifications"
          className="relative flex size-10 items-center justify-center rounded-lg border border-[#ebebeb] bg-white shadow-[0_1px_2px_0_rgb(16_24_40/0.05)]"
        >
          <HugeiconsIcon
            icon={Notification03Icon}
            size={20}
            strokeWidth={1.6}
            className="text-[#111]"
          />
          {/* Pastille de la maquette : posee sur l'angle du glyphe, cerclee de
              blanc pour se detacher du trait de la cloche. */}
          <span className="absolute top-[9px] right-[10px] size-1.5 rounded-full bg-[#ff782b] ring-2 ring-white" />
        </button>

        <button
          type="button"
          aria-label="Plus d'actions"
          className="flex size-10 items-center justify-center rounded-lg border border-[#ebebeb] bg-white shadow-[0_1px_2px_0_rgb(16_24_40/0.05)]"
        >
          <HugeiconsIcon
            icon={MoreHorizontalIcon}
            size={20}
            strokeWidth={1.6}
            className="text-[#111]"
          />
        </button>
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
export function PageFrame({
  title,
  description,
  children,
}: {
  title: string
  description?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="mt-4 flex min-h-0 flex-1 flex-col overflow-hidden rounded-tl-[20px] border border-[#ebebeb] bg-white">
      <FrameHeader title={title} description={description} />
      <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
    </div>
  )
}
