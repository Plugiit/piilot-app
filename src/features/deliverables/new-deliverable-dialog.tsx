import { PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { useCreateDeliverable } from '@/features/deliverables/api'
import { projectListQuery } from '@/features/projects/api'
import { HttpError } from '@/lib/api'

/**
 * Depot d'un livrable.
 *
 * Le livrable et sa premiere version partent ensemble : un livrable sans rien
 * a montrer n'est pas quelque chose qu'on depose. Il nait donc en attente, et
 * pas de champ « etat » — laisser choisir ferait deposer des livrables deja
 * valides, sans que personne ne les ait valides.
 *
 * Un lien et non un fichier : une agence web livre une preproduction ou une
 * maquette, qui vivent la ou elles sont. Le depot de fichier viendra avec le
 * stockage.
 */
const schema = z.object({
  project_id: z.string().min(1, 'Le projet est requis'),
  title: z.string().trim().min(1, 'Le titre est requis'),
  description: z.string(),
  url: z.string().trim().min(1, 'Le lien est requis'),
})

type Values = z.infer<typeof schema>

export function NewDeliverableDialog({ projectId }: { projectId?: string }) {
  const [open, setOpen] = useState(false)

  const { data: projects } = useQuery({
    ...projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }),
    enabled: open && projectId === undefined,
  })

  const vierge: Values = {
    project_id: projectId ?? '',
    title: '',
    description: '',
    url: '',
  }

  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: vierge })

  // Le projet choisi dans le formulaire porte la mutation : l'endpoint vit
  // sous le projet, c'est lui qui designe ou le livrable se depose.
  const create = useCreateDeliverable(projectId ?? form.watch('project_id'))

  function onSubmit(values: Values) {
    create.mutate(
      {
        title: values.title.trim(),
        description: values.description.trim(),
        url: values.url.trim(),
      },
      {
        onSuccess: (item) => {
          toast.success(`« ${item.title} » déposé`)
          setOpen(false)
          form.reset(vierge)
        },
        onError: (error) =>
          toast.error(error instanceof HttpError ? error.message : 'Dépôt impossible'),
      },
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (next) form.reset(vierge)
      }}
    >
      <DialogTrigger asChild>
        <Button size="lg" className="gap-1.5">
          <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
          Déposer un livrable
        </Button>
      </DialogTrigger>

      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Déposer un livrable</DialogTitle>
          <DialogDescription>
            Il part en attente de validation, dans sa première version.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form className="flex flex-col gap-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
            {projectId === undefined && (
              <FormField
                control={form.control}
                name="project_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Projet</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger className="w-full">
                          <SelectValue placeholder="Choisir un projet" />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {(projects?.items ?? []).map((project) => (
                          <SelectItem key={project.id} value={project.id}>
                            {project.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            <FormField
              control={form.control}
              name="title"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Titre</FormLabel>
                  <FormControl>
                    <Input autoFocus placeholder="Maquette de la page d’accueil" {...field} />
                  </FormControl>
                  <FormDescription>
                    Ce dont on parlera d’une version à l’autre : il ne change pas quand la v2 part.
                  </FormDescription>
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
                    <Input placeholder="https://preprod.exemple.fr" {...field} />
                  </FormControl>
                  <FormDescription>
                    La préproduction, la maquette ou le document à faire valider.
                  </FormDescription>
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

            <DialogFooter>
              <Button type="button" size="lg" variant="outline" onClick={() => setOpen(false)}>
                Annuler
              </Button>
              <Button type="submit" size="lg" disabled={create.isPending}>
                {create.isPending ? 'Dépôt…' : 'Déposer'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
