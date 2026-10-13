import { useEffect, useRef, useState } from 'react'

/**
 * Courbe a deux series, axes legers et lecture au survol : la courbe de la
 * page « Statistiques ». Dessinee a la main en SVG, comme les autres
 * graphiques de l'application : sa largeur se mesure, pour que les textes
 * restent nets et les points ou ils doivent etre.
 */
export interface LineSeries {
  key: string
  label: string
  color: string
  /** Une aire sous la courbe : la serie principale. */
  area?: boolean
}

export function LineChart<T extends { at: string }>({
  points,
  series,
  value,
  formatX,
  formatTooltipX,
  height = 240,
}: {
  points: T[]
  series: LineSeries[]
  value: (point: T, key: string) => number
  /** Le libelle d'un pas sous l'axe. */
  formatX: (at: string) => string
  formatTooltipX?: (at: string) => string
  height?: number
}) {
  const ref = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(0)
  const [hover, setHover] = useState<number | null>(null)

  useEffect(() => {
    const el = ref.current
    if (el === null) return
    const observer = new ResizeObserver(([entry]) => setWidth(entry?.contentRect.width ?? 0))
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  const left = 40
  const right = 8
  const top = 8
  const bottom = 26
  const plotW = Math.max(0, width - left - right)
  const plotH = height - top - bottom

  const max = niceMax(Math.max(1, ...points.flatMap((p) => series.map((s) => value(p, s.key)))))
  const ticks = [0, max / 4, max / 2, (max * 3) / 4, max]
  const x = (i: number) => left + (points.length <= 1 ? plotW / 2 : (i / (points.length - 1)) * plotW)
  const y = (v: number) => top + plotH - (v / max) * plotH

  // Un libelle tous les 80 px au plus, six au maximum : sur un telephone,
  // trente dates se chevaucheraient. Le dernier pas garde le sien, et celui
  // qui le precede s'efface s'il le touche.
  const room = Math.max(2, Math.min(6, Math.floor(plotW / 80)))
  const step = Math.max(1, Math.ceil(points.length / room))
  const last = points.length - 1
  const labels = points
    .map((p, i) => ({ i, at: p.at }))
    .filter(({ i }) => (i % step === 0 && (i === last || last - i >= step / 2)) || i === last)

  function onMove(event: React.PointerEvent<SVGRectElement>) {
    if (points.length === 0) return
    const box = event.currentTarget.getBoundingClientRect()
    const rel = (event.clientX - box.left) / box.width
    setHover(Math.min(points.length - 1, Math.max(0, Math.round(rel * (points.length - 1)))))
  }

  const hovered = hover === null ? undefined : points[hover]

  return (
    <div ref={ref} className="relative w-full" style={{ height }}>
      {width > 0 && (
        <svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" role="img" aria-label={series.map((s) => s.label).join(' et ')} className="block">
          <defs>
            {series
              .filter((s) => s.area)
              .map((s) => (
                <linearGradient key={s.key} id={`area-${s.key}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={s.color} stopOpacity={0.18} />
                  <stop offset="100%" stopColor={s.color} stopOpacity={0} />
                </linearGradient>
              ))}
          </defs>

          {ticks.map((t) => (
            <g key={t}>
              <line x1={left} x2={width - right} y1={y(t)} y2={y(t)} stroke="#f0f0f1" />
              <text x={left - 8} y={y(t)} dy="0.32em" textAnchor="end" className="fill-[#8d8d8d] text-[11px] tabular-nums">
                {compact(t)}
              </text>
            </g>
          ))}

          {labels.map(({ i, at }) => (
            <text key={at} x={x(i)} y={height - 6} textAnchor={i === 0 ? 'start' : i === points.length - 1 ? 'end' : 'middle'} className="fill-[#8d8d8d] text-[11px]">
              {formatX(at)}
            </text>
          ))}

          {series.map((s) => {
            if (points.length === 0) return null
            const line = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i)},${y(value(p, s.key))}`).join(' ')
            return (
              <g key={s.key}>
                {s.area && <path d={`${line} L${x(points.length - 1)},${y(0)} L${x(0)},${y(0)} Z`} fill={`url(#area-${s.key})`} />}
                <path d={line} fill="none" stroke={s.color} strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" />
              </g>
            )
          })}

          {hover !== null && hovered !== undefined && (
            <g>
              <line x1={x(hover)} x2={x(hover)} y1={top} y2={top + plotH} stroke="#d5d5d5" strokeDasharray="3 3" />
              {series.map((s) => (
                <circle key={s.key} cx={x(hover)} cy={y(value(hovered, s.key))} r={4} fill="#fff" stroke={s.color} strokeWidth={2} />
              ))}
            </g>
          )}

          <rect
            x={left}
            y={top}
            width={plotW}
            height={plotH}
            fill="transparent"
            onPointerMove={onMove}
            onPointerLeave={() => setHover(null)}
          />
        </svg>
      )}

      {hover !== null && hovered !== undefined && (
        <div
          className="pointer-events-none absolute top-2 z-10 flex min-w-[150px] flex-col gap-1 rounded-[10px] border border-[#e8e8e9] bg-white px-3 py-2 shadow-[0_8px_24px_-12px_rgb(16_24_40/0.25)]"
          style={x(hover) > width / 2 ? { right: width - x(hover) + 12 } : { left: x(hover) + 12 }}
        >
          <p className="text-[12px] text-[#73757c]">{(formatTooltipX ?? formatX)(hovered.at)}</p>
          {series.map((s) => (
            <p key={s.key} className="flex items-center gap-2 text-[13px] text-[#1b1b1b]">
              <span aria-hidden className="size-2 rounded-full" style={{ backgroundColor: s.color }} />
              <span className="flex-1">{s.label}</span>
              <span className="font-medium tabular-nums">{new Intl.NumberFormat('fr-FR').format(value(hovered, s.key))}</span>
            </p>
          ))}
        </div>
      )}
    </div>
  )
}

/** Un plafond rond : 1, 2, 2,5, 5 ou 10 fois une puissance de dix. */
function niceMax(v: number): number {
  const power = 10 ** Math.floor(Math.log10(v))
  for (const m of [1, 2, 2.5, 5, 10]) {
    if (v <= m * power) return m * power
  }
  return 10 * power
}

function compact(v: number): string {
  return new Intl.NumberFormat('fr-FR', { notation: 'compact', maximumFractionDigits: 1 }).format(v)
}
