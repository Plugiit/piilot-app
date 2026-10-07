import { AddSquareIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useState, type ReactNode } from 'react'
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
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PRIORITY_TONE, TASK_STATUS, TASK_STATUS_ORDER } from '@/features/projects/format'
import { peopleQuery, projectListQuery } from '@/features/projects/api'
import { useCreateTask } from '@/features/tasks/api'
import { ServicesPicker } from '@/features/services/tag'
import { HttpError } from '@/lib/api'
import { sessionQuery } from '@/lib/auth'
import { userInitials } from '@/lib/initials'
import { cn } from '@/lib/utils'
import type { Person, TaskPriority, TaskStatus } from '@/types/api'
import { useCreateIntent } from '@/lib/palette'

/**
 * Le minimum pour qu'une tache existe.
 *
 * Un libelle suffirait, mais statut et priorite sont deux clics et evitent
 * d'avoir a rouvrir la tache aussitot creee pour la ranger.
 */
const schema = z.object({
  // Toujours dans le schema, meme dans une fiche de projet : la route y met
  // l'identifiant en valeur par defaut et masque le champ. Un schema par cas
  // d'usage aurait fait deux validations a tenir.
  project_id: z.string().min(1, 'Le projet est requis'),
  title: z.string().trim().min(1, 'Le libellé est requis'),
  status: z.enum(['todo', 'progress', 'review', 'done']),
  priority: z.enum(['low', 'medium', 'high']),
  // Vide quand la tache ne releve d'aucune prestation : une reunion interne,
  // un correctif d'intendance.
  service_ids: z.array(z.string()),
  due_on: z.string().trim(),
  // Qui s'en charge, des la creation : assigner apres coup, c'est rouvrir la
  // tache. Par defaut, celui qui la cree.
  assignee_ids: z.array(z.string()),
})

type Values = z.infer<typeof schema>

const PRIORITIES: TaskPriority[] = ['low', 'medium', 'high']

/**
 * Creation d'une tache.
 *
 * Deux appelants, deux situations. Dans une fiche de projet, `projectId` est
 * connu : le champ « Projet » ne s'affiche pas, il n'y aurait qu'une reponse
 * possible. Sur l'ecran « Taches » du module, qui traverse les projets, il
 * faut bien demander lequel — une tache n'existe pas hors d'un projet.
 *
 * `trigger` sert au meme dialogue ouvert depuis deux barres differentes.
 */
