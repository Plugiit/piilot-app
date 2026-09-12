import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

/**
 * Chassis de carte du tableau de bord.
 *
 * Une coque grise de deux pixels, l'intitule pose dedans, et la carte blanche
 * filetee ou vit le contenu. C'est le creux qui detache la carte du fond, pas
 * une ombre : a plat sur un fond de la meme famille, elle disparaitrait.
 *
 * Remplace progressivement `DashboardCard`, dont le chassis est plus epais
 * (rembourrage de 8px, carte interieure a 4px de rayon). Les deux coexistent
 * le temps que les cartes migrent une a une ; a la derniere, `DashboardCard`
 * disparait.
 *
 * `@container` vit sur la coque et non sur son contenu : les graphiques
 * doivent regler leurs ruptures sur la largeur qu'on leur donne, pas sur celle
 * de la fenetre.
 */
export function PanelCard({
  icon,
  title,
  action,
  className,
  contentClassName,
  children,
}: {
  icon: IconSvgElement
  title: string
  /** Coin oppose de l'en-tete : une infobulle d'aide, un lien. */
  action?: ReactNode
  className?: string
  /** Pour le contenu qui a besoin de sa propre disposition. */
  contentClassName?: string
  children: ReactNode
}) {
  return (
    <section
      className={cn(
        '@container border-surface-sunken bg-surface flex flex-col overflow-clip rounded-[12px] border p-0.5',
        className,
      )}
    >
      <header className="flex w-full items-center gap-1 px-2 py-1.5">
        <HugeiconsIcon
          icon={icon}
          size={16}
          strokeWidth={1.6}
          className="shrink-0 text-[#606060]"
        />

        <h2 className="text-[14px] leading-[1.5] font-medium tracking-[0.56px] text-[#606060]">
          {title}
        </h2>

        {/* Pousse au bord oppose : l'en-tete separe ce qui nomme la carte de
            ce qui l'explique ou l'actionne. */}
        {action !== undefined && <div className="ml-auto flex shrink-0 items-center">{action}</div>}
      </header>

      {/* `flex-1` la fait descendre jusqu'au bas de la coque : mises en grille,
          deux cartes prennent la hauteur de la plus haute, et sans cela la
          blanche de la plus courte s'arreterait au milieu du gris. */}
      <div
        className={cn(
          'flex flex-1 flex-col rounded-[8px] border border-[#ebebeb] bg-white p-2',
          contentClassName,
        )}
      >
        {children}
      </div>
    </section>
  )
}
