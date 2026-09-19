import { ArrowDown01Icon, PlusSignIcon, Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useCreateClient } from '@/features/clients/api'
import { freeContactsQuery } from '@/features/contacts/api'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'

/**
 * Ce que le formulaire exige, et rien de plus.
 *
 * Le contact se choisit parmi ceux qui n'ont pas encore d'entreprise ; en
 * creer un se fait depuis l'ecran Contacts. Un formulaire qui sait a la fois
 * choisir et creer finit par faire les deux mal.
 */
const schema = z.object({
  name: z.string().trim().min(1, 'Le nom du client est requis'),
  contact_id: z.string(),
})

type Values = z.infer<typeof schema>

/**
 * Menu deroulant des contacts libres, avec sa recherche.
 *
 * La recherche part au serveur — `shouldFilter={false}` coupe le filtrage
 * interne de cmdk, qui re-filtrerait une liste deja reduite.
 */
function FreeContactCombobox({
  value,
  onChange,
}: {
  value: string
  onChange: (contactId: string) => void
}) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [label, setLabel] = useState('')

  const { data } = useQuery({ ...freeContactsQuery(search), enabled: open })
  const contacts = data?.items ?? []

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
          role="combobox"
          aria-expanded={open}
          className="flex h-9 w-full cursor-pointer items-center justify-between gap-2 rounded-[8px] border border-input bg-transparent px-3 text-[14px]"
        >
          <span className={cn('truncate', value === '' && 'text-[#8d8d8d]')}>
            {value === '' ? 'Aucun contact' : label}
          </span>
          <HugeiconsIcon
            icon={ArrowDown01Icon}
            size={16}
            strokeWidth={1.6}
            className="shrink-0 text-[#8d8d8d]"
          />
        </button>
      </PopoverTrigger>

      <PopoverContent align="start" className="w-[var(--radix-popover-trigger-width)] p-0">
        <Command shouldFilter={false}>
          <CommandInput
            value={search}
            onValueChange={setSearch}
            placeholder="Rechercher un contact"
          />
          <CommandList>
            <CommandEmpty>
              Aucun contact disponible. Créez-en un depuis Contacts, sans lui donner de client.
            </CommandEmpty>
            <CommandGroup>
              {contacts.map((contact) => {
                const name = `${contact.firstname} ${contact.lastname}`.trim()

                return (
                  <CommandItem
                    key={contact.id}
                    value={contact.id}
                    onSelect={() => {
                      onChange(contact.id)
                      setLabel(name)
                      setOpen(false)
                      setSearch('')
                    }}
                    className="gap-2"
                  >
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate">{name}</span>
                      {contact.role !== '' && (
                        <span className="truncate text-[12px] text-[#73757c]">{contact.role}</span>
                      )}
                    </span>
                    {value === contact.id && (
                      <HugeiconsIcon
                        icon={Tick02Icon}
                        size={16}
                        strokeWidth={2}
                        className="shrink-0 text-brand"
                      />
                    )}
                  </CommandItem>
                )
              })}

              {value !== '' && (
                <CommandItem
                  value="__aucun"
                  onSelect={() => {
                    onChange('')
                    setLabel('')
                    setOpen(false)
                    setSearch('')
                  }}
                  className="text-[#73757c]"
                >
                  Ne rattacher personne
                </CommandItem>
              )}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

/**
 * Creation d'un client.
 *
 * Rien ne suit la creation : contrairement a un projet, un client n'a pas de
 * fiche ou enchainer. Le dialogue se ferme et la ligne apparait dans la liste.
 */
export function NewClientDialog({ trigger }: { trigger?: ReactNode } = {}) {
  const [open, setOpen] = useState(false)
  const create = useCreateClient()

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', contact_id: '' },
  })

  function submit(values: Values) {
    create.mutate(
      {
        name: values.name,
        contact_id: values.contact_id === '' ? null : values.contact_id,
      },
      {
        onSuccess: (client) => {
          setOpen(false)
          form.reset()
          toast.success(`« ${client.name} » inscrit`)
        },
        onError: (error) => {
          if (error instanceof HttpError) {
            // Un nom deja pris se dit sur le champ qui le porte, pas dans un
            // bandeau que l'on referme sans savoir quoi corriger.
            if (error.status === 409 || error.status === 422) {
              let placed = false

              for (const [field, message] of Object.entries(error.details)) {
                if (field in form.getValues()) {
                  form.setError(field as keyof Values, { message: String(message) })
                  placed = true
                }
              }

              if (placed) return
            }

            toast.error(error.message)

            return
          }

          toast.error('Création impossible')
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger ?? (
          <Button size="lg" className="gap-1.5">
            <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
            Nouveau client
          </Button>
        )}
      </DialogTrigger>

      <DialogContent>
        <DialogHeader>
          <DialogTitle>Nouveau client</DialogTitle>
          <DialogDescription>
            Le nom suffit. Un contact sans entreprise peut lui être rattaché dans la foulée.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(submit)} className="flex flex-col gap-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Nom du client</FormLabel>
                  <FormControl>
                    <Input placeholder="Maison Aubert" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="contact_id"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Contact principal</FormLabel>
                  <FormControl>
                    <FreeContactCombobox value={field.value} onChange={field.onChange} />
                  </FormControl>
                  <FormDescription>
                    Seuls les contacts sans entreprise sont proposés : les autres appartiennent
                    déjà à un client.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                Annuler
              </Button>
              <Button type="submit" disabled={create.isPending}>
                {create.isPending ? 'Création…' : 'Créer le client'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
