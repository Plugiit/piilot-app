import {
  Delete02Icon,
  LinkSquare02Icon,
  MoreHorizontalIcon,
  PencilEdit02Icon,
  PlusSignIcon,
  RefreshIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useRef, useState } from 'react'
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
import {
  useCreateSidebarApp,
  useDeleteSidebarApp,
  useDeleteSidebarAppLogo,
  useRefetchSidebarAppLogo,
  useUpdateSidebarApp,
  useUploadSidebarAppLogo,
} from '@/features/sidebar-apps/api'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { SidebarApp } from '@/types/api'

const schema = z.object({
  name: z.string().trim().min(1, 'Le nom est requis'),
  url: z
    .string()
    .trim()
    .refine(
      (value) => value.startsWith('http://') || value.startsWith('https://'),
      'Une adresse commençant par http:// ou https:// est attendue',
    ),
  color: z.string(),
})

type Values = z.infer<typeof schema>

/** Teintes de la pastille affichee a defaut de logo. */
const COLORS = ['#73757c', '#1a73e8', '#8b5cf6', '#059669', '#e57000', '#a30f2c', '#0a6c9a']

const VIERGE: Values = { name: '', url: '', color: COLORS[0]! }

/**
 * Vignette d'une app : son logo, ou sa pastille a l'initiale.
 *
 * Le rail ne se troue jamais — une app sans logo reste reconnaissable a sa
 * lettre et a sa teinte, comme un compte sans photo.
 */
export function AppMark({ app, size = 32 }: { app: SidebarApp; size?: number }) {
  if (app.logo_url === null) {
    return (
      <span
        aria-hidden
        className="flex shrink-0 items-center justify-center rounded-[8px] font-medium text-white"
        style={{ width: size, height: size, backgroundColor: app.color, fontSize: size * 0.45 }}
      >
        {app.name.slice(0, 1).toUpperCase()}
      </span>
    )
  }

  return (
    <span
      className="flex shrink-0 items-center justify-center"
      style={{ width: size, height: size }}
    >
      <img src={app.logo_url} alt="" className="size-full object-contain" />
    </span>
  )
}

/** Les champs, partages par la creation et la modification. */
function AppFields({ form }: { form: UseFormReturn<Values> }) {
  return (
    <>
      <FormField
        control={form.control}
        name="name"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Nom</FormLabel>
            <FormControl>
              <Input autoFocus placeholder="Google Drive" {...field} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="url"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Lien</FormLabel>
            <FormControl>
              <Input placeholder="https://drive.google.com" {...field} />
            </FormControl>
            <FormDescription>Le rail l’ouvre dans un nouvel onglet.</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="color"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Couleur de repli</FormLabel>
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
            <FormDescription>Utilisée tant qu’aucun logo n’est déposé.</FormDescription>
            <FormMessage />
          </FormItem>
        )}
      />
    </>
  )
}

