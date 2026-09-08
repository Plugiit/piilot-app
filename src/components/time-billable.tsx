import { Clock01Icon } from '@hugeicons/core-free-icons'

import { DashboardCard } from '@/components/dashboard-card'
import { cn } from '@/lib/utils'

/**
 * Temps de la periode, en secondes.
 *
 * Figes, mais coherents entre eux : les chiffres affiches et les longueurs de
 * la barre sortent des memes valeurs, donc l'un ne peut pas contredire
 * l'autre. Le dessin, lui, annonce un total qui ne tombe pas juste avec ses
 * propres postes.
 */
const BUDGET = 12 * 3600
const BILLABLE = 3 * 3600 + 24 * 60 + 7
const INTERNAL = 4 * 3600 + 51 * 60 + 38
const REMAINING = BUDGET - BILLABLE - INTERNAL

/** Montant deja facturable, au taux de l'agence. */
const AMOUNT = 1020

const SEGMENTS = [
  { label: 'Non facturable', color: '#7bd25b', seconds: INTERNAL },
  { label: 'Facturable', color: '#4956f4', seconds: BILLABLE },
  { label: 'Restant', color: '#e6e6e6', seconds: REMAINING },
]

const AMOUNT_FORMAT = new Intl.NumberFormat('fr-FR', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
})

function formatDuration(seconds: number) {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  return [hours, minutes, seconds % 60].map((part) => String(part).padStart(2, '0')).join(':')
}

const SUMMARY = [
  { label: 'Heures saisies', value: formatDuration(BILLABLE + INTERNAL) },
  { label: 'Heures facturables', value: formatDuration(BILLABLE) },
  { label: 'Heures restantes', value: formatDuration(REMAINING) },
  { label: 'Montant facturable', value: AMOUNT_FORMAT.format(AMOUNT) },
]

export function TimeBillable() {
  return (
    <DashboardCard icon={Clock01Icon} title="TEMPS FACTURABLE">
      <div className="flex flex-1 flex-col justify-between gap-2.5">
        {/* Le dessin aligne les quatre chiffres sur une ligne fixe, separes par
            des filets. En grille ils passent a deux colonnes quand la carte se
            resserre, et le filet suit — c'est celui de gauche, donc il tombe
            de lui-meme en debut de ligne. */}
        <div className="grid grid-cols-2 gap-y-3 @xl:grid-cols-4">
          {SUMMARY.map((entry, index) => (
            <div
              key={entry.label}
              className={cn(
                'flex flex-col gap-0.5 px-3 first:pl-0',
                index % 2 === 1 && 'border-l border-[#e6e6e6]',
                '@xl:border-l @xl:first:border-l-0',
              )}
            >
              <p className="truncate text-[12px] text-[#64748b]">{entry.label}</p>
              <p className="text-[12px] font-semibold text-[#0f172a] tabular-nums">{entry.value}</p>
            </div>
          ))}
        </div>

        {/* Les raies sont peintes par un degrade repete plutot que par 130
            elements : a largeur variable, des raies de 2px non compressibles
            deborderaient de la carte. Le pas de 5px reproduit le trait de 2px
            et son ecart. */}
        <div className="flex h-[38px] overflow-hidden rounded-[4px]">
          {SEGMENTS.map((segment) => (
            <div
              key={segment.label}
              style={{
                width: `${(segment.seconds / BUDGET) * 100}%`,
                backgroundImage: `repeating-linear-gradient(90deg, ${segment.color} 0 2px, transparent 2px 5px)`,
              }}
            />
          ))}
        </div>

        <div className="flex flex-wrap items-center gap-x-[11px] gap-y-2">
          {SEGMENTS.map((segment) => (
            <div key={segment.label} className="flex items-center gap-2">
              <div
                aria-hidden
                className="size-2 shrink-0 rounded-[2px]"
                style={{ backgroundColor: segment.color }}
              />
              <p className="text-[12px] leading-[1.5] font-medium whitespace-nowrap text-[#030512]">
                {segment.label}
              </p>
            </div>
          ))}
        </div>
      </div>
    </DashboardCard>
  )
}
