import { StatusPill } from '@/features/projects/ui'
import { INTERVENTION_KIND } from '@/features/maintenance/format'
import { formatDuration } from '@/features/time/format'
import type { InterventionKind } from '@/types/api'

export function KindPill({ kind }: { kind: InterventionKind }) {
  const k = INTERVENTION_KIND[kind]
  return <StatusPill label={k.label} color={k.color} pill={k.pill} />
}

/**
 * Le temps consomme sur le temps inclus : une jauge, et le depassement en
 * rouge au-dela. Sans montant — la facturation reste ailleurs.
 */
export function ConsumptionBar({ consumed, included }: { consumed: number; included: number | null }) {
  if (included === null) {
    return <p className="text-[13px] text-[#73757c] tabular-nums">{formatDuration(consumed)} · sans formule</p>
  }
  const ratio = included === 0 ? (consumed > 0 ? 1 : 0) : Math.min(1, consumed / included)
  const overrun = Math.max(0, consumed - included)
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-baseline justify-between gap-2 text-[13px] tabular-nums">
        <span className="text-[#1b1b1b]">
          <strong className="font-medium">{formatDuration(consumed)}</strong>
          <span className="text-[#73757c]"> sur {formatDuration(included)}</span>
        </span>
        {overrun > 0 && <span className="font-medium text-[#e5484d]">+{formatDuration(overrun)}</span>}
      </div>
      <div aria-hidden className="h-1.5 w-full overflow-hidden rounded-full bg-[#ebebeb]">
        <div className="h-full rounded-full" style={{ width: `${ratio * 100}%`, backgroundColor: overrun > 0 ? '#e5484d' : '#0db471' }} />
      </div>
    </div>
  )
}
