import {
  ArrowDown01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Calendar03Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  PRESETS,
  PRESET_LABELS,
  periodLabel,
  presetOf,
  presetPeriod,
  shiftPeriod,
  type Period,
} from '@/features/time/period'
import { cn } from '@/lib/utils'

/** 366 jours au plus : la borne de l'API, verifiee ici pour eviter un aller-retour. */
const MAX_DAYS = 366

function lengthOf(from: string, to: string) {
  return Math.round((new Date(to).getTime() - new Date(from).getTime()) / 86_400_000) + 1
}

/**
 * Choix de la periode : fleches pour passer a la precedente ou a la suivante,
 * raccourcis et plage libre dans un panneau.
 */
export function PeriodPicker({
  period,
  onChange,
}: {
  period: Period
  onChange: (period: Period) => void
}) {
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState(period)
  const active = presetOf(period)

  const length = lengthOf(draft.from, draft.to)
  const invalid =
    draft.from === '' || draft.to === '' || length < 1
      ? 'La fin précède le début'
      : length > MAX_DAYS
        ? 'Une période ne dépasse pas un an'
        : null

  return (
    <div className="flex items-center gap-1">
      <Button
        variant="outline"
        size="icon-sm"
        aria-label="Période précédente"
        onClick={() => onChange(shiftPeriod(period, -1))}
      >
        <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
      </Button>

      <Popover
        open={open}
        onOpenChange={(next) => {
          setOpen(next)
          if (next) setDraft(period)
        }}
      >
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            size="lg"
            className="min-w-[210px] justify-between gap-2 text-[13px]"
          >
            <span className="flex items-center gap-1.5 text-[#111]">
              <HugeiconsIcon
                icon={Calendar03Icon}
                size={16}
                strokeWidth={1.6}
                className="text-[#64748b]"
              />
              {periodLabel(period)}
            </span>
            <HugeiconsIcon
              icon={ArrowDown01Icon}
              size={14}
              strokeWidth={2}
              className="text-[#8d8d8d]"
            />
          </Button>
        </PopoverTrigger>

        <PopoverContent align="start" className="flex w-[300px] flex-col gap-3 p-3">
          <div className="grid grid-cols-2 gap-1">
            {PRESETS.map((preset) => (
              <button
                key={preset}
                type="button"
                onClick={() => {
                  onChange(presetPeriod(preset))
                  setOpen(false)
                }}
                className={cn(
                  'cursor-pointer rounded-[8px] px-2 py-1.5 text-left text-[13px] transition-colors hover:bg-[#f4f4f4]',
                  active === preset ? 'bg-[#f4f4f4] font-medium text-[#111]' : 'text-[#444]',
                )}
              >
                {PRESET_LABELS[preset]}
              </button>
            ))}
          </div>

          <div className="flex flex-col gap-2 border-t border-[#eee] pt-3">
            <p className="text-[12px] font-medium text-[#64748b]">Période personnalisée</p>
            <div className="flex items-center gap-2">
              <Input
                type="date"
                aria-label="Du"
                value={draft.from}
                onChange={(event) => setDraft({ ...draft, from: event.target.value })}
                className="h-8 text-[13px]"
              />
              <span className="text-[13px] text-[#8d8d8d]">au</span>
              <Input
                type="date"
                aria-label="Au"
                value={draft.to}
                onChange={(event) => setDraft({ ...draft, to: event.target.value })}
                className="h-8 text-[13px]"
              />
            </div>
            {invalid !== null && <p className="text-[12px] text-[#e5484d]">{invalid}</p>}
            <Button
              size="sm"
              disabled={invalid !== null}
              onClick={() => {
                onChange(draft)
                setOpen(false)
              }}
            >
              Appliquer
            </Button>
          </div>
        </PopoverContent>
      </Popover>

      <Button
        variant="outline"
        size="icon-sm"
        aria-label="Période suivante"
        onClick={() => onChange(shiftPeriod(period, 1))}
      >
        <HugeiconsIcon icon={ArrowRight01Icon} size={16} strokeWidth={1.8} />
      </Button>
    </div>
  )
}
