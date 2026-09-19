import { ArrowDown01Icon, Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'

import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { contactsOfClientQuery, useSetPrimaryContact } from '@/features/contacts/api'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { ContactOption, ContactRef } from '@/types/api'

/** Nom affichable d'un contact, l'un des deux champs pouvant etre vide. */
export function contactName(contact: ContactRef | ContactOption): string {
  return `${contact.firstname} ${contact.lastname}`.trim()
}

/**
 * Designation de l'interlocuteur principal d'un client.
 *
 * Le menu propose les contacts du client, puis ceux qui n'ont pas encore
 * d'entreprise — choisir l'un de ces derniers le rattache, ce que l'ecran
 * annonce avant le clic. Les contacts d'un AUTRE client restent exclus : la
 * cle etrangere composite les refuserait.
 *
 * La recherche part au serveur — `shouldFilter={false}` coupe le filtrage
 * interne de cmdk, qui sinon re-filtrerait une liste deja reduite.
 */
export function PrimaryContactPicker({
  clientId,
  current,
  contactsCount,
}: {
  clientId: string
  current: ContactRef | null
  contactsCount: number
}) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')

  // La requete n'est lancee qu'a l'ouverture : une page de vingt-cinq clients
  // ferait autrement vingt-cinq appels que personne n'a demandes.
  const { data } = useQuery({ ...contactsOfClientQuery(clientId, search), enabled: open })
  const set = useSetPrimaryContact()

  const contacts = data?.items ?? []

  function choose(contactId: string | null) {
    set.mutate(
      { clientId, contactId },
      {
        onSuccess: () => {
          setOpen(false)
          setSearch('')
        },
        onError: (error) => {
          toast.error(error instanceof HttpError ? error.message : 'Désignation impossible')
        },
      },
    )
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
          type="button"
          aria-label={
            current === null
              ? 'Désigner un contact principal'
              : `Contact principal : ${contactName(current)}`
          }
          className="flex min-w-0 cursor-pointer items-center gap-1.5 rounded-[8px] px-1.5 py-1 text-left hover:bg-[#f3f4f4]"
        >
          {current === null ? (
            <span className="text-[14px] text-[#a2a3a7]">Aucun</span>
          ) : (
            <span className="flex min-w-0 flex-col">
              <span className="truncate text-[14px] text-[#1b1b1b]">{contactName(current)}</span>
              {current.role !== '' && (
                <span className="truncate text-[12px] text-[#73757c]">{current.role}</span>
              )}
            </span>
          )}
          <HugeiconsIcon
            icon={ArrowDown01Icon}
            size={14}
            strokeWidth={1.6}
            className="shrink-0 text-[#8d8d8d]"
          />
        </button>
      </PopoverTrigger>

      <PopoverContent align="start" className="w-[280px] p-0">
        <Command shouldFilter={false}>
          <CommandInput
            value={search}
            onValueChange={setSearch}
            placeholder="Rechercher un contact"
          />
          <CommandList>
            <CommandEmpty>
              {contactsCount === 0
                ? 'Aucun contact disponible. Créez-en un depuis Contacts.'
                : 'Aucun contact ne correspond.'}
            </CommandEmpty>

            <CommandGroup>
              {contacts.map((contact) => (
                <CommandItem
                  key={contact.id}
                  value={contact.id}
                  disabled={set.isPending}
                  onSelect={() => choose(contact.id)}
                  className="gap-2"
                >
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="flex items-center gap-1.5">
                      <span className="truncate">{contactName(contact)}</span>
                      {/* Le choisir le rattachera au client : le dire avant le
                          clic, pas apres. */}
                      {contact.is_free && (
                        <span className="shrink-0 rounded-full bg-[#f3f4f4] px-1.5 py-0.5 text-[10px] text-[#73757c]">
                          à rattacher
                        </span>
                      )}
                    </span>
                    {contact.role !== '' && (
                      <span className="truncate text-[12px] text-[#73757c]">{contact.role}</span>
                    )}
                  </span>
                  {current?.id === contact.id && (
                    <HugeiconsIcon
                      icon={Tick02Icon}
                      size={16}
                      strokeWidth={2}
                      className="shrink-0 text-brand"
                    />
                  )}
                </CommandItem>
              ))}

              {/* Retirer la designation sans supprimer personne : un client peut
                  cesser d'avoir un interlocuteur attitre. */}
              {current !== null && (
                <CommandItem
                  value="__retirer"
                  disabled={set.isPending}
                  onSelect={() => choose(null)}
                  className={cn('text-[#73757c]')}
                >
                  Retirer la désignation
                </CommandItem>
              )}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
