import { ArrowDown01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useEffect, useRef, useState } from 'react'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { formatNumber, localDate } from '@/features/analytics/format'
import { cn } from '@/lib/utils'
import type { AudienceSite, PortalUptimeSite } from '@/types/api'

const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })
const DAY_LONG = new Intl.DateTimeFormat('fr-FR', { weekday: 'long', day: 'numeric', month: 'long' })
const UPTIME = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 2 })

/** L'etat d'un site surveille, dit au client. */
const UPTIME_STATUS: Record<PortalUptimeSite['status'], string> = {
  up: 'En ligne',
  down: 'Hors ligne',
  pending: 'En attente',
  maintenance: 'En maintenance',
}

interface Point {
  at: string
  visitors: number
}

/**
 * Le site montre a l'arrivee : le plus visite, le site principal du client le
 * plus souvent.
 */
export function defaultSite(sites: AudienceSite[]): string | undefined {
  return [...sites].sort((a, b) => b.visitors - a.visitors)[0]?.id
}

/**
 * Le bandeau des sites, sous la barre du portail : les visiteurs d'un site sur
 * trente jours, en escalier blanc sur l'orange de la marque, et sur la page
 * d'un projet la disponibilite de son site.
 *
 * Un site a la fois : additionner des audiences differentes ne dit rien au
 * client. Avec plusieurs sites, un selecteur passe de l'un a l'autre ; avec un
 * seul, son domaine tient lieu de legende. Sans statistiques, le bandeau se
 * reduit a la disponibilite.
 */
