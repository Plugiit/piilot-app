import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { PhoneField } from '@/components/phone-field'
import { Button } from '@/components/ui/button'
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
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useUpdateClient } from '@/features/clients/api'
import { CLIENT_STATUS, CLIENT_STATUS_ORDER } from '@/features/clients/format'
import { peopleQuery } from '@/features/projects/api'
import { HttpError } from '@/lib/api'
import type { CrmClientDetail, Person } from '@/types/api'

/**
 * Fiche modifiable d'un client.
 *
 * Le contact principal n'y est pas : il se choisit depuis l'en-tete de la
 * fiche, ou la cle etrangere composite impose de rattacher avant de designer.
 *
 * `NON_ASSIGNE` sert de valeur au menu du chargé de compte : un Select ne peut
 * pas porter la chaine vide, qui vaut « aucune selection » pour Radix.
 */
const NON_ASSIGNE = '__aucun'

const schema = z.object({
  name: z.string().trim().min(1, 'Le nom du client est requis'),
  status: z.enum(['lead', 'devis', 'actif', 'veille', 'perdu']),
  account_manager_id: z.string(),
  website: z.string().trim(),
  phone: z.string().trim(),
  address: z.string().trim(),
  postal_code: z.string().trim(),
  city: z.string().trim(),
  country: z.string().trim(),
  siret: z.string().trim(),
  vat_number: z.string().trim(),
})

type Values = z.infer<typeof schema>

/**
 * Groupe de champs, avec son intitule.
 *
 * Onze champs d'affilee ne se lisent pas : ranges en trois groupes nommes, ils
 * se parcourent. L'intitule est discret — c'est un repere, pas un titre.
 */
function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <h3 className="font-heading text-[12px] font-medium tracking-[0.04em] text-[#73757c] uppercase">
        {title}
      </h3>
      {children}
    </section>
  )
}

export function EditClientDialog({
  client,
  trigger,
}: {
  client: CrmClientDetail
  trigger: ReactNode
}) {
  const [open, setOpen] = useState(false)
  const update = useUpdateClient()

  const { data: people } = useQuery({ ...peopleQuery, enabled: open })

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: client.name,
      status: client.status,
      account_manager_id: client.account_manager?.id ?? NON_ASSIGNE,
      website: client.website,
      phone: client.phone,
      address: client.address,
      postal_code: client.postal_code,
      city: client.city,
      country: client.country,
      siret: client.siret,
      vat_number: client.vat_number,
    },
  })

  function submit(values: Values) {
    update.mutate(
      {
        id: client.id,
        values: {
          ...values,
          account_manager_id:
            values.account_manager_id === NON_ASSIGNE ? null : values.account_manager_id,
        },
      },
      {
        onSuccess: () => {
          setOpen(false)
          toast.success('Fiche enregistrée')
        },
        onError: (error) => {
          if (error instanceof HttpError) {
            let placed = false

            for (const [field, message] of Object.entries(error.details)) {
              if (field in form.getValues()) {
                form.setError(field as keyof Values, { message: String(message) })
                placed = true
              }
            }

            if (placed) return

            toast.error(error.message)

            return
          }

          toast.error('Enregistrement impossible')
        },
      },
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        // Rouvrir doit repartir de ce qui est en base, pas d'une saisie
        // abandonnee.
        if (next) {
          form.reset({
            name: client.name,
            status: client.status,
            account_manager_id: client.account_manager?.id ?? NON_ASSIGNE,
            website: client.website,
            phone: client.phone,
            address: client.address,
            postal_code: client.postal_code,
            city: client.city,
            country: client.country,
            siret: client.siret,
            vat_number: client.vat_number,
          })
        }
      }}
    >
      <DialogTrigger asChild>{trigger}</DialogTrigger>

      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>Modifier la fiche</DialogTitle>
          <DialogDescription>
            Le contact principal se choisit depuis l'en-tête de la fiche.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(submit)} className="flex flex-col gap-6">
            <Section title="Identité">
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Nom du client</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className="grid gap-3 sm:grid-cols-2">
                <FormField
                  control={form.control}
                  name="status"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Étape</FormLabel>
                      <Select value={field.value} onValueChange={field.onChange}>
                        <FormControl>
                          <SelectTrigger className="w-full">
                            <SelectValue />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {CLIENT_STATUS_ORDER.map((status) => (
                            <SelectItem key={status} value={status}>
                              <span
                                className="size-2 shrink-0 rounded-full"
                                style={{ background: CLIENT_STATUS[status].color }}
                              />
                              {CLIENT_STATUS[status].label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name="account_manager_id"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Chargé de compte</FormLabel>
                      <Select value={field.value} onValueChange={field.onChange}>
                        <FormControl>
                          <SelectTrigger className="w-full">
                            <SelectValue />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectItem value={NON_ASSIGNE}>Non assigné</SelectItem>
                          {(people?.items ?? []).map((person: Person) => (
                            <SelectItem key={person.id} value={person.id}>
                              {`${person.firstname} ${person.lastname}`.trim() || 'Sans nom'}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </Section>

            <Section title="Coordonnées">
              <div className="grid gap-3 sm:grid-cols-2">
                <FormField
                  control={form.control}
                  name="website"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Site web</FormLabel>
                      <FormControl>
                        <Input placeholder="https://exemple.fr" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                {/* Le telephone porte deux controles : il prend une cellule
                    entiere, sans quoi l'indicatif ecraserait le numero. */}
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

              <FormField
                control={form.control}
                name="address"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Adresse</FormLabel>
                    <FormControl>
                      <Input autoComplete="street-address" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Code postal plus etroit que ville et pays : c'est la largeur
                  de ce qu'il contient, et un champ de cinq chiffres aussi large
                  qu'un nom de ville se lit mal. */}
              <div className="grid gap-3 sm:grid-cols-[120px_1fr_1fr]">
                <FormField
                  control={form.control}
                  name="postal_code"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Code postal</FormLabel>
                      <FormControl>
                        <Input inputMode="numeric" {...field} />
                      </FormControl>
                      <FormMessage />
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
                        <Input autoComplete="address-level2" {...field} />
                      </FormControl>
                      <FormMessage />
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
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </Section>

            <Section title="Identifiants légaux">
              <div className="grid gap-3 sm:grid-cols-2">
                <FormField
                  control={form.control}
                  name="siret"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>SIRET</FormLabel>
                      <FormControl>
                        <Input inputMode="numeric" placeholder="123 456 789 00011" {...field} />
                      </FormControl>
                      <FormMessage />
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
                        <Input placeholder="FR12345678900" {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              {/* Sous le groupe et non sous un champ : dans une grille a deux
                  colonnes, une note attachee a une seule cellule allonge celle-ci
                  et decale sa voisine. */}
              <p className="text-[13px] text-[#73757c]">
                Saisis aussi sur l'ancienne plateforme, qui porte la facturation.
              </p>
            </Section>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                Annuler
              </Button>
              <Button type="submit" disabled={update.isPending}>
                {update.isPending ? 'Enregistrement…' : 'Enregistrer'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
