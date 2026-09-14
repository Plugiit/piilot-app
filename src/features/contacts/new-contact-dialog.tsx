import { ArrowDown01Icon, PlusSignIcon, Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { PhoneField } from '@/components/phone-field'
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
import { useCreateContact } from '@/features/contacts/api'
import { clientListQuery } from '@/features/projects/api'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'

/**
 * Le client est facultatif : on rencontre quelqu'un avant de savoir chez qui
 * il travaille, et un contact laisse libre pourra etre adopte plus tard — par
 * la creation d'un client, ou par le menu du contact principal.
 *
 * Il se choisit dans la liste plutot qu'a la saisie, contrairement au
 * formulaire de projet ou un nom inconnu cree le client : ici, ne rien choisir
 * est deja une reponse valable.
 */
const schema = z.object({
  client_id: z.string(),
  firstname: z.string().trim(),
  lastname: z.string().trim(),
  role: z.string().trim(),
  email: z
    .string()
    .trim()
    .refine((value) => value === '' || z.email().safeParse(value).success, {
      message: 'Adresse e-mail invalide',
    }),
  phone: z.string().trim(),
})

type Values = z.infer<typeof schema>

/**
 * Menu deroulant des clients, avec sa recherche.
 *
 * La recherche part au serveur — `shouldFilter={false}` coupe le filtrage
 * interne de cmdk, qui re-filtrerait une liste deja reduite.
 */
function ClientCombobox({
  value,
  onChange,
}: {
  value: string
  onChange: (clientId: string) => void
}) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')

  const { data } = useQuery({
    ...clientListQuery(search === '' ? undefined : search),
    enabled: open,
  })

  // Le nom du client choisi vient de la meme liste : tant qu'elle n'est pas
  // chargee, le bouton garde son invite plutot que d'afficher un identifiant.
  const clients = data?.items ?? []
  const selected = clients.find((client) => client.id === value)
  const [label, setLabel] = useState('')

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
            {selected?.name ?? (label !== '' ? label : 'Aucun client')}
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
          <CommandInput value={search} onValueChange={setSearch} placeholder="Rechercher un client" />
          <CommandList>
            <CommandEmpty>Aucun client ne correspond.</CommandEmpty>
            <CommandGroup>
              {clients.map((client) => (
                <CommandItem
                  key={client.id}
                  value={client.id}
                  onSelect={() => {
                    onChange(client.id)
                    setLabel(client.name)
                    setOpen(false)
                    setSearch('')
                  }}
                  className="gap-2"
                >
                  <span className="flex-1 truncate">{client.name}</span>
                  {value === client.id && (
                    <HugeiconsIcon
                      icon={Tick02Icon}
                      size={16}
                      strokeWidth={2}
                      className="shrink-0 text-brand"
                    />
                  )}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

/** Creation d'un contact. */
export function NewContactDialog({ trigger }: { trigger?: ReactNode } = {}) {
  const [open, setOpen] = useState(false)
  const create = useCreateContact()

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { client_id: '', firstname: '', lastname: '', role: '', email: '', phone: '' },
  })

  function submit(values: Values) {
    if (values.firstname === '' && values.lastname === '') {
      form.setError('firstname', { message: 'Un prénom ou un nom est requis' })

      return
    }

    create.mutate(
      { ...values, client_id: values.client_id === '' ? undefined : values.client_id },
      {
        onSuccess: (contact) => {
          setOpen(false)
          form.reset()
          toast.success(`${contact.firstname} ${contact.lastname}`.trim() + ' inscrit')
        },
        onError: (error) => {
          if (error instanceof HttpError && error.status === 422) {
            let placed = false

            for (const [field, message] of Object.entries(error.details)) {
              if (field in form.getValues()) {
                form.setError(field as keyof Values, { message: String(message) })
                placed = true
              }
            }

            if (placed) return
          }

          toast.error(error instanceof HttpError ? error.message : 'Création impossible')
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
            Nouveau contact
          </Button>
        )}
      </DialogTrigger>

      <DialogContent>
        <DialogHeader>
          <DialogTitle>Nouveau contact</DialogTitle>
          <DialogDescription>
            Un prénom ou un nom suffit. Le premier contact d'un client devient son
            interlocuteur principal.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(submit)} className="flex flex-col gap-4">
            <FormField
              control={form.control}
              name="client_id"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Client</FormLabel>
                  <FormControl>
                    <ClientCombobox value={field.value} onChange={field.onChange} />
                  </FormControl>
                  <FormDescription>
                    Facultatif. Sans client, le contact reste disponible et pourra être rattaché
                    plus tard.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                control={form.control}
                name="firstname"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Prénom</FormLabel>
                    <FormControl>
                      <Input placeholder="Claire" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="lastname"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Nom</FormLabel>
                    <FormControl>
                      <Input placeholder="Fontaine" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name="role"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Rôle</FormLabel>
                  <FormControl>
                    <Input placeholder="Directrice marketing" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>E-mail</FormLabel>
                    <FormControl>
                      <Input type="email" placeholder="claire@exemple.fr" {...field} />
                    </FormControl>
                    <FormDescription>Servira à l'accès au portail.</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Meme composant que les reglages du compte : indicatif choisi
                  a part, prefixe national retire, chiffres espaces selon le
                  pays. C'est la forme internationale compacte qui part au
                  serveur. */}
              <FormField
                control={form.control}
                name="phone"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Téléphone</FormLabel>
                    <FormControl>
                      <PhoneField value={field.value} onChange={field.onChange} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                Annuler
              </Button>
              <Button type="submit" disabled={create.isPending}>
                {create.isPending ? 'Création…' : 'Créer le contact'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
