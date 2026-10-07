import { useQuery } from '@tanstack/react-query'
import { useDeferredValue, useEffect, useId, useRef, useState, type ComponentProps } from 'react'

import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

/** Une adresse telle que la Base Adresse Nationale la decoupe. */
export interface AddressParts {
  address: string
  postalCode: string
  city: string
}

interface Feature {
  properties: { label: string; name: string; postcode: string; city: string; type: string }
}

/**
 * Cherche une adresse dans la Base Adresse Nationale (api-adresse.data.gouv.fr).
 * Gratuite, sans cle, ouverte aux appels du navigateur : c'est l'ecran qui
 * l'interroge, jamais le serveur. France seulement — ailleurs, on tape.
 */
async function searchAddress(q: string, signal: AbortSignal): Promise<AddressParts[]> {
  const url = `https://api-adresse.data.gouv.fr/search/?q=${encodeURIComponent(q)}&limit=5&autocomplete=1`
  const res = await fetch(url, { signal })
  if (!res.ok) return []
  const body = (await res.json()) as { features?: Feature[] }

  return (body.features ?? [])
    .filter((f) => f.properties.type === 'housenumber' || f.properties.type === 'street')
    .map((f) => ({ address: f.properties.name, postalCode: f.properties.postcode, city: f.properties.city }))
}

/**
 * Champ d'adresse avec suggestions : on tape « 12 rue des li », on choisit,
 * et le code postal et la ville se remplissent avec. Le champ reste un champ
 * texte ordinaire : une adresse que la base ne connait pas se tape en entier.
 */
export function AddressInput({
  value,
  onChange,
  onPick,
  className,
  ...props
}: Omit<ComponentProps<typeof Input>, 'value' | 'onChange'> & {
  value: string
  onChange: (value: string) => void
  onPick: (parts: AddressParts) => void
}) {
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const deferred = useDeferredValue(value.trim())
  const listId = useId()
  const root = useRef<HTMLDivElement>(null)

  const { data: suggestions = [] } = useQuery({
    queryKey: ['address', deferred] as const,
    queryFn: ({ signal }) => searchAddress(deferred, signal),
    enabled: open && deferred.length >= 4,
    staleTime: 5 * 60_000,
    retry: false,
    placeholderData: (previous) => previous,
  })

  useEffect(() => setActive(0), [suggestions])

  function pick(parts: AddressParts) {
    onPick(parts)
    setOpen(false)
  }

  const visible = open && deferred.length >= 4 && suggestions.length > 0

  return (
    <div ref={root} className="relative">
      <Input
        {...props}
        value={value}
        onChange={(event) => {
          onChange(event.target.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onBlur={(event) => {
          // Un clic sur une suggestion ne doit pas la faire disparaitre avant
          // d'etre pris.
          if (!root.current?.contains(event.relatedTarget as Node | null)) setOpen(false)
        }}
        onKeyDown={(event) => {
          if (!visible) return
          if (event.key === 'ArrowDown') {
            event.preventDefault()
            setActive((i) => Math.min(i + 1, suggestions.length - 1))
          } else if (event.key === 'ArrowUp') {
            event.preventDefault()
            setActive((i) => Math.max(i - 1, 0))
          } else if (event.key === 'Enter') {
            event.preventDefault()
            pick(suggestions[active]!)
          } else if (event.key === 'Escape') {
            event.stopPropagation()
            setOpen(false)
          }
        }}
        role="combobox"
        aria-expanded={visible}
        aria-controls={listId}
        aria-autocomplete="list"
        autoComplete="off"
        className={className}
      />

      {visible && (
        <ul
          id={listId}
          role="listbox"
          className="absolute top-full right-0 left-0 z-20 mt-1 overflow-hidden rounded-[10px] border border-[#e8e8e9] bg-white py-1 shadow-[0_8px_24px_-8px_rgb(16_24_40/0.18)]"
        >
          {suggestions.map((s, i) => (
            <li key={`${s.address}-${s.postalCode}`} role="option" aria-selected={i === active}>
              <button
                type="button"
                tabIndex={-1}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => pick(s)}
                onMouseEnter={() => setActive(i)}
                className={cn(
                  'flex w-full cursor-pointer items-baseline gap-2 px-3 py-1.5 text-left text-[13px]',
                  i === active ? 'bg-[#f3f4f4]' : 'bg-white',
                )}
              >
                <span className="min-w-0 flex-1 truncate text-[#1b1b1b]">{s.address}</span>
                <span className="shrink-0 text-[12px] text-[#73757c]">
                  {s.postalCode} {s.city}
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
