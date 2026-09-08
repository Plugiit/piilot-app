import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

/**
 * Chassis des cartes du tableau de bord.
 *
 * Trois cartes portaient le meme empilement — degrade, filet, en-tete en
 * capitales, carte blanche interieure — recopie a l'identique. Le tenir ici
 * evite qu'elles ne divergent au premier ajustement.
 *
 * `@container` vit sur la carte et non sur son contenu : les graphiques
 * doivent regler leurs ruptures sur la largeur qu'on leur donne, pas sur celle
 * de la fenetre. Le tableau de bord les met cote a cote.
 *
 * Vit hors de `components/ui/`, reserve aux composants shadcn — voir la note
 * de `stat-card.tsx`.
 */
export function DashboardCard({
  icon,
  title,
  action,
  className,
  children,
}: {
  icon: IconSvgElement
  title: string
  /** Coin haut droit de l'en-tete : une infobulle d'aide, un lien. */
  action?: ReactNode
  className?: string
  children: ReactNode
}) {
  return (
    <section
      className={cn(
        '@container flex flex-col rounded-[12px] border border-[#ebebeb] bg-[#f8f8f8] px-2 pt-3 pb-2',
        className,
      )}
    >
      <header className="flex items-center gap-1 px-2 pb-3">
        <HugeiconsIcon
          icon={icon}
          size={16}
          strokeWidth={1.6}
          className="shrink-0 text-[#606060]"
        />

        <h2 className="text-[14px] leading-[1.5] font-medium tracking-[0.56px] text-[#606060]">
          {title}
        </h2>

        {action}
      </header>

      {/* Rayon interne = rayon du parent moins son padding, pour que les deux
          arrondis restent concentriques.

          `flex-1` la fait descendre jusqu'au bas de la carte : mises en
          grille, deux cartes prennent la hauteur de la plus haute, et sans
          cela la blanche de la plus courte s'arreterait au milieu du
          degrade. */}
      <div className="flex flex-1 flex-col justify-between rounded-[4px] border border-[#ebebeb] bg-white p-3">
        {children}
      </div>
    </section>
  )
}
