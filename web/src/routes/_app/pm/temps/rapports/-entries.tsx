import { Clock01Icon } from '@hugeicons/core-free-icons'
import { Link } from '@tanstack/react-router'

import { PanelCard } from '@/components/panel-card'
import { Button } from '@/components/ui/button'
import { formatDuration } from '@/features/time/format'
import { parseDay } from '@/features/time/period'
import { cn } from '@/lib/utils'
import type { ReportEntryPage } from '@/types/api'

const DAY = new Intl.DateTimeFormat('fr-FR', {
  weekday: 'short',
  day: 'numeric',
  month: 'short',
})

/** Detail des saisies de la periode filtree, la plus recente en tete. */
export function ReportEntries({
  page,
  onPage,
  fetching,
}: {
  page: ReportEntryPage | undefined
  onPage: (page: number) => void
  fetching: boolean
}) {
  const items = page?.items ?? []
  const pages = page === undefined ? 1 : Math.max(1, Math.ceil(page.total / page.page_size))

  return (
    <PanelCard icon={Clock01Icon} title="SAISIES">
      <div className={cn('flex flex-col rounded-[10px] bg-white', fetching && 'opacity-60')}>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-[13px]">
            <thead>
              <tr className="border-b border-[#eee] text-left text-[12px] text-[#64748b]">
                <th className="px-3 py-2 font-normal">Date</th>
                <th className="px-3 py-2 font-normal">Personne</th>
                <th className="px-3 py-2 font-normal">Projet</th>
                <th className="px-3 py-2 font-normal">Tâche · Service</th>
                <th className="px-3 py-2 font-normal">Note</th>
                <th className="px-3 py-2 text-right font-normal">Durée</th>
              </tr>
            </thead>
            <tbody>
              {page !== undefined && items.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-3 py-8 text-center text-[#8d8d8d]">
                    Aucune saisie sur cette période.
                  </td>
                </tr>
              )}

              {items.map((entry) => (
                <tr key={entry.id} className="border-b border-[#f4f4f4] align-top last:border-0">
                  <td className="px-3 py-2 whitespace-nowrap text-[#444] first-letter:uppercase">
                    {DAY.format(parseDay(entry.spent_on))}
                  </td>
                  <td className="px-3 py-2 whitespace-nowrap text-[#111]">
                    {entry.user_name || 'Sans nom'}
                  </td>
                  <td className="px-3 py-2">
                    <span className="flex flex-col">
                      <Link
                        to="/pm/projets/$id"
                        params={{ id: entry.project_id }}
                        className="text-[#111] hover:underline"
                      >
                        {entry.project_name}
                      </Link>
                      <span className="flex items-center gap-1.5 text-[12px] text-[#8d8d8d]">
                        {entry.client_name}
                        {!entry.billable && (
                          <span className="rounded-full bg-[#eef9e9] px-1.5 text-[11px] text-[#3f8a26]">
                            Interne
                          </span>
                        )}
                      </span>
                    </span>
                  </td>
                  <td className="px-3 py-2">
                    <span className="flex flex-col text-[12px]">
                      <span className="text-[#444]">{entry.task_title ?? '—'}</span>
                      {entry.service_name !== null && (
                        <span className="flex items-center gap-1 text-[#8d8d8d]">
                          <span
                            aria-hidden
                            className="size-1.5 rounded-full"
                            style={{
                              backgroundColor: entry.service_color ?? '#ccc',
                            }}
                          />
                          {entry.service_name}
                        </span>
                      )}
                    </span>
                  </td>
                  <td className="max-w-[260px] px-3 py-2 text-[12px] text-[#64748b]">
                    <span className="line-clamp-2">{entry.note}</span>
                  </td>
                  <td className="px-3 py-2 text-right font-medium whitespace-nowrap text-[#111] tabular-nums">
                    {formatDuration(entry.minutes)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {page !== undefined && page.total > 0 && (
          <div className="flex items-center justify-between gap-2 border-t border-[#eee] px-3 py-2">
            <p className="text-[12px] text-[#777]">
              {page.total} saisie{page.total > 1 ? 's' : ''} · page {page.page} sur {pages}
            </p>
            {pages > 1 && (
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page.page <= 1}
                  onClick={() => onPage(page.page - 1)}
                >
                  Précédent
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page.page >= pages}
                  onClick={() => onPage(page.page + 1)}
                >
                  Suivant
                </Button>
              </div>
            )}
          </div>
        )}
      </div>
    </PanelCard>
  )
}
