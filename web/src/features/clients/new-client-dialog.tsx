import {
  ArrowDown01Icon,
  Building03Icon,
  PlusSignIcon,
  Tick02Icon,
  UserIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'framer-motion'
import { useEffect, useState, type ReactNode } from 'react'
import { useForm, useWatch, type UseFormReturn } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { AutoHeight } from '@/components/auto-height'
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
import { useCreateClient } from '@/features/clients/api'
import { isValidSiret, normalizeSiret } from '@/features/clients/registry'
import { LegalNameCheck, RegistryStatus, useRegistry } from '@/features/clients/registry-status'
import { freeContactsQuery } from '@/features/contacts/api'
import { HttpError } from '@/lib/api'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'
import type { ClientKind } from '@/types/api'

/**
 * Un seul schema pour les deux types de client : les champs de l'un restent
 * en memoire quand on bascule vers l'autre, et reviennent si l'on se ravise.
 * Les exigences propres a chaque type se posent dans `superRefine`.
 */
const schema = z
  .object({
    kind: z.enum(['professionnel', 'particulier']),
    // Professionnel
    siret: z.string(),
    legal_name: z.string().trim(),
    name: z.string().trim(),
    legal_form: z.string().trim(),
    vat_number: z.string().trim(),
    contact_id: z.string(),
    // Particulier
    firstname: z.string().trim(),
    lastname: z.string().trim(),
    email: z.string().trim(),
    phone: z.string().trim(),
    // Communs
    address: z.string().trim(),
    postal_code: z.string().trim(),
    city: z.string().trim(),
    country: z.string().trim(),
  })
  .superRefine((values, ctx) => {
    const issue = (path: string, message: string) => ctx.addIssue({ code: 'custom', path: [path], message })

    if (values.kind === 'particulier') {
      if (values.firstname === '') issue('firstname', 'Le prénom est requis')
      if (values.lastname === '') issue('lastname', 'Le nom est requis')
      if (values.email !== '' && !z.email().safeParse(values.email).success) {
        issue('email', 'Adresse e-mail invalide')
      }
      return
    }

    const siret = normalizeSiret(values.siret)
    if (siret === '') issue('siret', 'Le SIRET est requis pour un professionnel')
    else if (!isValidSiret(siret)) issue('siret', 'SIRET invalide : quatorze chiffres, dont une clé de contrôle')
    if (values.legal_name === '') issue('legal_name', 'La raison sociale est requise')
  })

type Values = z.infer<typeof schema>

const EMPTY: Values = {
  kind: 'professionnel',
  siret: '',
  legal_name: '',
  name: '',
  legal_form: '',
  vat_number: '',
  contact_id: '',
  firstname: '',
  lastname: '',
  email: '',
  phone: '',
  address: '',
  postal_code: '',
  city: '',
  country: 'France',
}

/** Champs que le serveur peut designer dans ses erreurs, vers ceux du formulaire. */
const SERVER_FIELDS: Record<string, keyof Values> = {
  siret: 'siret',
  legal_name: 'legal_name',
  name: 'name',
  firstname: 'firstname',
  lastname: 'lastname',
  email: 'email',
  contact_id: 'contact_id',
}

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
 * Choix du type de client, en deux cartes : le choix decide de tout le reste
 * du formulaire, il merite mieux qu'un menu.
 */
function KindPicker({ value, onChange }: { value: ClientKind; onChange: (kind: ClientKind) => void }) {
  const transition = useSlideTransition()
  const options: { kind: ClientKind; icon: typeof UserIcon; title: string; hint: string }[] = [
    { kind: 'professionnel', icon: Building03Icon, title: 'Professionnel', hint: 'Une entreprise, avec un SIRET' },
    { kind: 'particulier', icon: UserIcon, title: 'Particulier', hint: 'Une personne, sans SIRET' },
  ]

  return (
    <div role="radiogroup" aria-label="Type de client" className="grid grid-cols-2 gap-2">
      {options.map((option) => {
        const selected = option.kind === value

        return (
          <button
            key={option.kind}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(option.kind)}
            className={cn(
              'relative flex cursor-pointer items-start gap-2.5 rounded-[10px] border px-3 py-2.5 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50',
              selected ? 'border-transparent' : 'border-input hover:bg-[#fafafa]',
            )}
          >
            {selected && (
              <motion.span
                layoutId="client-kind"
                transition={transition}
                className="absolute inset-0 rounded-[10px] border border-brand bg-[#fff8f3]"
              />
            )}
            <HugeiconsIcon
              icon={option.icon}
              size={18}
              strokeWidth={1.6}
              className={cn('relative mt-0.5 shrink-0', selected ? 'text-brand' : 'text-[#8d8d8d]')}
            />
            <span className="relative flex min-w-0 flex-col">
              <span className="text-[14px] font-medium">{option.title}</span>
              <span className="text-[12px] text-[#73757c]">{option.hint}</span>
            </span>
          </button>
        )
      })}
    </div>
  )
}

