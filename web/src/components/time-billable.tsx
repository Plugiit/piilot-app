import { Clock01Icon } from '@hugeicons/core-free-icons'
import { useQuery } from '@tanstack/react-query'

import { PanelCard } from '@/components/panel-card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { dashboardQuery } from '@/features/projects/api'
import { LIGHT_TOOLTIP } from '@/lib/tooltip'
import { cn } from '@/lib/utils'

/**
 * Duree lisible, « 12 h 05 ».
 *
 * Les heures arrivent en decimal (12,08) : la conversion en minutes se fait
 * sur l'arrondi, sans quoi 12,999 afficherait « 12 h 60 ».
 */
function formatHours(hours: number) {
  const minutes = Math.round(hours * 60)

  return `${Math.floor(minutes / 60)} h ${String(minutes % 60).padStart(2, '0')}`
}

/**
 * Nombre de crans de la jauge.
 *
 * Des crans reels, repartis par une grille flex, plutot qu'un degrade repete :
 * ils sont assez larges pour se lire, et leur ecart est tenu par un `gap`
 * qu'aucun arrondi ne peut refermer.
 */
const TICKS = 64

interface Segment {
  label: string
  color: string
  hours: number
}

/**
 * Poste auquel appartient un cran.
 *
 * Le milieu du cran decide, pas son bord : une frontiere qui tombe au milieu
 * d'un cran le donne au poste qui en occupe la plus grande part.
 */
function segmentAt(index: number, segments: Segment[], total: number) {
  const position = ((index + 0.5) / TICKS) * total

  let cumulated = 0
  for (const segment of segments) {
    cumulated += segment.hours
    if (position <= cumulated) return segment
  }

  return segments[segments.length - 1]!
}

/**
 * Temps facturable de l'agence.
 *
 * Une heure est facturable quand elle est saisie sur un projet client, non
 * facturable sur un projet interne. Le restant est ce qui reste a consommer
 * des heures vendues sur les projets clients.
 */
export function TimeBillable() {
  const { data, isPending } = useQuery(dashboardQuery)
  const time = data?.time

  const segments: Segment[] = [
    { label: 'Non facturable', color: '#7bd25b', hours: time?.non_billable_hours ?? 0 },
    { label: 'Facturable', color: '#4956f4', hours: time?.billable_hours ?? 0 },
    { label: 'Restant', color: '#e6e6e6', hours: time?.remaining_hours ?? 0 },
  ]
  const total = segments.reduce((sum, segment) => sum + segment.hours, 0)

  const summary = [
    {
      label: 'Heures saisies',
      value: formatHours((time?.billable_hours ?? 0) + (time?.non_billable_hours ?? 0)),
    },
    { label: 'Heures facturables', value: formatHours(time?.billable_hours ?? 0) },
    { label: 'Non facturables', value: formatHours(time?.non_billable_hours ?? 0) },
    { label: 'Heures restantes', value: formatHours(time?.remaining_hours ?? 0) },
  ]

  return (
    <TooltipProvider>
      <PanelCard icon={Clock01Icon} title="TEMPS FACTURABLE">
        <div className="flex flex-1 flex-col justify-between gap-2.5">
          {isPending && (
            <div className="flex flex-col gap-3">
              <div className="h-8 animate-pulse rounded-[4px] bg-[#f2f2f2]" />
              <div className="h-[38px] animate-pulse rounded-[4px] bg-[#f2f2f2]" />
            </div>
          )}

          {!isPending && total === 0 && (
            <p className="py-6 text-center text-[13px] text-[#8d8d8d]">
              Aucun temps saisi ni vendu pour l’instant.
            </p>
          )}

          {!isPending && total > 0 && (
            <>
              {/* En grille, les chiffres passent a deux colonnes quand la
                  carte se resserre ; le filet suit, c'est celui de gauche. */}
              <div className="grid grid-cols-2 gap-y-3 @xl:grid-cols-4">
                {summary.map((entry, index) => (
                  <div
                    key={entry.label}
                    className={cn(
                      'flex flex-col gap-0.5 px-3',
                      // Colle au bord gauche en debut de ligne : la premiere
                      // cellule toujours, la troisieme tant qu'elle ouvre la
                      // seconde ligne.
                      index === 0 && 'pl-0',
                      index === 2 && 'pl-0 @xl:pl-3',
                      index % 2 === 1 && 'border-l border-[#e6e6e6]',
                      '@xl:border-l @xl:first:border-l-0',
                    )}
                  >
                    <p className="truncate text-[12px] text-[#64748b]">{entry.label}</p>
                    <p className="text-[12px] font-semibold text-[#0f172a] tabular-nums">
                      {entry.value}
                    </p>
                  </div>
                ))}
              </div>

              <Tooltip>
                <TooltipTrigger asChild>
                  <div
                    aria-label={segments
                      .map((s) => `${s.label} : ${formatHours(s.hours)}`)
                      .join(', ')}
                    className="flex h-[38px] items-stretch gap-0.5"
                  >
                    {Array.from({ length: TICKS }, (_, index) => (
                      <span
                        key={index}
                        aria-hidden
                        className="min-w-px flex-1 rounded-[2px]"
                        style={{ backgroundColor: segmentAt(index, segments, total).color }}
                      />
                    ))}
                  </div>
                </TooltipTrigger>

                <TooltipContent
                  className={cn(LIGHT_TOOLTIP, 'flex-col items-start gap-1 px-3 py-2')}
                >
                  <p className="text-[11px] leading-none text-[#777]">
                    {formatHours(time?.budget_hours ?? 0)} vendues sur les projets clients
                  </p>

                  <div className="flex flex-col gap-1 pt-0.5">
                    {segments.map((segment) => (
                      <p
                        key={segment.label}
                        className="flex items-center gap-1.5 text-[11px] leading-none text-[#777]"
                      >
                        <span
                          aria-hidden
                          className="size-2 shrink-0 rounded-[2px]"
                          style={{ backgroundColor: segment.color }}
                        />
                        {segment.label} · {formatHours(segment.hours)}
                      </p>
                    ))}
                  </div>
                </TooltipContent>
              </Tooltip>
            </>
          )}
        </div>
      </PanelCard>
    </TooltipProvider>
  )
}