export function NewTaskDialog({
  projectId,
  trigger,
}: {
  projectId?: string
  trigger?: ReactNode
}) {
  const [open, setOpen] = useState(false)
  // La palette Cmd+K sait creer d'ici sans passer par le bouton.
  useCreateIntent('task', () => setOpen(true))
  // Creer puis enchainer : la fenetre reste ouverte, le libelle se vide, le
  // reste — projet, statut, assignes — est garde pour la suivante.
  const [again, setAgain] = useState(false)
  const { data: me } = useQuery(sessionQuery)
  const { data: people } = useQuery({ ...peopleQuery, enabled: open })
  const { data: projects } = useQuery({
    ...projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }),
    // Rien a charger quand le projet est impose par l'ecran.
    enabled: projectId === undefined,
  })

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      project_id: projectId ?? '',
      title: '',
      status: 'todo',
      priority: 'medium',
      service_ids: [],
      due_on: '',
      assignee_ids: me === undefined ? [] : [me.id],
    },
  })

  // Le compte arrive apres le premier rendu : la valeur par defaut le suit.
  useEffect(() => {
    if (me !== undefined && !form.formState.isDirty && form.getValues('assignee_ids').length === 0) {
      form.setValue('assignee_ids', [me.id])
    }
  }, [me, form])

  // Le projet choisi decide de l'endpoint : le hook suit la valeur du
  // formulaire tant qu'aucun n'est impose.
  const create = useCreateTask(projectId ?? form.watch('project_id'))

  function onSubmit(values: Values) {
    create.mutate(
      {
        title: values.title,
        status: values.status,
        priority: values.priority,
        service_ids: values.service_ids,
        due_on: values.due_on === '' ? null : values.due_on,
        assignee_ids: values.assignee_ids,
      },
      {
        onSuccess: () => {
          toast.success('Tâche créée')
          form.reset({
            project_id: projectId ?? values.project_id,
            title: '',
            status: again ? values.status : 'todo',
            priority: again ? values.priority : 'medium',
            service_ids: again ? values.service_ids : [],
            due_on: '',
            assignee_ids: again ? values.assignee_ids : me === undefined ? [] : [me.id],
          })
          if (again) {
            setAgain(false)
            form.setFocus('title')
          } else {
            setOpen(false)
          }
        },
        onError: (error) => {
          toast.error(error instanceof HttpError ? error.message : 'Création impossible')
        },
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger ?? (
          <button
            type="button"
            className="flex shrink-0 cursor-pointer items-center gap-2 px-3.5 py-2 text-[16px] text-[#1b1b1b] transition-colors hover:text-brand"
          >
            <HugeiconsIcon icon={AddSquareIcon} size={20} strokeWidth={1.6} />
            Créer une tâche
          </button>
        )}
      </DialogTrigger>

      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>Nouvelle tâche</DialogTitle>
          <DialogDescription>
            Le libellé suffit. Le détail se complète depuis la tâche.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
            {projectId === undefined && (
              <FormField
                control={form.control}
                name="project_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Projet</FormLabel>
                    <FormControl>
                      <select
                        {...field}
                        className="h-9 w-full cursor-pointer rounded-[8px] border border-[#e8e8e9] bg-white px-2 text-[13px] text-[#1b1b1b]"
                      >
                        <option value="">Choisir un projet…</option>
                        {(projects?.items ?? []).map((project) => (
                          <option key={project.id} value={project.id}>
                            {project.name}
                          </option>
                        ))}
                      </select>
                    </FormControl>
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
                  <FormLabel>Libellé</FormLabel>
                  <FormControl>
                    <Input {...field} autoFocus placeholder="Créer le parcours utilisateur" />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="status"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Statut</FormLabel>
                  <div className="flex flex-wrap gap-1.5">
                    {TASK_STATUS_ORDER.map((status: TaskStatus) => (
                      <button
                        key={status}
                        type="button"
                        onClick={() => field.onChange(status)}
                        className={cn(
                          'cursor-pointer rounded-full border px-2.5 py-1 text-[12px] transition-colors',
                          field.value === status
                            ? 'border-transparent'
                            : 'border-[#e8e8e9] bg-white text-[#73757c] hover:bg-[#f8f8f8]',
                        )}
                        style={
                          field.value === status
                            ? {
                                backgroundColor: TASK_STATUS[status].pill.bg,
                                borderColor: TASK_STATUS[status].pill.border,
                                color: TASK_STATUS[status].pill.text,
                              }
                            : undefined
                        }
                      >
                        {TASK_STATUS[status].label}
                      </button>
                    ))}
                  </div>
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
                  <div className="flex flex-wrap gap-1.5">
                    {PRIORITIES.map((priority) => (
                      <button
                        key={priority}
                        type="button"
                        onClick={() => field.onChange(priority)}
                        className={cn(
                          'cursor-pointer rounded-full border px-2.5 py-1 text-[12px] transition-colors',
                          field.value === priority
                            ? 'border-transparent text-white'
                            : 'border-[#e8e8e9] bg-white text-[#73757c] hover:bg-[#f8f8f8]',
                        )}
                        style={
                          field.value === priority
                            ? { backgroundColor: PRIORITY_TONE[priority].bg }
                            : undefined
                        }
                      >
                        {PRIORITY_TONE[priority].label}
                      </button>
                    ))}
                  </div>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="service_ids"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Services</FormLabel>
                  <FormControl>
                    <ServicesPicker value={field.value} onChange={field.onChange} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="due_on"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Échéance</FormLabel>
                  <FormControl>
                    <Input {...field} type="date" />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="assignee_ids"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Assignée à</FormLabel>
                  <div className="flex flex-wrap gap-1.5">
                    {(people?.items ?? []).map((person: Person) => {
                      const active = field.value.includes(person.id)
                      const name = `${person.firstname} ${person.lastname}`.trim() || 'Sans nom'
                      return (
                        <button
                          key={person.id}
                          type="button"
                          role="checkbox"
                          aria-checked={active}
                          onClick={() =>
                            field.onChange(
                              active ? field.value.filter((id: string) => id !== person.id) : [...field.value, person.id],
                            )
                          }
                          className={cn(
                            'flex cursor-pointer items-center gap-1.5 rounded-full border py-0.5 pr-2.5 pl-0.5 text-[12px] transition-colors',
                            active ? 'border-brand bg-[#fff2ea] text-[#b84a0c]' : 'border-[#e8e8e9] bg-white text-[#73757c] hover:bg-[#f8f8f8]',
                          )}
                        >
                          <span className={cn('flex size-5 items-center justify-center rounded-full text-[10px] font-medium', active ? 'bg-brand text-white' : 'bg-[#f3f4f4] text-[#73757c]')}>
                            {userInitials({ firstname: person.firstname, lastname: person.lastname, email: '' })}
                          </span>
                          {person.id === me?.id ? 'Moi' : name}
                        </button>
                      )
                    })}
                  </div>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type="button" variant="ghost" size="lg" onClick={() => setOpen(false)}>
                Annuler
              </Button>
              <Button type="submit" size="lg" variant="outline" disabled={create.isPending} onClick={() => setAgain(true)}>
                Créer et ajouter une autre
              </Button>
              <Button type="submit" size="lg" disabled={create.isPending} onClick={() => setAgain(false)}>
                {create.isPending ? 'Création…' : 'Créer la tâche'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
