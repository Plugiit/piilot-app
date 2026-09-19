import { Delete02Icon, MoreHorizontalIcon, PencilEdit02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
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
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { useDeleteContact, useUpdateContact } from '@/features/contacts/api'
import { HttpError } from '@/lib/api'
import type { CrmContact } from '@/types/api'

/** Mêmes règles qu'à la création : un prénom ou un nom suffit. */
const schema = z.object({
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
 * Actions d'une ligne de contact : modifier, supprimer.
 *
 * Le client n'est pas modifiable ici : rattacher quelqu'un passe par la
 * designation du contact principal, seule a savoir tenir la cle etrangere
 * composite dans le bon ordre.
 */
export function ContactRowActions({ contact }: { contact: CrmContact }) {
  const [editing, setEditing] = useState(false)
  const [confirming, setConfirming] = useState(false)

  const update = useUpdateContact()
  const remove = useDeleteContact()

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      firstname: contact.firstname,
      lastname: contact.lastname,
      role: contact.role,
      email: contact.email ?? '',
      phone: contact.phone,
    },
  })

  function submit(values: Values) {
    if (values.firstname === '' && values.lastname === '') {
      form.setError('firstname', { message: 'Un prénom ou un nom est requis' })

      return
    }

    update.mutate(
      { id: contact.id, values },
      {
        onSuccess: () => {
          setEditing(false)
          toast.success('Contact modifié')
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

          toast.error(error instanceof HttpError ? error.message : 'Modification impossible')
        },
      },
    )
  }

  const name = `${contact.firstname} ${contact.lastname}`.trim()

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label={`Actions sur ${name}`}
            className="flex size-7 cursor-pointer items-center justify-center rounded-[8px] hover:bg-[#f3f4f4]"
          >
            <HugeiconsIcon
              icon={MoreHorizontalIcon}
              size={16}
              strokeWidth={1.8}
              className="text-[#73757c]"
            />
          </button>
        </DropdownMenuTrigger>

        <DropdownMenuContent align="end">
          <DropdownMenuItem
            onSelect={() => {
              form.reset({
                firstname: contact.firstname,
                lastname: contact.lastname,
                role: contact.role,
                email: contact.email ?? '',
                phone: contact.phone,
              })
              setEditing(true)
            }}
          >
            <HugeiconsIcon icon={PencilEdit02Icon} size={16} strokeWidth={1.6} />
            Modifier
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          <DropdownMenuItem variant="destructive" onSelect={() => setConfirming(true)}>
            <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.6} />
            Supprimer
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={editing} onOpenChange={setEditing}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Modifier le contact</DialogTitle>
            <DialogDescription>
              Le client ne se change pas ici : il se choisit depuis la colonne « Contact
              principal » du tableau des clients.
            </DialogDescription>
          </DialogHeader>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(submit)} className="flex flex-col gap-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <FormField
                  control={form.control}
                  name="firstname"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Prénom</FormLabel>
                      <FormControl>
                        <Input {...field} />
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
                        <Input {...field} />
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
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>E-mail</FormLabel>
                    <FormControl>
                      <Input type="email" {...field} />
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

              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setEditing(false)}>
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

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Supprimer {name} ?</DialogTitle>
            <DialogDescription>
              {contact.is_primary
                ? 'C’est l’interlocuteur principal de son client : la désignation sera retirée.'
                : 'Le contact ne sera plus proposé nulle part.'}
            </DialogDescription>
          </DialogHeader>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirming(false)}>
              Annuler
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={remove.isPending}
              onClick={() =>
                remove.mutate(contact.id, {
                  onSuccess: () => {
                    setConfirming(false)
                    toast.success(`${name} supprimé`)
                  },
                  onError: (error) => {
                    toast.error(
                      error instanceof HttpError ? error.message : 'Suppression impossible',
                    )
                    setConfirming(false)
                  },
                })
              }
            >
              {remove.isPending ? 'Suppression…' : 'Supprimer'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
