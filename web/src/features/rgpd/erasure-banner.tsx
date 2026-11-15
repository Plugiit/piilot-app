import { Button } from '@/components/ui/button'
import type { RgpdErasureItem } from '@/types/api'

import { useCancelAction } from './actions'
import { formatDay } from './labels'

/**
 * « Suppression prévue le … » : posé sur la fiche ou la ligne du sujet tant
 * que l'effacement est planifié, avec le geste pour l'annuler.
 */
export function ErasureBanner({ erasure, compact = false }: { erasure: RgpdErasureItem; compact?: boolean }) {
  const cancel = useCancelAction()

  return (
    <div
      role="status"
      className={
        compact
          ? 'flex flex-wrap items-center gap-x-2 text-[12px] text-[#a30f2c]'
          : 'flex flex-wrap items-center justify-between gap-2 rounded-[10px] border border-[#f5c2c7] bg-[#fff5f5] px-3 py-2.5 text-[13px] text-[#a30f2c]'
      }
    >
      <span>Suppression prévue le {formatDay(erasure.scheduled_for)}</span>
      <Button type="button" variant={compact ? 'link' : 'outline'} size="sm" className={compact ? 'h-auto p-0 text-[12px]' : undefined} disabled={cancel.pending} onClick={() => cancel.run(erasure)}>
        Annuler
      </Button>
    </div>
  )
}
