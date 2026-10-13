import type { ReactNode } from 'react'

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

export function Panel({ title, aside, children }: { title: string; aside?: ReactNode; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3 rounded-[14px] border border-[#e8e8e9] bg-white p-4 break-inside-avoid">
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-[15px] font-medium text-[#1b1b1b]">{title}</h2>
        {aside}
      </header>
      {children}
    </section>
  )
}

/**
 * Un classement : la valeur, son compte, et une barre proportionnelle au
 * premier de la liste, posee sous le texte.
 */
export function TopList({ title, unit, rows, label }: { title: string; unit: string; rows: StatisticsRow[]; label: (v: string) => ReactNode }) {
  const max = Math.max(1, ...rows.map((r) => r.count))
  return (
    <Panel title={title} aside={<span className="text-[12px] text-[#8d8d8d]">{unit}</span>}>
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
