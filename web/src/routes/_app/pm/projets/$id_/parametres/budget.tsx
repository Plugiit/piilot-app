import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { Input } from '@/components/ui/input'
import { projectDetailQuery, useUpdateProject } from '@/features/projects/api'
import { ALERT_COLOR, DONE_COLOR, PROGRESS_COLOR } from '@/features/projects/format'
import { Meter } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { ProjectDetail } from '@/types/api'

import { CHAMP, Card, Field, SaveBar } from '@/components/settings-ui'

const schema = z.object({
  hours_sold: z.number().min(0, 'Un nombre positif est attendu'),
})

type Values = z.infer<typeof schema>

export const Route = createFileRoute('/_app/pm/projets/$id_/parametres/budget')({
  component: BudgetPage,
})

function BudgetPage() {
  const { id } = Route.useParams()
  const { data: project } = useQuery(projectDetailQuery(id))

  if (project === undefined) return null

  return <BudgetForm key={project.id} project={project} />
}

function BudgetForm({ project }: { project: ProjectDetail }) {
  const update = useUpdateProject(project.id)

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { hours_sold: project.hours_sold },
  })

  const vendues = form.watch('hours_sold')
  const cible = Number.isNaN(vendues) ? 0 : vendues
  const consommees = project.hours_spent
  const reste = cible - consommees
  const ratio = cible === 0 ? 0 : (consommees / cible) * 100

  function onSubmit(values: Values) {
    update.mutate(
      { hours_sold: values.hours_sold },
      {
        onSuccess: () => {
          toast.success('Budget enregistré')
          form.reset(values)
        },
        onError: (error) =>
          toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
      },
    )
  }

  return (
    <form noValidate onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-3 p-4">
      <Card title="Heures vendues" description="Ce qui a été chiffré au devis.">
        <Field
          label="Heures vendues"
          hint="Modifiable ici. Les heures consommées, elles, viendront des saisies de temps."
          error={form.formState.errors.hours_sold?.message}
        >
          <div className="flex items-center gap-2">
            <Input
              {...form.register('hours_sold', { valueAsNumber: true })}
              type="number"
              min={0}
              step="0.25"
              className={`${CHAMP} w-[160px] text-right tabular-nums`}
            />
            <span className="text-[16px] text-[#73757c]">heures</span>
          </div>
        </Field>
      </Card>

      <Card title="Consommation" description="Lecture seule : ce chiffre vient des saisies de temps.">
        <div className="flex flex-col gap-2">
          <div className="flex items-baseline justify-between gap-4">
            <p className="text-[16px] text-[#1b1b1b] tabular-nums">
              {consommees} h consommées sur {cible} h
            </p>
            <p
              className={cn(
                'text-[16px] font-medium tabular-nums',
                reste < 0 ? 'text-[#e5484d]' : 'text-[#73757c]',
              )}
            >
              {reste < 0 ? `${-reste} h au-delà du vendu` : `${reste} h restantes`}
            </p>
          </div>

          <Meter
            ratio={ratio}
            color={reste < 0 ? ALERT_COLOR : ratio >= 90 ? PROGRESS_COLOR : DONE_COLOR}
            className="max-w-none"
          />

          {project.hours_spent === 0 && (
            <p className="text-[13px] text-[#73757c]">
              Aucune heure saisie : le module de suivi du temps n’est pas encore écrit.
            </p>
          )}
        </div>
      </Card>

      <SaveBar
        dirty={form.formState.isDirty}
        pending={update.isPending}
        onCancel={() => form.reset()}
      />
    </form>
  )
}