/** Ajoute une app au rail. */
export function NewAppDialog() {
  const [open, setOpen] = useState(false)
  const create = useCreateSidebarApp()
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
          Ajouter une app
        </Button>
      </DialogTrigger>

      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>Ajouter une app</DialogTitle>
          <DialogDescription>
            Elle se place au bout du rail. Son logo se dépose ensuite.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form
            className="flex flex-col gap-4"
            noValidate
            onSubmit={form.handleSubmit((values) =>
              create.mutate(values, {
                onSuccess: (app) => {
                  toast.success(`« ${app.name} » ajoutée`)
                  setOpen(false)
                  form.reset(VIERGE)
                },
                onError: (error) => reportError(error, form, 'Ajout impossible'),
              }),
            )}
          >
            <AppFields form={form} />

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

/** Actions d'une ligne : logo, modification, suppression. */
export function AppRowActions({ app }: { app: SidebarApp }) {
  const [editing, setEditing] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const input = useRef<HTMLInputElement>(null)

  const update = useUpdateSidebarApp(app.id)
  const remove = useDeleteSidebarApp(app.id)
  const upload = useUploadSidebarAppLogo(app.id)
  const clearLogo = useDeleteSidebarAppLogo(app.id)
  const refetch = useRefetchSidebarAppLogo(app.id)

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { name: app.name, url: app.url, color: app.color },
  })

  return (
    <>
      {/* Remis a zero apres coup pour que redeposer le meme fichier declenche
          bien un nouvel evenement `change`. */}
      <input
        ref={input}
        type="file"
        hidden
        accept="image/svg+xml,image/png,image/jpeg,image/webp,image/gif"
        onChange={(event) => {
          const file = event.target.files?.[0]
          event.target.value = ''

          if (file === undefined) return

          upload.mutate(file, {
            onSuccess: () => toast.success('Logo déposé'),
            onError: (error) =>
              toast.error(error instanceof HttpError ? error.message : 'Dépôt impossible'),
          })
        }}
      />

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon-sm" aria-label={`Actions sur ${app.name}`}>
            <HugeiconsIcon icon={MoreHorizontalIcon} size={16} strokeWidth={1.8} />
          </Button>
        </DropdownMenuTrigger>

        <DropdownMenuContent align="end" className="w-56">
          <DropdownMenuItem
            onSelect={() => {
              form.reset({ name: app.name, url: app.url, color: app.color })
              setEditing(true)
            }}
          >
            <HugeiconsIcon icon={PencilEdit02Icon} size={16} strokeWidth={1.6} />
            Modifier
          </DropdownMenuItem>

          <DropdownMenuItem disabled={upload.isPending} onSelect={() => input.current?.click()}>
            <HugeiconsIcon icon={LinkSquare02Icon} size={16} strokeWidth={1.6} />
            {app.logo_url === null ? 'Déposer un logo' : 'Remplacer le logo'}
          </DropdownMenuItem>

          {/* Toujours proposee, logo ou non : c'est le geste qui repart du
              site, et on veut pouvoir y revenir apres avoir depose le sien
              comme apres l'avoir retire. */}
          <DropdownMenuItem
            disabled={refetch.isPending}
            onSelect={() =>
              refetch.mutate(undefined, {
                onSuccess: () =>
                  toast.success('Récupération lancée', {
                    description: 'Le logo du site apparaîtra dans un instant.',
                  }),
                onError: (error) =>
                  toast.error(error instanceof HttpError ? error.message : 'Relance impossible'),
              })
            }
          >
            <HugeiconsIcon icon={RefreshIcon} size={16} strokeWidth={1.6} />
            Récupérer le logo du site
          </DropdownMenuItem>

          {app.logo_url !== null && (
            <DropdownMenuItem
              disabled={clearLogo.isPending}
              onSelect={() =>
                clearLogo.mutate(undefined, {
                  onSuccess: () => toast.success('Logo retiré'),
                  onError: (error) =>
                    toast.error(error instanceof HttpError ? error.message : 'Retrait impossible'),
                })
              }
            >
              <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.6} />
              Retirer le logo
            </DropdownMenuItem>
          )}

          <DropdownMenuSeparator />

          <DropdownMenuItem variant="destructive" onSelect={() => setConfirming(true)}>
            <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.6} />
            Retirer du rail
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={editing} onOpenChange={setEditing}>
        <DialogContent className="sm:max-w-[480px]">
          <DialogHeader>
            <DialogTitle>Modifier l’app</DialogTitle>
            <DialogDescription>{app.name}</DialogDescription>
          </DialogHeader>

          <Form {...form}>
            <form
              className="flex flex-col gap-4"
              noValidate
              onSubmit={form.handleSubmit((values) =>
                update.mutate(values, {
                  onSuccess: () => {
                    toast.success('App modifiée')
                    setEditing(false)
                  },
                  onError: (error) => reportError(error, form, 'Modification impossible'),
                }),
              )}
            >
              <AppFields form={form} />

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
            <DialogTitle>Retirer {app.name} du rail ?</DialogTitle>
            <DialogDescription>
              Le raccourci disparaît de la barre latérale, et son logo est effacé.
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
                    toast.success(`« ${app.name} » retirée`)
                    setConfirming(false)
                  },
                  onError: (error) =>
                    toast.error(error instanceof HttpError ? error.message : 'Retrait impossible'),
                })
              }
            >
              {remove.isPending ? 'Retrait…' : 'Retirer'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

/** Porte l'erreur du serveur sur le champ fautif quand il le nomme. */
function reportError(error: unknown, form: UseFormReturn<Values>, fallback: string) {
  if (error instanceof HttpError) {
    for (const champ of ['name', 'url'] as const) {
      const message = error.details[champ]

      if (typeof message === 'string') {
        form.setError(champ, { message })

        return
      }
    }

    toast.error(error.message)

    return
  }

  toast.error(fallback)
}
