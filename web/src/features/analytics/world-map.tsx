import { useEffect, useState } from 'react'

import { countryLabel, formatNumber } from '@/features/analytics/format'
import type { StatisticsRow } from '@/types/api'

interface WorldShape {
  width: number
  height: number
  countries: Record<string, string>
}

/**
 * La carte du monde de la page « Statistiques » : chaque pays teinte selon
 * ses visiteurs, plus fonce qu'il en compte. Le fond de carte est un fichier
 * statique genere une fois (scripts/world-map.mjs), charge a part : la page
 * s'affiche sans l'attendre.
 *
 * L'echelle est logarithmique : un pays a dix mille visiteurs ne doit pas
 * effacer ceux qui en ont trente.
 */
export function WorldMap({ rows }: { rows: StatisticsRow[] }) {
  const [shape, setShape] = useState<WorldShape | null>(null)
  const [hover, setHover] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    void import('@/assets/world-map.json').then((m) => {
      if (alive) setShape(m.default as WorldShape)
    })
    return () => {
      alive = false
    }
  }, [])

  const counts = new Map(rows.filter((r) => r.value !== '').map((r) => [r.value.toUpperCase(), r.count]))
  const max = Math.max(1, ...counts.values())
  const tone = (n: number) => 0.18 + 0.82 * (Math.log(n + 1) / Math.log(max + 1))
  const hovered = hover === null ? null : { label: countryLabel(hover), count: counts.get(hover) ?? 0 }

  return (
    <div className="relative w-full">
      <div className="aspect-[960/470] w-full">
        {shape !== null && (
          <svg viewBox={`0 0 ${shape.width} ${shape.height}`} className="block h-full w-full" role="img" aria-label="Visiteurs par pays">
            {Object.entries(shape.countries).map(([code, d]) => {
              const n = counts.get(code)
              return (
                <path
                  key={code}
                  d={d}
                  fill={n === undefined ? '#ececed' : '#ff782b'}
                  fillOpacity={n === undefined ? 1 : tone(n)}
                  stroke="#fff"
                  strokeWidth={0.5}
                  className="transition-[fill-opacity] hover:fill-opacity-100"
                  onPointerEnter={() => setHover(code)}
                  onPointerLeave={() => setHover((h) => (h === code ? null : h))}
                />
              )
            })}
          </svg>
        )}
      </div>
      {hovered !== null && (
        <p className="pointer-events-none absolute top-2 left-2 flex items-center gap-2 rounded-[8px] border border-[#e8e8e9] bg-white px-2.5 py-1.5 text-[13px] text-[#1b1b1b] shadow-[0_8px_24px_-12px_rgb(16_24_40/0.25)]">
          <span aria-hidden>{hovered.label.flag}</span>
          {hovered.label.name}
          <span className="font-medium tabular-nums">{formatNumber(hovered.count)}</span>
        </p>
      )}
    </div>
  )
}
