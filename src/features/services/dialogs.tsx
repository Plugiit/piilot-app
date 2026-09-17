import { Delete02Icon, MoreHorizontalIcon, PencilEdit02Icon, PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm, type UseFormReturn } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
import { Textarea } from '@/components/ui/textarea'
import { useCreateService, useDeleteService, useUpdateService } from '@/features/services/api'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { Service } from '@/types/api'

const schema = z.object({
  name: z.string().trim().min(1, 'Le nom est requis'),
  description: z.string(),
  color: z.string(),
})

type Values = z.infer<typeof schema>

/**
 * Teintes proposees.
 *
 * Une palette fermee plutot qu'un champ libre : ce sont les couleurs de texte
 * des pastilles du reste de l'app, et les laisser saisir a la main ferait
 * apparaitre des services illisibles sur fond clair.
 */
const COLORS = ['#73757c', '#3b45c9', '#7134c9', '#0a6c9a', '#006f1f', '#9a6a00', '#a30f2c']

const VIERGE: Values = { name: '', description: '', color: COLORS[0]! }

/** Les champs, partages par la creation et la modification. */
function ServiceFields({ form }: { form: UseFormReturn<Values> }) {
  return (
    <>
      <FormField
        control={form.control}
        name="name"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Nom</FormLabel>
            <FormControl>
              <Input autoFocus placeholder="Développement web" {...field} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="description"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Description</FormLabel>
            <FormControl>
              <Textarea rows={3} placeholder="Facultatif" {...field} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="color"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Couleur</FormLabel>
            <FormControl>
              <div className="flex flex-wrap gap-2">
                {COLORS.map((color) => (
                  <button
                    key={color}
                    type="button"
                    aria-label={`Teinte ${color}`}
                    aria-pressed={field.value === color}
                    onClick={() => field.onChange(color)}
                    style={{ backgroundColor: color }}
                    className={cn(
                      'size-7 cursor-pointer rounded-full transition-[outline]',
                      field.value === color
                        ? 'outline-2 outline-offset-2 outline-[#1b1b1b]'
                        : 'outline-0',
                    )}
                  />
                ))}
              </div>
            </FormControl>
            <FormDescription>Elle distingue le service d’un coup d’œil dans une liste.</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </>
  )
}

/** Ajoute une prestation au referentiel. */
export function NewServiceDialog() {
  const [open, setOpen] = useState(false)
  const create = useCreateService()
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: VIERGE })

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (next) form.reset(VIERGE)
      }}
    >
      <DialogTrigger asChild>
        <Button size="lg" className="gap-1.5">
          <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
          Ajouter un service
        </Button>
      </DialogTrigger>

      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>Ajouter un service</DialogTitle>
          <DialogDescription>Une prestation que l’agence vend.</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form
            className="flex flex-col gap-4"
            noValidate
            onSubmit={form.handleSubmit((values) =>
              create.mutate(
                { ...values, name: values.name.trim(), description: values.description.trim() },
                {
                  onSuccess: (service) => {
                    toast.success(`« ${service.name} » ajouté`)
                    setOpen(false)
                    form.reset(VIERGE)
                  },
                  onError: (error) => reportError(error, form, 'Ajout impossible'),
                },
              ),
            )}
          >
            <ServiceFields form={form} />

            <DialogFooter>
              <Button type="button" size="lg" variant="outline" onClick={() => setOpen(false)}>
                Annuler
              </Button>
              <Button type="submit" size="lg" disabled={create.isPending}>
                {create.isPending ? 'Ajout…' : 'Ajouter'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}

/** Actions d'une ligne : modifier, supprimer. */
export function ServiceRowActions({ service }: { service: Service }) {
  const [editing, setEditing] = useState(false)
  const [confirming, setConfirming] = useState(false)

  const update = useUpdateService(service.id)
  const remove = useDeleteService(service.id)

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: service.name,
      description: service.description,
      color: service.color,
    },
  })

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon-sm" aria-label={`Actions sur ${service.name}`}>
            <HugeiconsIcon icon={MoreHorizontalIcon} size={16} strokeWidth={1.8} />
          </Button>
        </DropdownMenuTrigger>

        <DropdownMenuContent align="end" className="w-48">
          <DropdownMenuItem
            onSelect={() => {
              // Le formulaire repart de la ligne : le dialogue reste monte
              // d'un service a l'autre, et garderait sinon le precedent.
              form.reset({
                name: service.name,
                description: service.description,
                color: service.color,
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
        <DialogContent className="sm:max-w-[480px]">
          <DialogHeader>
            <DialogTitle>Modifier le service</DialogTitle>
            <DialogDescription>{service.name}</DialogDescription>
          </DialogHeader>

          <Form {...form}>
            <form
              className="flex flex-col gap-4"
              noValidate
              onSubmit={form.handleSubmit((values) =>
                update.mutate(
                  { ...values, name: values.name.trim(), description: values.description.trim() },
                  {
                    onSuccess: () => {
                      toast.success('Service modifié')
                      setEditing(false)
                    },
                    onError: (error) => reportError(error, form, 'Modification impossible'),
                  },
                ),
              )}
            >
              <ServiceFields form={form} />

              <DialogFooter>
                <Button type="button" size="lg" variant="outline" onClick={() => setEditing(false)}>
                  Annuler
                </Button>
                <Button type="submit" size="lg" disabled={update.isPending}>
                  {update.isPending ? 'Enregistrement…' : 'Enregistrer'}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </Dialog>

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>Supprimer {service.name} ?</DialogTitle>
            <DialogDescription>
              Le service quitte le référentiel. Ce qu’il a déjà étiqueté le reste.
            </DialogDescription>
          </DialogHeader>

          <DialogFooter>
            <Button size="lg" variant="outline" onClick={() => setConfirming(false)}>
              Annuler
            </Button>
            <Button
              size="lg"
              variant="destructive"
              disabled={remove.isPending}
              onClick={() =>
                remove.mutate(undefined, {
                  onSuccess: () => {
                    toast.success(`« ${service.name} » supprimé`)
                    setConfirming(false)
                  },
                  onError: (error) =>
                    toast.error(
                      error instanceof HttpError ? error.message : 'Suppression impossible',
                    ),
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

/**
 * Porte l'erreur du serveur sur le champ fautif quand il le nomme.
 *
 * Un nom deja pris revient en 422 avec `details.name` : l'afficher sous le
 * champ evite de faire chercher dans un toast ce qui cloche.
 */
function reportError(error: unknown, form: UseFormReturn<Values>, fallback: string) {
  if (error instanceof HttpError) {
    const nom = error.details.name

    if (typeof nom === 'string') {
      form.setError('name', { message: nom })

      return
    }

    toast.error(error.message)

    return
  }

  toast.error(fallback)
}
