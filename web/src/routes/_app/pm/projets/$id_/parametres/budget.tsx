import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { Input } from '@/components/ui/input'
import { projectDetailQuery, useUpdateProject } from '@/features/projects/api'
import { BUDGET_STATE, budgetStateOf, formatHours } from '@/features/projects/format'
import { Meter } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { ProjectDetail } from '@/types/api'

import { CHAMP, Card, Field, SaveBar } from '@/components/settings-ui'
import { can } from '@/lib/auth'

const schema = z.object({
  hours_sold: z.number().min(0, 'Un nombre positif est attendu'),
})

type Values = z.infer<typeof schema>

export const Route = createFileRoute('/_app/pm/projets/$id_/parametres/budget')({
  // Sans le droit, pas de budget a regler : retour a la fiche.
  beforeLoad: ({ context, params }) => {
    if (!can(context.user, 'budgets.read')) {
      throw redirect({ to: '/pm/projets/$id', params: { id: params.id }, replace: true })
    }
  },
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
  // Recalcule a la frappe : on voit l'effet d'un budget revu avant de
  // l'enregistrer. Meme seuil que l'API, qui classe les projets ailleurs.
  const etat = budgetStateOf(cible, consommees, project.is_internal)

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
          hint="Modifiable ici. Les heures consommées, elles, viennent des saisies de temps."
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
              {formatHours(consommees)} consommées sur {formatHours(cible)}
            </p>
            <p
              className={cn('text-[16px] tabular-nums', etat === 'ok' || etat === 'none' ? 'text-[#73757c]' : 'font-medium')}
              style={etat === 'warning' || etat === 'over' ? { color: BUDGET_STATE[etat].color } : undefined}
            >
              {reste < 0 ? `${formatHours(-reste)} au-delà du vendu` : `${formatHours(reste)} restantes`}
            </p>
          </div>

          <Meter ratio={ratio} color={BUDGET_STATE[etat].color} className="max-w-none" />

          {project.is_internal && (
            <p className="text-[13px] text-[#73757c]">
              Projet interne : son temps n’est pas facturable, le budget ne déclenche aucune alerte.
            </p>
          )}

          {!project.is_internal && etat === 'warning' && (
            <p className="text-[13px] text-[#73757c]">
              Plus de 80 % du budget est consommé : c’est le moment de prévenir le client.
            </p>
          )}

          {project.hours_spent === 0 && (
            <p className="text-[13px] text-[#73757c]">Aucune heure saisie sur ce projet pour l’instant.</p>
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