/** Champs d'adresse, communs aux deux types. */
function AddressFields({ form }: { form: UseFormReturn<Values> }) {
  return (
    <div className="flex flex-col gap-3">
      <FormField
        control={form.control}
        name="address"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Adresse</FormLabel>
            <FormControl>
              <Input placeholder="12 rue des Lilas" autoComplete="street-address" {...field} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <div className="grid grid-cols-[110px_1fr_1fr] gap-3">
        <FormField
          control={form.control}
          name="postal_code"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Code postal</FormLabel>
              <FormControl>
                <Input inputMode="numeric" autoComplete="postal-code" placeholder="59000" {...field} />
              </FormControl>
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="city"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Ville</FormLabel>
              <FormControl>
                <Input autoComplete="address-level2" placeholder="Lille" {...field} />
              </FormControl>
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="country"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Pays</FormLabel>
              <FormControl>
                <Input autoComplete="country-name" {...field} />
              </FormControl>
            </FormItem>
          )}
        />
      </div>
    </div>
  )
}

/** Partie propre au professionnel : SIRET, identite au registre, contact. */
function ProfessionalFields({
  form,
  registry,
}: {
  form: UseFormReturn<Values>
  registry: ReturnType<typeof useRegistry>
}) {
  const legalName = useWatch({ control: form.control, name: 'legal_name' })
  const official = registry.state === 'found' && registry.company != null && !registry.company.hidden
    ? registry.company.legalName
    : ''

  return (
    <div className="flex flex-col gap-4">
      <FormField
        control={form.control}
        name="siret"
        render={({ field }) => (
          <FormItem>
            <FormLabel>SIRET</FormLabel>
            <FormControl>
              <Input inputMode="numeric" placeholder="552 032 534 00646" autoComplete="off" {...field} />
            </FormControl>
            <FormMessage />
            <RegistryStatus siret={registry.siret} state={registry.state} company={registry.company} />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="legal_name"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Raison sociale</FormLabel>
            <FormControl>
              <Input placeholder="Telle que déclarée au registre" {...field} />
            </FormControl>
            <FormMessage />
            <LegalNameCheck
              value={legalName}
              official={official}
              onAdopt={(name) => form.setValue('legal_name', name, { shouldValidate: true })}
            />
          </FormItem>
        )}
      />

      <div className="grid grid-cols-2 gap-3">
        <FormField
          control={form.control}
          name="legal_form"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Forme juridique</FormLabel>
              <FormControl>
                <Input placeholder="SAS, SARL…" {...field} />
              </FormControl>
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name="vat_number"
          render={({ field }) => (
            <FormItem>
              <FormLabel>N° de TVA</FormLabel>
              <FormControl>
                <Input placeholder="Déduit du SIRET" {...field} />
              </FormControl>
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={form.control}
        name="name"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Nom d’usage</FormLabel>
            <FormControl>
              <Input placeholder={legalName === '' ? 'Facultatif' : legalName} {...field} />
            </FormControl>
            <FormDescription>Le nom que l’agence emploie, s’il diffère de la raison sociale.</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <AddressFields form={form} />

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
              Seuls les contacts sans entreprise sont proposés : les autres appartiennent déjà à un client.
            </FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  )
}

/** Partie propre au particulier : la personne, qui devient le contact principal. */
function PrivateFields({ form }: { form: UseFormReturn<Values> }) {
  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3">
        <FormField
          control={form.control}
          name="firstname"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Prénom</FormLabel>
              <FormControl>
                <Input autoComplete="given-name" placeholder="Camille" {...field} />
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
                <Input autoComplete="family-name" placeholder="Martin" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={form.control}
        name="email"
        render={({ field }) => (
          <FormItem>
            <FormLabel>E-mail</FormLabel>
            <FormControl>
              <Input type="email" autoComplete="email" placeholder="camille.martin@exemple.fr" {...field} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

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

      <AddressFields form={form} />
    </div>
  )
}

/**
 * Creation d'un client, particulier ou professionnel.
 *
 * Un professionnel s'identifie par son SIRET : le registre des entreprises
 * dit s'il existe, et remplit sa forme juridique et son adresse. La raison
 * sociale saisie est comparee a celle du registre. Un particulier se saisit
 * comme la personne qu'il est, qui devient son contact principal.
 */
export function NewClientDialog({ trigger }: { trigger?: ReactNode } = {}) {
  const [open, setOpen] = useState(false)
  const create = useCreateClient()
  const transition = useSlideTransition()

  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: EMPTY })
  const kind = useWatch({ control: form.control, name: 'kind' })
  const siret = useWatch({ control: form.control, name: 'siret' })
  const registry = useRegistry(siret)
  const { company } = registry

  // Ce que le registre sait remplit la fiche, une fois par etablissement
  // trouve : ce qu'on corrige ensuite a la main n'est pas ecrase. La raison
  // sociale ne se remplit que si elle est vide — sinon, elle se compare.
  useEffect(() => {
    if (company == null) return

    const fill = (field: keyof Values, value: string) => {
      if (value !== '') form.setValue(field, value, { shouldDirty: true })
    }
    if (form.getValues('legal_name') === '' && !company.hidden) {
      fill('legal_name', company.legalName)
      form.clearErrors('legal_name')
    }
    fill('legal_form', company.legalForm)
    fill('vat_number', company.vatNumber)
    fill('address', company.address)
    fill('postal_code', company.postalCode)
    fill('city', company.city)
    fill('country', 'France')
  }, [company, form])

  function close(next: boolean) {
    setOpen(next)
    if (!next) {
      form.reset(EMPTY)
      create.reset()
    }
  }

  function submit(values: Values) {
    // Un SIRET que le registre ne connait pas ne s'inscrit pas : c'est presque
    // toujours une faute de frappe. Un registre injoignable, lui, ne bloque
    // personne — le client s'inscrit sans la date de verification.
    if (values.kind === 'professionnel' && registry.state === 'missing') {
      form.setError('siret', { message: 'Ce SIRET est inconnu du registre' })
      return
    }
    if (values.kind === 'professionnel' && registry.state === 'loading') return

    const common = {
      kind: values.kind,
      address: values.address,
      postal_code: values.postal_code,
      city: values.city,
      country: values.country,
    }

    const body =
      values.kind === 'particulier'
        ? {
            ...common,
            phone: values.phone,
            person: {
              firstname: values.firstname,
              lastname: values.lastname,
              email: values.email,
              phone: values.phone,
            },
          }
        : {
            ...common,
            name: values.name,
            siret: normalizeSiret(values.siret),
            legal_name: values.legal_name,
            legal_form: values.legal_form,
            vat_number: values.vat_number,
            contact_id: values.contact_id === '' ? null : values.contact_id,
            registry_checked: registry.state === 'found',
          }

    create.mutate(body, {
      onSuccess: (client) => {
        close(false)
        toast.success(`« ${client.name} » inscrit`)
      },
      onError: (error) => {
        if (error instanceof HttpError) {
          // Une erreur se dit sur le champ qui la porte, pas dans un bandeau
          // que l'on referme sans savoir quoi corriger.
          if (error.status === 409 || error.status === 422) {
            let placed = false

            for (const [key, message] of Object.entries(error.details)) {
              let field = SERVER_FIELDS[key]
              // Un particulier n'a pas de champ « nom du client » : son nom est
              // celui de la personne.
              if (field === 'name' && values.kind === 'particulier') field = 'lastname'
              if (field === 'name' && values.name === '') field = 'legal_name'
              if (field !== undefined) {
                form.setError(field, { message: String(message) })
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
    })
  }

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogTrigger asChild>
        {trigger ?? (
          <Button size="lg" className="gap-1.5">
            <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
            Nouveau client
          </Button>
        )}
      </DialogTrigger>

      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>Nouveau client</DialogTitle>
          <DialogDescription>
            {kind === 'professionnel'
              ? 'Saisissez le SIRET : le registre des entreprises complète la fiche.'
              : 'Un particulier devient lui-même le contact principal de sa fiche.'}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(submit)} className="flex flex-col gap-4">
            <KindPicker
              value={kind}
              onChange={(next) => {
                form.setValue('kind', next)
                form.clearErrors()
              }}
            />

            <AutoHeight>
              <AnimatePresence mode="popLayout" initial={false}>
                <motion.div
                  key={kind}
                  initial={{ opacity: 0, x: kind === 'particulier' ? 16 : -16 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: kind === 'particulier' ? -16 : 16 }}
                  transition={transition}
                >
                  {kind === 'professionnel' ? (
                    <ProfessionalFields form={form} registry={registry} />
                  ) : (
                    <PrivateFields form={form} />
                  )}
                </motion.div>
              </AnimatePresence>
            </AutoHeight>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => close(false)}>
                Annuler
              </Button>
              <Button
                type="submit"
                disabled={create.isPending || (kind === 'professionnel' && registry.state === 'loading')}
              >
                {create.isPending ? 'Création…' : 'Créer le client'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
