import { Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Command as CommandPrimitive } from 'cmdk'
import { useState, type ReactNode } from 'react'

import { CHAMP } from '@/components/settings-ui'
import { CommandEmpty, CommandItem, CommandList } from '@/components/ui/command'
import { cn } from '@/lib/utils'

/**
 * Un champ de recherche toujours visible, dont la liste s'ouvre au focus —
 * au clic et a la fleche aussi : apres un choix, le champ garde le focus, et
 * un focus ne se redeclenche pas. La recherche est servie par l'API, par
 * le hook `useItems`, appele a chaque rendu comme tout hook.
 */
export function SearchPicker<T>({
  placeholder,
  useItems,
  keyOf,
  filter,
  render,
  onPick,
}: {
  placeholder: string
  /** Les resultats pour un texte ; `enabled` est faux liste fermee. */
  useItems: (text: string, enabled: boolean) => { items: T[]; fetching: boolean }
  keyOf: (item: T) => string
  /** Ecarte ce qui ne se choisit pas : deja lie, retire du service. */
  filter?: (item: T) => boolean
  render: (item: T) => ReactNode
  onPick: (item: T) => void
}) {
  const [open, setOpen] = useState(false)
  const [text, setText] = useState('')
  const { items, fetching: isFetching } = useItems(text.trim(), open)
  const options = items.filter((item) => filter?.(item) ?? true)

  return (
    <CommandPrimitive shouldFilter={false} loop className="relative" label={placeholder}>
      <HugeiconsIcon
        icon={Search01Icon}
        size={16}
        strokeWidth={1.6}
        className="pointer-events-none absolute top-1/2 left-3 z-10 -translate-y-1/2 text-[#8d8d8d]"
      />
      <CommandPrimitive.Input
        value={text}
        onValueChange={(v) => {
          setText(v)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onClick={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={(e) => {
          if (e.key === 'Escape') setOpen(false)
          if (e.key === 'ArrowDown') setOpen(true)
        }}
        placeholder={placeholder}
        aria-label={placeholder}
        className={cn(CHAMP, 'border pl-9 outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50')}
      />
      {open && (
        <div className="absolute top-full right-0 left-0 z-20 mt-1 rounded-[12px] border border-[#e8e8e9] bg-white p-1 shadow-[0_12px_32px_-12px_rgb(16_24_40/0.25)]">
          <CommandList>
            <CommandEmpty className="py-4 text-[13px] text-[#73757c]">
              {isFetching ? 'Recherche…' : text.trim() === '' ? 'Aucun site à lier.' : 'Aucun site ne correspond.'}
            </CommandEmpty>
            {options.map((item) => (
              <CommandItem
                key={keyOf(item)}
                value={keyOf(item)}
                // Le clic ne doit pas faire perdre le focus au champ avant
                // d'etre pris en compte : la liste se fermerait sous le
                // pointeur.
                onMouseDown={(e) => e.preventDefault()}
                onSelect={() => {
                  onPick(item)
                  setText('')
                  setOpen(false)
                }}
                className="flex items-center gap-2"
              >
                {render(item)}
              </CommandItem>
            ))}
          </CommandList>
        </div>
      )}
    </CommandPrimitive>
  )
}
