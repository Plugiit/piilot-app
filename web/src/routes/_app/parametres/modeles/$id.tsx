import { Cancel01Icon, CheckmarkSquare02Icon, Flag02Icon, PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import { PageFrame, type Crumb } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { PROJECT_PRIORITY } from '@/features/projects/format'
import { serviceOptionsQuery } from '@/features/services/api'
import { templateQuery, useDeleteTemplate, useSaveTemplate, type TemplateValues } from '@/features/templates/api'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { ProjectPriority, ProjectTemplate } from '@/types/api'

/**
 * Editeur d'un modele de projet.
 *
 * Tout s'edite sur une page et s'enregistre d'un bloc : un modele se relit en
 * entier avant d'etre utilise, et l'enregistrer ligne a ligne laisserait un
 * modele a moitie modifie entre deux clics.
 *
 * `nouveau` en guise d'identifiant ouvre un modele vierge.
 */
export const Route = createFileRoute('/_app/parametres/modeles/$id')({
  loader: ({ context, params }) =>
    params.id === 'nouveau'
      ? undefined
      : context.queryClient.query({ ...templateQuery(params.id), staleTime: 'static' }),
  component: TemplatePage,
})

const MODELES: Crumb = { label: 'Modèles de projet', to: '/parametres/modeles' }

const VIERGE: TemplateValues = {
  name: '',
  description: '',
  service_ids: [],
  milestones: [{ title: 'Cadrage', offset_days: 0 }],
  tasks: [],
}

function valuesOf(template: ProjectTemplate): TemplateValues {
  return {
    name: template.name,
    description: template.description,
    service_ids: template.services.map((service) => service.id),
    milestones: template.milestones,
    tasks: template.tasks,
  }
}

function TemplatePage() {
  const { id } = Route.useParams()
  const creating = id === 'nouveau'
  const { data, isError, error } = useQuery({ ...templateQuery(id), enabled: !creating })

  if (isError) {
    return (
      <PageFrame title="Modèle de projet" trail={[MODELES]}>
        <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
          {error instanceof HttpError ? error.message : 'Chargement impossible'}
        </p>
      </PageFrame>
    )
  }

  if (!creating && data === undefined) return null

  return (
    <Editor
      key={data?.updated_at ?? 'nouveau'}
      id={creating ? null : id}
      initial={data === undefined ? VIERGE : valuesOf(data)}
    />
  )
}

function Editor({ id, initial }: { id: string | null; initial: TemplateValues }) {
  const navigate = useNavigate()
  const { user } = Route.useRouteContext()
  const canEdit = can(user, 'projects.write')
  const [values, setValues] = useState(initial)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const save = useSaveTemplate(id)
  const remove = useDeleteTemplate(id ?? '')
  const { data: services } = useQuery(serviceOptionsQuery())

  const dirty = JSON.stringify(values) !== JSON.stringify(initial)

  function patch(next: Partial<TemplateValues>) {
    setValues((prev) => ({ ...prev, ...next }))
  }

  function submit() {
    if (values.name.trim() === '') {
      setErrors({ name: 'Le nom est requis' })
      return
    }
    setErrors({})

    save.mutate(values, {
      onSuccess: (template) => {
        toast.success(id === null ? 'Modèle créé' : 'Modèle enregistré')
        if (id === null) void navigate({ to: '/parametres/modeles/$id', params: { id: template.id }, replace: true })
      },
      onError: (error) => {
        if (error instanceof HttpError && error.code === 'VALIDATION_FAILED') {
          setErrors(Object.fromEntries(Object.entries(error.details).map(([k, v]) => [k, String(v)])))
        }
        toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible')
      },
    })
  }

  return (
    <PageFrame title={values.name.trim() === '' ? 'Nouveau modèle' : values.name} trail={[MODELES]}>
      <div className="flex min-h-full flex-col gap-4 p-4 pb-24">
        <section className="grid max-w-[880px] grid-cols-1 gap-4 md:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="template-name">Nom</Label>
            <Input
              id="template-name"
              value={values.name}
              disabled={!canEdit}
              onChange={(event) => patch({ name: event.target.value })}
              placeholder="Site vitrine, refonte, maintenance…"
              aria-invalid={errors.name !== undefined}
            />
            {errors.name !== undefined && <p className="text-[12px] text-[#e5484d]">{errors.name}</p>}
          </div>

          <div className="flex flex-col gap-1.5 md:row-span-2">
            <Label htmlFor="template-description">Description</Label>
            <Textarea
              id="template-description"
              rows={4}
              value={values.description}
              disabled={!canEdit}
              onChange={(event) => patch({ description: event.target.value })}
              placeholder="Pour quel type de projet, et ce qu’il faut adapter (facultatif)"
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <span className="text-[14px] font-medium text-[#1b1b1b]">Services</span>
            <div className="flex flex-wrap gap-1.5">
              {(services?.items ?? []).map((service) => {
                const on = values.service_ids.includes(service.id)

                return (
                  <button
                    key={service.id}
                    type="button"
                    disabled={!canEdit}
                    aria-pressed={on}
                    onClick={() =>
                      patch({
                        service_ids: on
                          ? values.service_ids.filter((sid) => sid !== service.id)
                          : [...values.service_ids, service.id],
                      })
                    }
                    className={cn(
                      'flex cursor-pointer items-center gap-1.5 rounded-full border px-2.5 py-1 text-[12px] transition-colors disabled:cursor-default',
                      on
                        ? 'border-[#1b1b1b] bg-[#1b1b1b] text-white'
                        : 'border-[#e8e8e9] bg-white text-[#4b4b4f] hover:border-[#d0d1d3]',
                    )}
                  >
                    <span aria-hidden className="size-2 rounded-full" style={{ backgroundColor: service.color }} />
                    {service.name}
                  </button>
                )
              })}
            </div>
          </div>
        </section>

        <Block
          icon={Flag02Icon}
          title="Jalons"
          hint="Les étapes du projet. L’échéance est comptée en jours depuis son début."
          error={errors.milestones}
          action={
            canEdit && (
              <AddButton
                label="Ajouter un jalon"
                onClick={() =>
                  patch({
                    milestones: [
                      ...values.milestones,
                      { title: '', offset_days: (values.milestones.at(-1)?.offset_days ?? 0) + 14 },
                    ],
                  })
                }
              />
            )
          }
        >
          {values.milestones.length === 0 && <EmptyLine>Aucun jalon.</EmptyLine>}
          {values.milestones.map((milestone, index) => (
            <Row key={index}>
              <Input
                value={milestone.title}
                disabled={!canEdit}
                placeholder="Maquettes validées"
                aria-label={`Titre du jalon ${index + 1}`}
                onChange={(event) =>
                  patch({
                    milestones: values.milestones.map((m, i) => (i === index ? { ...m, title: event.target.value } : m)),
                  })
                }
                className="h-9 flex-1 text-[13px]"
              />
              <Offset
                value={milestone.offset_days}
                disabled={!canEdit}
                label={`Échéance du jalon ${index + 1}`}
                onChange={(days) =>
                  patch({
                    milestones: values.milestones.map((m, i) => (i === index ? { ...m, offset_days: days ?? 0 } : m)),
                  })
                }
              />
              {canEdit && (
                <RemoveButton
                  label={`Retirer le jalon ${index + 1}`}
                  onClick={() => patch({ milestones: values.milestones.filter((_, i) => i !== index) })}
                />
              )}
            </Row>
          ))}
        </Block>

        <Block
          icon={CheckmarkSquare02Icon}
          title="Tâches"
          hint="Créées « à faire » dans le projet. Sans échéance, la tâche n’en reçoit pas."
          error={errors.tasks}
          action={
            canEdit && (
              <AddButton
                label="Ajouter une tâche"
                onClick={() =>
                  patch({
                    tasks: [...values.tasks, { title: '', description: '', priority: 'medium', offset_days: null }],
                  })
                }
              />
            )
          }
        >
          {values.tasks.length === 0 && <EmptyLine>Aucune tâche.</EmptyLine>}
          {values.tasks.map((task, index) => (
            <Row key={index}>
              <Input
                value={task.title}
                disabled={!canEdit}
                placeholder="Atelier de cadrage"
                aria-label={`Titre de la tâche ${index + 1}`}
                onChange={(event) =>
                  patch({ tasks: values.tasks.map((t, i) => (i === index ? { ...t, title: event.target.value } : t)) })
                }
                className="h-9 flex-1 text-[13px]"
              />
              <Select
                value={task.priority}
                disabled={!canEdit}
                onValueChange={(priority) =>
                  patch({
                    tasks: values.tasks.map((t, i) =>
                      i === index ? { ...t, priority: priority as ProjectPriority } : t,
                    ),
                  })
                }
              >
                <SelectTrigger className="h-9 w-[120px] text-[13px]" aria-label={`Priorité de la tâche ${index + 1}`}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(PROJECT_PRIORITY) as ProjectPriority[]).map((priority) => (
                    <SelectItem key={priority} value={priority}>
                      {PROJECT_PRIORITY[priority].label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Offset
                value={task.offset_days}
                optional
                disabled={!canEdit}
                label={`Échéance de la tâche ${index + 1}`}
                onChange={(days) =>
                  patch({ tasks: values.tasks.map((t, i) => (i === index ? { ...t, offset_days: days } : t)) })
                }
              />
              {canEdit && (
                <RemoveButton
                  label={`Retirer la tâche ${index + 1}`}
                  onClick={() => patch({ tasks: values.tasks.filter((_, i) => i !== index) })}
                />
              )}
            </Row>
          ))}
        </Block>

        {canEdit && id !== null && (
          <div className="max-w-[880px]">
            <Button
              variant="ghost"
              className="text-[#e5484d] hover:bg-[#fdf3f3] hover:text-[#e5484d]"
              disabled={remove.isPending}
              onClick={() => {
                if (!window.confirm('Supprimer ce modèle ? Les projets déjà créés à partir de lui ne changent pas.')) return
                remove.mutate(undefined, {
                  onSuccess: () => {
                    toast.success('Modèle supprimé')
                    void navigate({ to: '/parametres/modeles' })
                  },
                  onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Suppression impossible'),
                })
              }}
            >
              Supprimer le modèle
            </Button>
          </div>
        )}

        {canEdit && (dirty || id === null) && (
          <div className="fixed bottom-6 left-1/2 z-20 flex -translate-x-1/2 items-center gap-3 rounded-[12px] border border-[#e8e8e9] bg-white py-2 pr-2 pl-4 shadow-[0_12px_28px_-8px_rgb(16_24_40/0.28)]">
            <p className="text-[13px] text-[#1b1b1b]">
              {id === null ? 'Nouveau modèle, pas encore enregistré' : 'Modifications non enregistrées'}
            </p>
            {id !== null && (
              <Button size="sm" variant="ghost" onClick={() => setValues(initial)}>
                Annuler
              </Button>
            )}
            <Button size="sm" disabled={save.isPending} onClick={submit}>
              {save.isPending ? 'Enregistrement…' : 'Enregistrer'}
            </Button>
          </div>
        )}
      </div>
    </PageFrame>
  )
}

function Block({
  icon,
  title,
  hint,
  error,
  action,
  children,
}: {
  icon: IconSvgElement
  title: string
  hint: string
  error?: string
  action?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="flex max-w-[880px] flex-col gap-2 rounded-[12px] border border-[#e8e8e9] bg-white p-3">
      <header className="flex items-start gap-2">
        <HugeiconsIcon icon={icon} size={16} strokeWidth={1.8} className="mt-0.5 text-[#606060]" />
        <div className="flex flex-1 flex-col">
          <h2 className="text-[14px] font-medium text-[#1b1b1b]">{title}</h2>
          <p className="text-[12px] text-[#8d8d8d]">{hint}</p>
        </div>
        {action}
      </header>
      {error !== undefined && <p className="text-[12px] text-[#e5484d]">{error}</p>}
      <div className="flex flex-col gap-1.5">{children}</div>
    </section>
  )
}

function Row({ children }: { children: ReactNode }) {
  return <div className="flex items-center gap-2">{children}</div>
}

function EmptyLine({ children }: { children: ReactNode }) {
  return <p className="py-2 text-[13px] text-[#a2a3a7]">{children}</p>
}

function AddButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <Button variant="outline" size="sm" className="gap-1.5" onClick={onClick}>
      <HugeiconsIcon icon={PlusSignIcon} size={14} strokeWidth={2} />
      {label}
    </Button>
  )
}

function RemoveButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      aria-label={label}
      onClick={onClick}
      className="flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-[8px] text-[#a2a3a7] hover:bg-[#f3f4f4] hover:text-[#1b1b1b]"
    >
      <HugeiconsIcon icon={Cancel01Icon} size={14} strokeWidth={1.8} />
    </button>
  )
}

/** Ecart en jours depuis le debut du projet : « J+14 ». Vide, sans echeance. */
function Offset({
  value,
  optional = false,
  disabled,
  label,
  onChange,
}: {
  value: number | null
  optional?: boolean
  disabled: boolean
  label: string
  onChange: (days: number | null) => void
}) {
  return (
    <span className="flex h-9 w-[104px] shrink-0 items-center rounded-[8px] border border-[#e8e8e9] bg-white pl-2 text-[13px] text-[#73757c] focus-within:border-[#d0d1d3]">
      J+
      <input
        type="number"
        min={0}
        max={3650}
        inputMode="numeric"
        disabled={disabled}
        aria-label={label}
        placeholder={optional ? '—' : '0'}
        value={value ?? ''}
        onChange={(event) => {
          const raw = event.target.value
          if (raw === '') {
            onChange(optional ? null : 0)
            return
          }
          const days = Math.max(0, Math.min(3650, Math.round(Number(raw))))
          if (!Number.isNaN(days)) onChange(days)
        }}
        className="h-full w-full min-w-0 bg-transparent px-1 text-[#1b1b1b] tabular-nums outline-none"
      />
    </span>
  )
}
