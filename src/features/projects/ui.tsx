import type { IconSvgElement } from '@hugeicons/react'
import type { ReactNode } from 'react'

import { DashboardCard } from '@/components/dashboard-card'
import { cn } from '@/lib/utils'

/**
 * Vocabulaire visuel partage par les ecrans d'un projet.
 *
 * La jauge et la tuile vivaient dans la page de detail, ou elles etaient
 * seules a servir. Elles servent maintenant a l'en-tete du projet comme a ses
 * onglets : les tenir ici evite que la meme jauge ne prenne deux epaisseurs
 * selon l'ecran qui la dessine.
 */

/**
 * Couleurs des jauges du module.
 *
 * Le temps consomme etait bleu — une couleur qui n'existe nulle part ailleurs
 * dans le module, et qui tirait l'oeil sur la donnee la plus banale de la
 * rangee. Il passe a l'encre du texte : une jauge n'a pas a etre coloree pour
 * etre lue, la couleur est reservee a ce qui alerte.
 *
 * L'avancement, lui, garde l'orange de marque et vire au vert une fois
 * complet — c'est la seule jauge dont l'etat plein veut dire quelque chose.
 */
export const BILLABLE_COLOR = '#111'
export const PROGRESS_COLOR = '#ff782b'
export const DONE_COLOR = '#0db471'
export const WARN_COLOR = '#ff782b'
export const ALERT_COLOR = '#e5484d'

/** Jauge fine : une part, jamais un axe gradue. */
export function Meter({
  ratio,
  color,
  className,
}: {
  ratio: number
  color: string
  className?: string
}) {
  return (
    <div
      aria-hidden
      className={cn('h-1 w-full overflow-hidden rounded-full bg-[#ebebeb]', className)}
    >
      <div
        className="h-full rounded-full"
        style={{ width: `${Math.min(100, Math.max(0, ratio))}%`, backgroundColor: color }}
      />
    </div>
  )
}

/**
 * Chiffre-cle de l'en-tete d'un projet.
 *
 * Elle se dessinait a part : un cadre gris, un intitule en bas de casse, pas
 * d'icone, pas de carte blanche. Posee au-dessus des blocs JALONS et
 * LIVRABLES, elle ne ressemblait a rien d'autre dans l'application.
 *
 * Elle prend donc le chassis commun — creux gris, icone, intitule en
 * capitales, carte blanche filetee posee dedans — et n'ajoute que ce qui la
 * distingue : un chiffre, sa jauge, sa mention.
 */
export function Tile({
  icon,
  label,
  value,
  hint,
  tone,
  children,
}: {
  icon: IconSvgElement
  label: string
  value: string
  hint?: string
  tone?: 'alert'
  children?: ReactNode
}) {
  return (
    <DashboardCard icon={icon} title={label}>
      <div className="flex flex-1 flex-col gap-1.5">
        <p
          className={cn(
            'text-[20px] leading-[1.3] font-semibold tabular-nums',
            tone === 'alert' ? 'text-[#e5484d]' : 'text-[#111]',
          )}
        >
          {value}
        </p>

        {/* Le pied est colle en bas et ses deux lignes ont une hauteur fixe,
            occupees ou non : c'est ce qui aligne les jauges entre elles et les
            mentions entre elles d'une tuile a l'autre. Sans cette reserve, une
            tuile sans jauge remonte sa mention d'un cran et la rangee ondule. */}
        <div className="mt-auto flex flex-col gap-1.5 pt-1">
          <div className="flex h-1 items-center">{children}</div>
          <p className="h-[18px] text-[12px] leading-[1.5] text-[#8d8d8d]">{hint}</p>
        </div>
      </div>
    </DashboardCard>
  )
}
