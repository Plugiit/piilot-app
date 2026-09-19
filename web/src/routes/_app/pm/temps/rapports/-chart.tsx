import { ChartHistogramIcon } from '@hugeicons/core-free-icons'

import { PanelCard } from '@/components/panel-card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { formatDuration } from '@/features/time/format'
import { bucketLabel, bucketTitle } from '@/features/time/period'
import { LIGHT_TOOLTIP } from '@/lib/tooltip'
import { cn } from '@/lib/utils'
import type { TimeReport } from '@/types/api'

export const BILLABLE_COLOR = '#4956f4'
export const NON_BILLABLE_COLOR = '#7bd25b'

const BUCKET_NAMES = {
  day: 'par jour',
  week: 'par semaine',
  month: 'par mois',
} as const

/**
 * Evolution du temps sur la periode, en barres empilees : facturable en bas,
 * non facturable au-dessus.
 *
 * Une barre par tranche, vides comprises — un jour sans pointage se voit comme
 * un creux, au lieu de disparaitre de l'axe.
 */
export function ReportChart({ report }: { report: TimeReport }) {
  const series = report.series
  const max = Math.max(1, ...series.map((b) => b.billable_minutes + b.non_billable_minutes))

  // Un libelle toutes les N barres : au-dela d'une douzaine, ils se
  // chevaucheraient.
  const every = Math.max(1, Math.ceil(series.length / 12))

  return (
    <TooltipProvider>
      <PanelCard
        icon={ChartHistogramIcon}
        title="ÉVOLUTION"
        action={
          <span className="flex items-center gap-3 pr-1 text-[12px] text-[#64748b]">
            <Legend color={BILLABLE_COLOR} label="Facturable" />
            <Legend color={NON_BILLABLE_COLOR} label="Non facturable" />
            <span className="hidden sm:inline">· {BUCKET_NAMES[report.bucket]}</span>
          </span>
        }
      >
        <div className="flex flex-col gap-1.5 px-1 pt-2">
          <div className="flex h-[180px] items-end gap-[3px]">
            {series.map((bucket) => {
              const total = bucket.billable_minutes + bucket.non_billable_minutes

              return (
                <Tooltip key={bucket.start}>
                  <TooltipTrigger asChild>
                    <div
                      className="group flex h-full min-w-0 flex-1 cursor-default flex-col justify-end"
                      aria-label={`${bucketTitle(bucket.start, report.bucket)} : ${formatDuration(total)}`}
                    >
                      {total === 0 ? (
                        <div className="h-[2px] rounded-full bg-[#ececec]" />
                      ) : (
                        <div
                          className="flex flex-col overflow-hidden rounded-[3px] transition-opacity group-hover:opacity-80"
                          style={{ height: `${(total / max) * 100}%` }}
                        >
                          <div
                            style={{
                              flex: bucket.non_billable_minutes,
                              backgroundColor: NON_BILLABLE_COLOR,
                            }}
                          />
                          <div
                            style={{
                              flex: bucket.billable_minutes,
                              backgroundColor: BILLABLE_COLOR,
                            }}
                          />
                        </div>
                      )}
                    </div>
                  </TooltipTrigger>
                  <TooltipContent
                    className={cn(LIGHT_TOOLTIP, 'flex-col items-start gap-1 px-3 py-2')}
                  >
                    <p className="text-[12px] font-medium text-[#111] first-letter:uppercase">
                      {bucketTitle(bucket.start, report.bucket)}
                    </p>
                    <p className="text-[11px] text-[#777]">Total · {formatDuration(total)}</p>
                    <p className="text-[11px] text-[#777]">
                      Facturable · {formatDuration(bucket.billable_minutes)}
                    </p>
                    <p className="text-[11px] text-[#777]">
                      Non facturable · {formatDuration(bucket.non_billable_minutes)}
                    </p>
                  </TooltipContent>
                </Tooltip>
              )
            })}
          </div>

          <div className="flex gap-[3px] pb-1">
            {series.map((bucket, index) => (
              <span
                key={bucket.start}
                className="min-w-0 flex-1 overflow-visible text-center text-[10px] whitespace-nowrap text-[#8d8d8d]"
              >
                {index % every === 0 ? bucketLabel(bucket.start, report.bucket) : ''}
              </span>
            ))}
          </div>
        </div>
      </PanelCard>
    </TooltipProvider>
  )
}

function Legend({ color, label }: { color: string; label: string }) {
  return (
    <span className="flex items-center gap-1">
      <span aria-hidden className="size-2 rounded-[2px]" style={{ backgroundColor: color }} />
      {label}
    </span>
  )
}