export function AudienceBand({
  sites,
  selected,
  onSelect,
  uptime = [],
}: {
  sites: AudienceSite[]
  selected: string
  onSelect: (site: string) => void
  uptime?: PortalUptimeSite[]
}) {
  const site = sites.find((s) => s.id === selected) ?? sites[0]
  const points = site?.series ?? []
  const total = site?.visitors ?? 0
  const [hover, setHover] = useState<Point | null>(null)

  if (site === undefined) {
    return (
      <div className="flex min-h-9 flex-wrap items-center gap-x-4 gap-y-1 px-4 pb-2">
        {uptime.map((u) => (
          <UptimeLine key={u.id} site={u} named />
        ))}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-1 px-4 pt-1 pb-3">
      <div className="flex min-h-7 flex-wrap items-center gap-x-3 gap-y-1">
        <p className="min-w-0 flex-1 truncate text-[13px] text-white/60">
          {hover === null ? (
            <>
              <span className="font-medium text-white tabular-nums">{formatNumber(total)}</span> visiteurs sur 30 jours
            </>
          ) : (
            <>
              <span className="font-medium text-white tabular-nums">{formatNumber(hover.visitors)}</span> visiteurs le{' '}
              {DAY_LONG.format(localDate(hover.at))}
            </>
          )}
        </p>
        {uptime.map((u) => (
          <UptimeLine key={u.id} site={u} named={uptime.length > 1} />
        ))}
        {sites.length > 1 ? (
          <DropdownMenu>
            <DropdownMenuTrigger className="flex h-7 min-w-0 cursor-pointer items-center gap-1.5 rounded-[8px] px-2.5 text-[13px] font-medium text-white/60 transition-colors hover:bg-white/10 hover:text-white data-[state=open]:bg-white/10 data-[state=open]:text-white">
              <span className="max-w-[200px] truncate">{site?.domain || site?.name}</span>
              <HugeiconsIcon icon={ArrowDown01Icon} size={14} strokeWidth={1.8} className="shrink-0" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-60">
              <DropdownMenuRadioGroup value={site?.id} onValueChange={onSelect}>
                {sites.map((s) => (
                  <DropdownMenuRadioItem key={s.id} value={s.id}>
                    <span className="truncate">{s.domain || s.name}</span>
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : (
          <span className="truncate text-[13px] text-white/60">{site?.domain || site?.name}</span>
        )}
      </div>
      <StepChart points={points} onHover={setHover} />
    </div>
  )
}

/**
 * La disponibilite d'un site surveille : son etat, et la part du temps ou il a
 * repondu sur trente jours. Hors ligne, la ligne passe en blanc plein pour se
 * voir sur l'orange.
 */
function UptimeLine({ site, named }: { site: PortalUptimeSite; named: boolean }) {
  const down = site.status === 'down'
  return (
    <p
      className={cn(
        'flex h-7 shrink-0 items-center gap-1.5 rounded-[8px] px-2.5 text-[13px] whitespace-nowrap',
        down ? 'bg-white font-medium text-[#d92d20]' : 'bg-white/10 text-white/80',
      )}
    >
      <span aria-hidden className={cn('size-2 rounded-full', down ? 'bg-[#d92d20]' : site.status === 'up' ? 'bg-[#7ee2a8]' : 'bg-white/60')} />
      {named && <span className="max-w-[160px] truncate">{site.name} ·</span>}
      {UPTIME_STATUS[site.status]}
      {site.uptime_30d !== null && (
        <span className={down ? undefined : 'text-white/60'}>
          · <span className={cn('tabular-nums', !down && 'font-medium text-white')}>{UPTIME.format(site.uptime_30d)} %</span> sur 30 jours
        </span>
      )}
    </p>
  )
}

/**
 * La courbe en escalier : un palier par jour, des axes a peine marques. Sa
 * largeur se mesure, comme les autres graphiques, pour des textes nets.
 */
function StepChart({ points, onHover }: { points: Point[]; onHover: (p: Point | null) => void }) {
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

  const height = 96
  const left = 32
  const top = 6
  const bottom = 20
  const plotW = Math.max(0, width - left)
  const plotH = height - top - bottom
  const n = Math.max(1, points.length)
  const max = niceMax(Math.max(1, ...points.map((p) => p.visitors)))
  const x = (i: number) => left + (i / n) * plotW
  const y = (v: number) => top + plotH - (v / max) * plotH

  // Un palier par jour : chaque point tient toute sa largeur.
  const line = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i)},${y(p.visitors)} L${x(i + 1)},${y(p.visitors)}`).join(' ')
  const room = Math.max(2, Math.min(8, Math.floor(plotW / 90)))
  const step = Math.max(1, Math.ceil(points.length / room))

  function pick(i: number | null) {
    setHover(i)
    onHover(i === null ? null : (points[i] ?? null))
  }

  function onMove(event: React.PointerEvent<SVGRectElement>) {
    const box = event.currentTarget.getBoundingClientRect()
    pick(Math.min(points.length - 1, Math.max(0, Math.floor(((event.clientX - box.left) / box.width) * n))))
  }

  return (
    <div ref={ref} className="w-full" style={{ height }}>
      {width > 0 && points.length > 0 && (
        <svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Visiteurs par jour" className="block">
          {[0, max / 2, max].map((t) => (
            <g key={t}>
              <line x1={left} x2={width} y1={y(t)} y2={y(t)} stroke="rgb(255 255 255 / 0.12)" />
              <text x={left - 6} y={y(t)} dy="0.32em" textAnchor="end" className="fill-white/60 text-[11px] tabular-nums">
                {compact(t)}
              </text>
            </g>
          ))}
          {points.map((p, i) =>
            i % step === 0 ? (
              <text key={p.at} x={x(i)} y={height - 4} className="fill-white/60 text-[11px]">
                {DAY.format(localDate(p.at))}
              </text>
            ) : null,
          )}
          {hover !== null && <rect x={x(hover)} y={top} width={plotW / n} height={plotH} fill="rgb(255 255 255 / 0.12)" />}
          <path d={`${line} L${x(points.length)},${y(0)} L${x(0)},${y(0)} Z`} fill="rgb(255 255 255 / 0.18)" />
          <path d={line} fill="none" stroke="rgb(255 255 255 / 0.85)" strokeWidth={1.5} strokeLinejoin="round" />
          <rect x={left} y={top} width={plotW} height={plotH} fill="transparent" onPointerMove={onMove} onPointerLeave={() => pick(null)} />
        </svg>
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
