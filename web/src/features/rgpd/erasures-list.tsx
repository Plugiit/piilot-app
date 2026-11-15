import { useQuery } from '@tanstack/react-query'

import { Button } from '@/components/ui/button'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'

import { erasuresQuery } from './api'
import { useCancelAction } from './actions'
import { Pager } from './exports-list'
import {
  ERASURE_REASON_LABEL,
  ERASURE_STATUS_LABEL,
  erasureTotals,
  formatDay,
  formatWhen,
  SUBJECT_LABEL,
  type ErasureStatus,
} from './labels'

/** Historique des effacements : un planifié s'annule tant que son échéance n'est pas passée. */
export function ErasuresList({ status, page, onPage }: { status: ErasureStatus | undefined; page: number; onPage: (page: number) => void }) {
  const { data, isError, error } = useQuery(erasuresQuery(status, page))
  const cancel = useCancelAction()

  const total = data?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / (data?.page_size ?? 25)))

  if (isError) return <p className="text-[13px] text-[#e5484d]">{error instanceof HttpError ? error.message : 'Chargement impossible'}</p>

  return (
    <>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[720px] text-left text-[13.5px]">
          <thead className="bg-[#f3f4f4] text-[#73757c]">
            <tr>
              <th className="px-3 py-2.5 font-normal">Demandé le</th>
              <th className="px-3 py-2.5 font-normal">Sujet</th>
              <th className="px-3 py-2.5 font-normal">Motif</th>
              <th className="px-3 py-2.5 font-normal">Statut</th>
              <th className="px-3 py-2.5 font-normal">Échéance</th>
              <th className="px-3 py-2.5 font-normal"><span className="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            {(data?.items ?? []).map((item) => {
              const totals = item.status === 'done' ? erasureTotals(item.summary) : null

              return (
                <tr key={item.id} className="border-b border-[#f3f4f4] align-top">
                  <td className="px-3 py-2.5 whitespace-nowrap text-[#4b4b4f] tabular-nums">{formatWhen(item.created_at)}</td>
                  <td className="max-w-[260px] truncate px-3 py-2.5 text-[#1b1b1b]" title={item.subject_label}>
                    {item.subject_label}
                    <span className="ml-2 text-[12px] text-[#777]">{SUBJECT_LABEL[item.subject_type]}</span>
                  </td>
                  <td className="px-3 py-2.5 text-[#4b4b4f]">{ERASURE_REASON_LABEL[item.reason]}</td>
                  <td className={cn('px-3 py-2.5', item.status === 'failed' ? 'text-[#a30f2c]' : 'text-[#1b1b1b]')}>
                    {ERASURE_STATUS_LABEL[item.status]}
                    {item.status === 'failed' && <span className="block text-[12px]">Le détail est dans les journaux du serveur. Planifiez un nouvel effacement après correction.</span>}
                    {totals !== null && (
                      <span className="block text-[12px] text-[#73757c]">
                        {totals.rows} ligne{totals.rows > 1 ? 's' : ''}, {totals.files} fichier{totals.files > 1 ? 's' : ''}
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-2.5 whitespace-nowrap text-[#4b4b4f]">{item.status === 'done' && item.executed_at !== null ? formatDay(item.executed_at) : formatDay(item.scheduled_for)}</td>
                  <td className="px-3 py-2.5 text-right whitespace-nowrap">
                    {item.status === 'scheduled' && (
                      <Button variant="outline" size="sm" disabled={cancel.pending} onClick={() => cancel.run(item)}>
                        Annuler
                      </Button>
                    )}
                  </td>
                </tr>
              )
            })}
            {data !== undefined && data.items.length === 0 && (
              <tr>
                <td colSpan={6} className="px-3 py-8 text-center text-[#73757c]">
                  {status === undefined ? 'Aucun effacement pour l’instant.' : 'Aucun effacement avec ce statut.'}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <Pager total={total} page={page} pages={pages} noun="effacement" onPage={onPage} />
    </>
  )
}
