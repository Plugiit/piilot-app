import type { IconSvgElement } from '@hugeicons/react'
import type { ReactNode } from 'react'

import { PanelCard } from '@/components/panel-card'
import { formatNumber, formatVariation } from '@/features/analytics/format'
import type { StatisticsRow } from '@/types/api'

/**
 * Les pieces de la page « Statistiques », partagees avec le rapport
 * mensuel : la variation, le panneau, le classement.
 */

/** La variation : verte si elle va dans le bon sens, rouge sinon. */
export function Delta({ value, lowerIsBetter = false }: { value: number | null; lowerIsBetter?: boolean }) {
  if (value === null) return <span className="text-[#a2a3a7]">—</span>
  const good = lowerIsBetter ? value <= 0 : value >= 0
  return <span className={`shrink-0 font-medium tabular-nums ${good ? 'text-[#006f1f]' : 'text-[#e5484d]'}`}>{formatVariation(value)}</span>
}

export function Legend({ color, label }: { color: string; label: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <span aria-hidden className="h-[3px] w-3 rounded-full" style={{ backgroundColor: color }} />
      {label}
    </span>
  )
}

/** Un panneau au chassis des cartes de l'admin : coque grise, titre, carte blanche. */
export function Panel({
  icon,
  title,
  aside,
  children,
  contentClassName,
}: {
  icon: IconSvgElement
  title: string
  aside?: ReactNode
  children: ReactNode
  contentClassName?: string
}) {
  return (
    <PanelCard icon={icon} title={title} action={aside} className="break-inside-avoid" contentClassName={contentClassName}>
      {children}
    </PanelCard>
  )
}

/**
 * Un classement : la valeur, son compte, et une barre proportionnelle au
 * premier de la liste, posee sous le texte.
 */
export function TopList({
  icon,
  title,
  unit,
  rows,
  label,
}: {
  icon: IconSvgElement
  title: string
  unit: string
  rows: StatisticsRow[]
  label: (v: string) => ReactNode
}) {
  const max = Math.max(1, ...rows.map((r) => r.count))
  return (
    <Panel icon={icon} title={title} aside={<span className="text-[12px] text-[#8d8d8d]">{unit}</span>}>
      {rows.length === 0 ? (
        <p className="py-6 text-center text-[13px] text-[#73757c]">Rien sur cette période.</p>
      ) : (
        <ul className="flex flex-col gap-1">
          {rows.map((r) => (
            <li key={r.value} className="relative flex items-center gap-3 overflow-hidden rounded-[8px] px-2 py-1.5 text-[13px] text-[#1b1b1b]">
              <span aria-hidden className="absolute inset-y-0 left-0 rounded-[8px] bg-[#fff2ea]" style={{ width: `${(r.count / max) * 100}%` }} />
              <span className="relative min-w-0 flex-1 truncate">{label(r.value)}</span>
              <span className="relative shrink-0 tabular-nums">{formatNumber(r.count)}</span>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  )
}
