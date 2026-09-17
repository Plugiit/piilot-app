import { PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useId, useState, type ReactNode } from 'react'
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
import { clientListQuery, peopleQuery, useCreateProject } from '@/features/projects/api'
import { PROJECT_STATUS, PROJECT_STATUS_ORDER, tintOf } from '@/features/projects/format'
import { ServicesPicker } from '@/features/services/tag'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { ProjectStatus } from '@/types/api'

/**
 * Ce que le formulaire exige, et rien de plus.
 *
 * Un projet se cree avec un nom et un client — le reste se complete depuis sa
 * fiche. Demander l'echeance et le budget des la creation ferait un formulaire
 * qu'on abandonne, alors que ces deux valeurs se decouvrent souvent apres.
 */
const schema = z.object({
  name: z.string().trim().min(1, 'Le nom du projet est requis'),
  client_name: z.string().trim().min(1, 'Un projet appartient à un client'),
  status: z.enum(['cadrage', 'production', 'attente', 'livre']),
  hours_sold: z
    .string()
    .trim()
    .refine((value) => value === '' || Number(value) >= 0, 'Un nombre positif est attendu'),
  due_on: z.string().trim(),
  // Vide quand le projet ne releve d'aucune prestation.
  service_ids: z.array(z.string()),
})

type Values = z.infer<typeof schema>

/**
 * Creation d'un projet.
 *
 * Le client se saisit au clavier, avec les clients connus en suggestion : un
 * nom deja pris est reutilise cote serveur, un nom inconnu cree le client. Ce
 * detour evite un ecran de gestion des clients que le CRM apportera.
 */
/**
 * `trigger` permet a la carte pointillee de la liste d'ouvrir ce meme dialogue.
 * Sans lui, le bouton par defaut de la barre d'outils est rendu.
 */
export function NewProjectDialog({ trigger }: { trigger?: ReactNode } = {}) {
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  const listId = useId()

  const { data: clients } = useQuery({ ...clientListQuery(), enabled: open })
  const { data: people } = useQuery({ ...peopleQuery, enabled: open })
  const create = useCreateProject()

  /** Equipe affectee des la creation : cochee ici, elle evite un second passage. */
  const [team, setTeam] = useState<string[]>([])

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: '',
      client_name: '',
      status: 'cadrage',
      hours_sold: '',
      due_on: '',
      service_ids: [],
    },
  })

  function submit(values: Values) {
    create.mutate(
      {
        name: values.name,
        client_name: values.client_name,
        status: values.status,
        hours_sold: values.hours_sold === '' ? 0 : Number(values.hours_sold),
        due_on: values.due_on === '' ? null : values.due_on,
        service_ids: values.service_ids,
        team_ids: team,
      },
      {
        onSuccess: (project) => {
          setOpen(false)
          form.reset()
          setTeam([])
          toast.success(`« ${project.name} » créé`)

          // On enchaine sur le projet cree : c'est ce qu'on venait faire, et
          // le retrouver dans une liste de vingt lignes serait un geste de
          // plus.
          void navigate({ to: '/pm/projets/$id', params: { id: project.id } })
        },
        onError: (error) => {
          // Les erreurs par champ remontent dans le formulaire ; le reste en
          // message general.
          if (error instanceof HttpError && error.status === 422) {
            for (const [field, message] of Object.entries(error.details)) {
              if (field in form.getValues()) {
                form.setError(field as keyof Values, { message: String(message) })
              }
            }

            if (error.details.client !== undefined) {
              form.setError('client_name', { message: String(error.details.client) })
            }

            return
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
          <Button size="lg" className="ml-auto gap-1.5">
            <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
            Nouveau projet
          </Button>
        )}
      </DialogTrigger>

      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Nouveau projet</DialogTitle>
          <DialogDescription>
            Le nom et le client suffisent. Tout le reste se complète depuis la fiche du projet.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form className="flex flex-col gap-4" onSubmit={form.handleSubmit(submit)} noValidate>
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Nom du projet</FormLabel>
                  <FormControl>
                    <Input autoFocus placeholder="Refonte du site vitrine" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="client_name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Client</FormLabel>
                  <FormControl>
                    <Input list={listId} placeholder="Nom du client" {...field} />
                  </FormControl>
                  <datalist id={listId}>
                    {(clients?.items ?? []).map((client) => (
                      <option key={client.id} value={client.name} />
                    ))}
                  </datalist>
                  <FormDescription>
                    Un client inconnu est créé avec le projet.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="grid grid-cols-2 gap-4">
              <FormField
                control={form.control}
                name="hours_sold"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Heures vendues</FormLabel>
                    <FormControl>
                      <Input type="number" min={0} step={0.5} placeholder="0" {...field} />
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
                      <Input type="date" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

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
              name="status"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Statut</FormLabel>
                  <div className="flex flex-wrap gap-2">
                    {PROJECT_STATUS_ORDER.map((status) => (
                      <button
                        key={status}
                        type="button"
                        onClick={() => field.onChange(status)}
                        aria-pressed={field.value === status}
                        className={cn(
                          'flex items-center gap-1.5 rounded-[8px] border px-2.5 py-1.5 text-[13px] transition-colors',
                          field.value === status
                            ? 'border-[#111] bg-[#f8f8f8] font-medium text-[#111]'
                            : 'border-[#ebebeb] text-[#777] hover:bg-[#f8f8f8]',
                        )}
                      >
                        <span
                          aria-hidden
                          className="size-1.5 shrink-0 rounded-full"
                          style={{ backgroundColor: PROJECT_STATUS[status as ProjectStatus].color }}
                        />
                        {PROJECT_STATUS[status as ProjectStatus].label}
                      </button>
                    ))}
                  </div>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="flex flex-col gap-2">
              <p className="text-sm font-medium">Équipe</p>

              {(people?.items ?? []).length === 0 ? (
                <p className="text-[13px] text-[#999]">Aucun compte interne à affecter.</p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {(people?.items ?? []).map((person) => {
                    const picked = team.includes(person.id)

                    return (
                      <button
                        key={person.id}
                        type="button"
                        onClick={() =>
                          setTeam((previous) =>
                            picked
                              ? previous.filter((id) => id !== person.id)
                              : [...previous, person.id],
                          )
                        }
                        aria-pressed={picked}
                        className={cn(
                          'flex items-center gap-1.5 rounded-full border py-1 pr-2.5 pl-1 text-[13px] transition-colors',
                          picked
                            ? 'border-[#111] bg-[#f8f8f8] font-medium text-[#111]'
                            : 'border-[#ebebeb] text-[#777] hover:bg-[#f8f8f8]',
                        )}
                      >
                        <span
                          className="flex size-6 shrink-0 items-center justify-center rounded-full text-[10px] font-semibold text-[#111]"
                          style={{ backgroundColor: tintOf(person.id) }}
                        >
                          {person.initials}
                        </span>
                        {`${person.firstname} ${person.lastname}`.trim() || 'Sans nom'}
                      </button>
                    )
                  })}
                </div>
              )}
            </div>

            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
                Annuler
              </Button>
              <Button type="submit" disabled={create.isPending}>
                {create.isPending ? 'Création…' : 'Créer le projet'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
