import { LayoutTable01Icon } from '@hugeicons/core-free-icons'

import { PanelCard } from '@/components/panel-card'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatDuration } from '@/features/time/format'
import type { ReportGroupBy } from '@/features/time/reports'
import { cn } from '@/lib/utils'
import type { TimeReport } from '@/types/api'

import { BILLABLE_COLOR, NON_BILLABLE_COLOR } from './-chart'

export const GROUP_LABELS: Record<ReportGroupBy, string> = {
  project: 'Projet',
  user: 'Personne',
  service: 'Service',
  client: 'Client',
}

const PERCENT = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 0 })

/**
 * Repartition du temps par projet, personne, service ou client.
 *
 * Cliquer une ligne filtre tout l'ecran sur elle : c'est le geste pour
 * descendre d'un cran — de « quel client a coute le plus » a « sur quels
 * projets de ce client », sans rien ressaisir.
 */
export function ReportGroups({
  report,
  groupBy,
  onGroupBy,
  onPick,
  onPage,
  fetching,
}: {
  report: TimeReport
  groupBy: ReportGroupBy
  onGroupBy: (value: ReportGroupBy) => void
  /** Filtre sur la ligne choisie. Absent pour les lignes non filtrables. */
  onPick: (key: string) => void
  onPage: (page: number) => void
  fetching: boolean
}) {
  const { groups, totals } = report
  const pages = Math.max(1, Math.ceil(groups.total / groups.page_size))

  const tabs = (
    <Tabs value={groupBy} onValueChange={(value) => onGroupBy(value as ReportGroupBy)}>
      <TabsList className="h-8">
        {(Object.keys(GROUP_LABELS) as ReportGroupBy[]).map((key) => (
          <TabsTrigger key={key} value={key} className="px-2.5 text-[12px]">
            {GROUP_LABELS[key]}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  )

  return (
    <PanelCard
      icon={LayoutTable01Icon}
      title="RÉPARTITION"
      // Les onglets tiennent dans l'en-tete a partir d'une tablette ; sur un
      // telephone ils passent sous le titre, sur toute la largeur.
      action={<div className="hidden sm:block">{tabs}</div>}
    >
      <div className={cn('flex flex-col rounded-[10px] bg-white', fetching && 'opacity-60')}>
        <div className="px-2 pt-2 sm:hidden">{tabs}</div>

        <div className="overflow-x-auto">
          <table className="w-full min-w-[620px] text-[13px]">
            <thead>
              <tr className="border-b border-[#eee] text-left text-[12px] text-[#64748b]">
                <th className="px-3 py-2 font-normal">{GROUP_LABELS[groupBy]}</th>
                <th className="px-3 py-2 text-right font-normal">Temps</th>
                <th className="px-3 py-2 text-right font-normal">Facturable</th>
                <th className="px-3 py-2 text-right font-normal">Non facturable</th>
                <th className="w-[200px] px-3 py-2 font-normal">Part du total</th>
              </tr>
            </thead>
            <tbody>
              {groups.items.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-3 py-8 text-center text-[#8d8d8d]">
                    Aucun temps sur cette période.
                  </td>
                </tr>
              )}

              {groups.items.map((group) => {
                const share = totals.minutes === 0 ? 0 : (group.minutes / totals.minutes) * 100
                const pickable = group.key !== ''

                return (
                  <tr
                    key={group.key || 'none'}
                    onClick={pickable ? () => onPick(group.key) : undefined}
                    className={cn(
                      'border-b border-[#f4f4f4] last:border-0',
                      pickable && 'cursor-pointer hover:bg-[#fafafa]',
                    )}
                    title={pickable ? `Filtrer sur « ${group.label} »` : undefined}
                  >
                    <td className="px-3 py-2">
                      <span className="flex min-w-0 items-center gap-2">
                        {group.color !== '' && (
                          <span
                            aria-hidden
                            className="size-2 shrink-0 rounded-full"
                            style={{ backgroundColor: group.color }}
                          />
                        )}
                        <span className="truncate text-[#111]">{group.label || 'Sans nom'}</span>
                        {group.detail !== '' && (
                          <span className="truncate text-[12px] text-[#8d8d8d]">
                            {group.detail}
                          </span>
                        )}
                      </span>
                    </td>
                    <td className="px-3 py-2 text-right font-medium text-[#111] tabular-nums">
                      {formatDuration(group.minutes)}
                    </td>
                    <td className="px-3 py-2 text-right text-[#444] tabular-nums">
                      {formatDuration(group.billable_minutes)}
                    </td>
                    <td className="px-3 py-2 text-right text-[#444] tabular-nums">
                      {formatDuration(group.non_billable_minutes)}
                    </td>
                    <td className="px-3 py-2">
                      <span className="flex items-center gap-2">
                        <span className="flex h-1.5 flex-1 overflow-hidden rounded-full bg-[#f0f0f0]">
                          <span
                            style={{
                              width: `${totals.minutes === 0 ? 0 : (group.billable_minutes / totals.minutes) * 100}%`,
                              backgroundColor: BILLABLE_COLOR,
                            }}
                          />
                          <span
                            style={{
                              width: `${totals.minutes === 0 ? 0 : (group.non_billable_minutes / totals.minutes) * 100}%`,
                              backgroundColor: NON_BILLABLE_COLOR,
                            }}
                          />
                        </span>
                        <span className="w-9 text-right text-[12px] text-[#64748b] tabular-nums">
                          {PERCENT.format(share)} %
                        </span>
                      </span>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>

        {pages > 1 && (
          <div className="flex items-center justify-between gap-2 border-t border-[#eee] px-3 py-2">
            <p className="text-[12px] text-[#777]">
              {groups.total} lignes · page {groups.page} sur {pages}
            </p>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={groups.page <= 1}
                onClick={() => onPage(groups.page - 1)}
              >
                Précédent
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={groups.page >= pages}
                onClick={() => onPage(groups.page + 1)}
              >
                Suivant
              </Button>
            </div>
          </div>
        )}
      </div>
    </PanelCard>
  )
}
