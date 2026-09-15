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
import { peopleQuery, projectListQuery } from '@/features/projects/api'
import { useCreateTicket } from '@/features/tickets/api'
import {
  TICKET_PRIORITY,
  TICKET_PRIORITY_ORDER,
  TICKET_TRACKER,
  TICKET_TRACKER_ORDER,
} from '@/features/tickets/format'
import { HttpError } from '@/lib/api'
import type { Person } from '@/types/api'

/**
 * Depot d'un ticket.
 *
 * Pas de champ « statut » : un ticket nait dans le backlog, et le laisser
 * choisir ferait deposer des demandes deja terminees. Il se deplacera ensuite.
 *
 * L'assignation est facultative — un ticket sans destinataire reste a prendre,
 * ce qui est un etat normal. Mais l'ecran ne montrant que SES tickets, un
 * ticket depose sans assigne disparaitrait aussitot : le formulaire le dit
 * plutot que de laisser croire a une perte.
 */
const formSchema = z.object({
  project_id: z.string().min(1, 'Le projet est requis'),
  subject: z.string().trim().min(1, 'Le sujet est requis'),
  description: z.string(),
  tracker: z.enum(['anomalie', 'evolution', 'assistance']),
  priority: z.enum(['low', 'normal', 'high', 'urgent', 'critical']),
  assignee_id: z.string(),
})

type FormValues = z.infer<typeof formSchema>

export function NewTicketDialog() {
  const [open, setOpen] = useState(false)

  // Les deux listes ne partent qu'a l'ouverture : l'ecran des tickets n'a
  // besoin ni des projets ni des comptes pour s'afficher.
  const { data: projects } = useQuery({
    ...projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }),
    enabled: open,
  })
  const { data: people } = useQuery({ ...peopleQuery, enabled: open })

  const create = useCreateTicket()

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      project_id: '',
      subject: '',
      description: '',
      tracker: 'anomalie',
      priority: 'normal',
      assignee_id: '',
    },
  })

  function onSubmit(values: FormValues) {
    create.mutate(
      {
        ...values,
        description: values.description.trim(),
        assignee_id: values.assignee_id === '' ? null : values.assignee_id,
      },
      {
        onSuccess: (ticket) => {
          toast.success(`Ticket #${ticket.numero} déposé`)
          setOpen(false)
          form.reset()
        },
        onError: (error) => {
          toast.error(error instanceof HttpError ? error.message : 'Dépôt impossible')
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {/* `lg` : le declencheur vit sur la rangee des onglets, qu'il ne doit
            pas rehausser sous peine d'en decoller le filet actif. */}
        <Button size="lg" className="gap-1.5">
          <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
          Déposer un ticket
        </Button>
      </DialogTrigger>

      <DialogContent className="sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>Déposer un ticket</DialogTitle>
          <DialogDescription>
            Le ticket entre dans le backlog. Son numéro lui est attribué au dépôt.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form className="flex flex-col gap-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
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

            <FormField
              control={form.control}
              name="subject"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Sujet</FormLabel>
                  <FormControl>
                    <Input autoFocus placeholder="Ce que le ticket demande" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="grid gap-4 sm:grid-cols-2">
              <FormField
                control={form.control}
                name="tracker"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Tracker</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {TICKET_TRACKER_ORDER.map((tracker) => (
                          <SelectItem key={tracker} value={tracker}>
                            {TICKET_TRACKER[tracker]!.label}
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
                name="priority"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Priorité</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {TICKET_PRIORITY_ORDER.map((priority) => (
                          <SelectItem key={priority} value={priority}>
                            {TICKET_PRIORITY[priority]!.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name="assignee_id"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Assigné à</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger className="w-full">
                        <SelectValue placeholder="Personne pour l’instant" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {(people?.items ?? []).map((person: Person) => (
                        <SelectItem key={person.id} value={person.id}>
                          {`${person.firstname} ${person.lastname}`.trim() || 'Sans nom'}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    Cet écran ne montre que vos tickets : un ticket confié à quelqu’un d’autre n’y
                    apparaîtra pas.
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
                    <Textarea rows={4} placeholder="Facultatif" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                Annuler
              </Button>
              <Button type="submit" disabled={create.isPending}>
                {create.isPending ? 'Dépôt…' : 'Déposer'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
