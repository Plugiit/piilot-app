import { ArrowDown01Icon, Building03Icon, PlusSignIcon, Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useDeferredValue, useState } from 'react'

import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { clientListQuery } from '@/features/projects/api'
import { cn } from '@/lib/utils'

/**
 * Client choisi : un client existant (`id`), ou un nouveau, a creer avec le
 * projet (`id` nul, `name` seul).
 */
export interface ClientChoice {
  id: string | null
  name: string
}

/**
 * Liste deroulante des clients, avec recherche.
 *
 * La recherche part au serveur : la liste est paginee, et une agence peut
 * compter plus de clients qu'une page n'en montre. Quand le nom tape ne
 * correspond a aucun client, la liste propose de le creer — le projet
 * l'emporte alors avec lui, comme le faisait la saisie libre.
 */
export function ClientSelect({
  value,
  onChange,
  invalid = false,
  id,
}: {
  value: ClientChoice | null
  onChange: (value: ClientChoice) => void
  invalid?: boolean
  id?: string
}) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const deferred = useDeferredValue(search.trim())

  const { data, isPending } = useQuery({
    ...clientListQuery(deferred === '' ? undefined : deferred),
    enabled: open,
  })
  const clients = data?.items ?? []
  const exact = clients.some((client) => client.name.toLowerCase() === deferred.toLowerCase())

  function pick(choice: ClientChoice) {
    onChange(choice)
    setOpen(false)
    setSearch('')
  }

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) setSearch('')
      }}
    >
      <PopoverTrigger asChild>
        <button
          id={id}
          type="button"
          role="combobox"
          aria-expanded={open}
          aria-invalid={invalid}
          className={cn(
            'flex h-9 w-full cursor-pointer items-center gap-2 rounded-[8px] border border-input bg-transparent px-3 text-left text-[14px] outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50',
            invalid && 'border-destructive',
          )}
        >
          <HugeiconsIcon icon={Building03Icon} size={15} strokeWidth={1.6} className="shrink-0 text-[#8d8d8d]" />
          <span className={cn('min-w-0 flex-1 truncate', value === null && 'text-[#8d8d8d]')}>
            {value === null ? 'Choisir un client' : value.name}
          </span>
          {value !== null && value.id === null && (
            <span className="shrink-0 rounded-full bg-[#fff2ea] px-2 py-0.5 text-[11px] text-[#b84a0c]">nouveau</span>
          )}
          <HugeiconsIcon icon={ArrowDown01Icon} size={16} strokeWidth={1.6} className="shrink-0 text-[#8d8d8d]" />
        </button>
      </PopoverTrigger>

      <PopoverContent align="start" className="w-[var(--radix-popover-trigger-width)] p-0">
        <Command shouldFilter={false}>
          <CommandInput value={search} onValueChange={setSearch} placeholder="Rechercher un client" />
          <CommandList className="max-h-[260px]">
            {!isPending && deferred === '' && <CommandEmpty>Aucun client pour l’instant.</CommandEmpty>}
            <CommandGroup>
              {clients.map((client) => (
                <CommandItem
                  key={client.id}
                  value={client.id}
                  onSelect={() => pick({ id: client.id, name: client.name })}
                  className="gap-2"
                >
                  <span className="flex-1 truncate">{client.name}</span>
                  {client.contact_name !== '' && (
                    <span className="truncate text-[12px] text-[#a2a3a7]">{client.contact_name}</span>
                  )}
                  {value?.id === client.id && (
                    <HugeiconsIcon icon={Tick02Icon} size={16} strokeWidth={2} className="shrink-0 text-brand" />
                  )}
                </CommandItem>
              ))}

              {deferred !== '' && !exact && !isPending && (
                <CommandItem
                  value={`creer:${deferred}`}
                  onSelect={() => pick({ id: null, name: deferred })}
                  className="gap-2"
                >
                  <HugeiconsIcon icon={PlusSignIcon} size={14} strokeWidth={2} className="shrink-0 text-brand" />
                  <span className="truncate">
                    Créer le client « <strong className="font-medium">{deferred}</strong> »
                  </span>
                </CommandItem>
              )}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
