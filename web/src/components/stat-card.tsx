import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

/**
 * Carte de chiffre-cle, celle du tableau de bord.
 *
 * Deux etages dans un meme cadre : la valeur sur une carte blanche, la mention
 * dans le creux gris en dessous. Partagee par le tableau de bord et « Mon
 * travail », pour que les chiffres se lisent de la meme facon partout.
 *
 * Vit hors de `components/ui/`, reserve aux composants shadcn.
 */
export function StatCard({
  label,
  value,
  footer,
  tone,
  chart,
  className,
}: {
  label: string
  value: ReactNode
  /** Le creux du bas : une mention, une evolution. */
  footer: ReactNode
  /** `alert` passe la valeur en rouge : un retard, un depassement. */
  tone?: 'alert'
  /**
   * Un graphique sous la valeur, pose au ras du bas de la carte blanche : il
   * prend la place du rembourrage, et son zero tombe sur le bord.
   */
  chart?: ReactNode
  className?: string
}) {
  return (
    <div
      className={cn(
        'border-surface-sunken bg-surface flex min-w-px flex-1 flex-col gap-0.5 overflow-clip rounded-[12px] border p-0.5',
        className,
      )}
    >
      <div className={cn('flex w-full flex-col gap-1 overflow-hidden rounded-[10px] bg-white p-2', chart !== undefined && 'pb-0')}>
        <p className="w-full text-sm leading-[1.5] text-[#111]">{label}</p>
        <p
          className={cn(
            'w-full text-xl leading-[1.4] font-semibold tabular-nums',
            tone === 'alert' ? 'text-[#e5484d]' : 'text-[#111]',
          )}
        >
          {value}
        </p>
        {chart !== undefined && <div className="-mx-2 mt-1 h-[40px]">{chart}</div>}
      </div>

      <div className="bg-surface flex w-full items-center justify-center gap-2.5 px-2 py-1.5">{footer}</div>
    </div>
  )
}
