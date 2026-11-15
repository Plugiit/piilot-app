import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { ImportItem, ImportRowPage } from '@/types/api'

import { OUTCOME_LABEL, OUTCOME_ROW_LABEL, reasonLabel, type RowOutcome } from './entities'

const TONE: Record<RowOutcome, string> = {
  created: 'text-[#006f1f]',
  skipped: 'text-[#73757c]',
  error: 'text-[#a30f2c]',
}

const COUNT: Record<RowOutcome, (item: ImportItem) => number> = {
  created: (i) => i.created_n,
  skipped: (i) => i.skipped_n,
  error: (i) => i.error_n,
}

/**
 * Rapport d'un import ou d'une simulation : trois compteurs qui servent de
 * filtres, puis les lignes. Le libellé d'une ligne invalide vient de l'API
 * (« nom : champ : motif ») et s'affiche tel quel.
 */
export function Report({
  item,
  rows,
  outcome,
  page,
  onOutcome,
  onPage,
}: {
  item: ImportItem
  rows: ImportRowPage | undefined
  outcome: RowOutcome | undefined
  page: number
  onOutcome: (outcome: RowOutcome | undefined) => void
  onPage: (page: number) => void
}) {
  const total = rows?.total ?? 0
  const pageSize = rows?.page_size ?? 25
  const pages = Math.max(1, Math.ceil(total / pageSize))

  return (
    <div className="flex flex-col gap-3">
      <div role="group" aria-label="Filtrer par résultat" className="flex flex-wrap gap-2">
        <Button variant={outcome === undefined ? 'default' : 'outline'} size="sm" aria-pressed={outcome === undefined} onClick={() => onOutcome(undefined)}>
          Toutes · {item.created_n + item.skipped_n + item.error_n}
        </Button>
        {(Object.keys(OUTCOME_LABEL) as RowOutcome[]).map((key) => (
          <Button key={key} variant={outcome === key ? 'default' : 'outline'} size="sm" aria-pressed={outcome === key} onClick={() => onOutcome(key)}>
            {OUTCOME_LABEL[key]} · {COUNT[key](item)}
          </Button>
        ))}
      </div>

      <div className="overflow-x-auto">
        <table className="w-full min-w-[560px] text-left text-[13.5px]">
          <thead className="bg-[#f3f4f4] text-[#73757c]">
            <tr>
              <th className="px-3 py-2.5 font-normal">Ligne</th>
              <th className="px-3 py-2.5 font-normal">Résultat</th>
              <th className="px-3 py-2.5 font-normal">Motif</th>
              <th className="px-3 py-2.5 font-normal">Détail</th>
            </tr>
          </thead>
          <tbody>
            {(rows?.items ?? []).map((row) => (
              <tr key={row.line} className="border-b border-[#f3f4f4] align-top">
                <td className="px-3 py-2.5 text-[#4b4b4f] tabular-nums">{row.line}</td>
                <td className={cn('px-3 py-2.5', TONE[row.outcome])}>{OUTCOME_ROW_LABEL[row.outcome]}</td>
                <td className="px-3 py-2.5 text-[#1b1b1b]">{reasonLabel(row.reason)}</td>
                <td className="px-3 py-2.5 break-words text-[#73757c]">{row.label}</td>
              </tr>
            ))}
            {rows !== undefined && rows.items.length === 0 && (
              <tr>
                <td colSpan={4} className="px-3 py-8 text-center text-[#73757c]">
                  Aucune ligne pour ce filtre.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-[12px] text-[#777]">
          {total} ligne{total > 1 ? 's' : ''} · page {page} sur {pages}
        </p>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPage(page - 1)}>
            Précédent
          </Button>
          <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => onPage(page + 1)}>
            Suivant
          </Button>
        </div>
      </div>
    </div>
  )
}
