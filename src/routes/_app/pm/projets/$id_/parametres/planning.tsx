import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { Input } from '@/components/ui/input'
import { projectDetailQuery, useUpdateProject } from '@/features/projects/api'
import { PROJECT_STATUS, PROJECT_STATUS_ORDER } from '@/features/projects/format'
import { HttpError } from '@/lib/api'
import type { ProjectDetail, ProjectStatus } from '@/types/api'

import { CHAMP, Card, Choices, Field, SaveBar } from '@/components/settings-ui'

const schema = z
  .object({
    status: z.enum(['cadrage', 'production', 'attente', 'livre']),
    progress: z.number().int().min(0, 'De 0 à 100').max(100, 'De 0 à 100'),
    starts_on: z.string(),
    due_on: z.string(),
  })
  // La base refuse deja une echeance anterieure au demarrage ; le dire ici
  // evite d'envoyer une saisie qu'on sait perdue, et designe le bon champ.
  .refine(
    (values) => values.starts_on === '' || values.due_on === '' || values.due_on >= values.starts_on,
    { path: ['due_on'], message: 'L’échéance précède la date de début' },
  )

type Values = z.infer<typeof schema>

export const Route = createFileRoute('/_app/pm/projets/$id_/parametres/planning')({
  component: PlanningPage,
})

const STATUTS: { value: ProjectStatus; label: string; color: string }[] = PROJECT_STATUS_ORDER.map(
  (value) => ({ value, label: PROJECT_STATUS[value].label, color: PROJECT_STATUS[value].color }),
)

function PlanningPage() {
  const { id } = Route.useParams()
  const { data: project } = useQuery(projectDetailQuery(id))

  if (project === undefined) return null

  return <PlanningForm key={project.id} project={project} />
}

function PlanningForm({ project }: { project: ProjectDetail }) {
  const update = useUpdateProject(project.id)

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      status: project.status,
      progress: project.progress,
      starts_on: project.starts_on ?? '',
      due_on: project.due_on ?? '',
    },
  })

  const errors = form.formState.errors
  const progress = form.watch('progress')

  function onSubmit(values: Values) {
    update.mutate(
      {
        status: values.status,
        progress: values.progress,
        // Une date effacee part a `null` : le handler distingue la cle absente
        // — « ne touche pas » — de la cle nulle, qui vide le champ.
        starts_on: values.starts_on === '' ? null : values.starts_on,
        due_on: values.due_on === '' ? null : values.due_on,
      },
      {
        onSuccess: () => {
          toast.success('Planning enregistré')
          form.reset(values)
        },
        onError: (error) =>
          toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
      },
    )
  }

  return (
    <form noValidate onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-3 p-4">
      <Card title="Étape" description="Où en est le projet dans son cycle de vie.">
        <Field label="Statut">
          <Choices
            value={form.watch('status')}
            options={STATUTS}
            onChange={(value) => form.setValue('status', value, { shouldDirty: true })}
          />
        </Field>

        <Field
          label="Avancement"
          hint="Déclaré par l’équipe. Distinct du rapport des tâches faites, qui se calcule tout seul."
          error={errors.progress?.message}
        >
          <div className="flex items-center gap-4">
            <input
              type="range"
              min={0}
              max={100}
              step={5}
              value={Number.isNaN(progress) ? 0 : progress}
              onChange={(event) =>
                form.setValue('progress', Number(event.target.value), { shouldDirty: true })
              }
              className="accent-brand h-1 flex-1 cursor-pointer"
              aria-label="Avancement"
            />
            <div className="flex w-[110px] shrink-0 items-center gap-2">
              <Input
                {...form.register('progress', { valueAsNumber: true })}
                type="number"
                min={0}
                max={100}
                className={`${CHAMP} text-right tabular-nums`}
              />
              <span className="text-[16px] text-[#73757c]">%</span>
            </div>
          </div>
        </Field>
      </Card>

      <Card title="Dates" description="Laisser vide un projet dont la date n’est pas arrêtée.">
        <Field label="Date de début" error={errors.starts_on?.message}>
          <Input {...form.register('starts_on')} type="date" className={CHAMP} />
        </Field>

        <Field label="Échéance" error={errors.due_on?.message}>
          <Input {...form.register('due_on')} type="date" className={CHAMP} />
        </Field>
      </Card>

      <SaveBar
        dirty={form.formState.isDirty}
        pending={update.isPending}
        onCancel={() => form.reset()}
      />
    </form>
  )
}
