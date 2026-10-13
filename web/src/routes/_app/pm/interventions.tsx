import { Add01Icon, ArrowLeft01Icon, ArrowRight01Icon, CheckmarkSquare02Icon, EyeIcon, Wrench01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'
import { z } from 'zod'

import { PageFrame } from '@/components/layout/page-frame'
import { PanelCard } from '@/components/panel-card'
import { CHAMP, Field } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { interventionsQuery } from '@/features/maintenance/api'
import { INTERVENTION_KIND, INTERVENTION_KINDS, monthLabel, monthOf, shiftMonth } from '@/features/maintenance/format'
import { ConsumptionBar, KindPill } from '@/features/maintenance/ui'
import { projectListQuery } from '@/features/projects/api'
import { Avatars } from '@/features/projects/ui'
import { useCreateTimeEntry } from '@/features/time/api'
import { formatDuration, parseDuration } from '@/features/time/format'
import { HttpError } from '@/lib/api'
import { can, sessionQuery } from '@/lib/auth'
import type { InterventionKind } from '@/types/api'

/**
 * Le journal des interventions : ce que l'equipe a fait sur les sites sous
 * maintenance, mois par mois, et ou en est chaque projet de son temps inclus.
 * Une intervention est une saisie de temps qualifiee : elle compte aussi dans
 * les rapports de temps.
 */
const searchSchema = z.object({
  mois: z.string().regex(/^\d{4}-\d{2}$/).optional().catch(undefined),
  projet: z.string().uuid().optional().catch(undefined),
  page: z.number().int().min(1).optional().catch(undefined),
})

export const Route = createFileRoute('/_app/pm/interventions')({
  validateSearch: searchSchema,
  component: InterventionsPage,
})

const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })
const ALL = 'tous'
const PAGE_SIZE = 25

function InterventionsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const month = search.mois ?? monthOf(new Date())
  const project = search.projet ?? null
  const page = search.page ?? 1
  const [creating, setCreating] = useState(false)
  const { data: session } = useQuery(sessionQuery)

  const { data } = useQuery(interventionsQuery(month, project, page))
  const { data: projects } = useQuery(projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }))
  const pages = Math.max(1, Math.ceil((data?.total ?? 0) / PAGE_SIZE))

  function go(next: { mois?: string; projet?: string | undefined; page?: number }) {
    void navigate({ search: (prev) => ({ ...prev, page: undefined, ...next }), replace: true })
  }

  return (
    <PageFrame title="Interventions">
      <div className="flex flex-col gap-4 p-4">
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex items-center gap-1">
            <Button type="button" variant="outline" size="icon-sm" aria-label="Mois précédent" onClick={() => go({ mois: shiftMonth(month, -1) })}>
              <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
            </Button>
            <span className="min-w-[140px] text-center text-[15px] font-medium text-[#1b1b1b]">{monthLabel(month)}</span>
            <Button type="button" variant="outline" size="icon-sm" aria-label="Mois suivant" onClick={() => go({ mois: shiftMonth(month, 1) })}>
              <HugeiconsIcon icon={ArrowRight01Icon} size={16} strokeWidth={1.8} />
            </Button>
          </div>
          <Select value={project ?? ALL} onValueChange={(v) => go({ projet: v === ALL ? undefined : v })}>
            <SelectTrigger size="sm" className="h-8 w-[240px] bg-white text-[13px]" aria-label="Projet">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Tous les projets</SelectItem>
              {(projects?.items ?? []).map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {can(session, 'time.write') && (
            <Button type="button" size="sm" className="ml-auto gap-1.5" onClick={() => setCreating(true)}>
              <HugeiconsIcon icon={Add01Icon} size={14} strokeWidth={1.8} />
              Nouvelle intervention
            </Button>
          )}
        </div>

        {data !== undefined && data.summaries.length > 0 && (
          <PanelCard icon={Wrench01Icon} title="TEMPS DU MOIS" contentClassName="p-3">
            <ul className="grid grid-cols-1 gap-x-8 gap-y-4 md:grid-cols-2 xl:grid-cols-3">
              {data.summaries.map((s) => (
                <li key={s.project.id} className="flex flex-col gap-1.5">
                  <span className="flex items-baseline justify-between gap-2">
                    <button type="button" className="truncate text-left text-[14px] text-[#1b1b1b] hover:underline" onClick={() => go({ projet: s.project.id })}>
                      {s.project.name}
                    </button>
                    <span className="shrink-0 text-[12px] text-[#8d8d8d]">{s.plan_name ?? 'Sans formule'}</span>
                  </span>
                  <ConsumptionBar consumed={s.consumed_minutes} included={s.included_minutes} />
                </li>
              ))}
            </ul>
          </PanelCard>
        )}

        <PanelCard
          icon={CheckmarkSquare02Icon}
          title="INTERVENTIONS"
          contentClassName="p-3"
          action={data !== undefined && <span className="text-[12px] text-[#73757c] tabular-nums">{data.total}</span>}
        >
          {data !== undefined && data.items.length === 0 ? (
            <p className="py-6 text-center text-[13px] text-[#73757c]">Aucune intervention en {monthLabel(month).toLowerCase()}.</p>
          ) : (
            <ul className="flex flex-col">
              {(data?.items ?? []).map((i) => (
                <li key={i.id} className="flex items-start gap-3 border-b border-[#f3f4f4] py-2.5 last:border-b-0">
                  <span className="w-[52px] shrink-0 pt-0.5 text-[12px] text-[#73757c] tabular-nums">{DAY.format(new Date(i.spent_on))}</span>
                  <span className="flex min-w-0 flex-1 flex-col gap-1">
                    <span className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-[14px] text-[#1b1b1b]">{i.project.name}</span>
                      <KindPill kind={i.kind} />
                    </span>
                    {i.note !== '' && <span className="text-[13px] text-[#4b4b4f]">{i.note}</span>}
                    {i.client_visible && (
                      <span className="flex items-start gap-1.5 text-[12px] text-[#3b45c9]">
                        <HugeiconsIcon icon={EyeIcon} size={14} strokeWidth={1.8} className="mt-px shrink-0" />
                        {i.client_note}
                      </span>
                    )}
                  </span>
                  {i.person !== null && <Avatars people={[i.person]} max={1} size={24} />}
                  <span className="w-[56px] shrink-0 pt-0.5 text-right text-[13px] font-medium text-[#1b1b1b] tabular-nums">{formatDuration(i.minutes)}</span>
                </li>
              ))}
            </ul>
          )}
          {pages > 1 && (
            <div className="flex items-center justify-end gap-2 pt-2 text-[13px] text-[#73757c]">
              Page {page} sur {pages}
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => go({ page: page - 1 })}>
                Précédent
              </Button>
              <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => go({ page: page + 1 })}>
                Suivant
              </Button>
            </div>
          )}
        </PanelCard>
      </div>
      {creating && <InterventionDialog defaultProject={project} onClose={() => setCreating(false)} />}
    </PageFrame>
  )
}

/** La saisie d'une intervention : une saisie de temps, qualifiee. */
function InterventionDialog({ defaultProject, onClose }: { defaultProject: string | null; onClose: () => void }) {
  const create = useCreateTimeEntry()
  const { data: projects } = useQuery(projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }))
  const [projectId, setProjectId] = useState(defaultProject ?? '')
  const [day, setDay] = useState(new Date().toISOString().slice(0, 10))
  const [duration, setDuration] = useState('')
  const [kind, setKind] = useState<InterventionKind>('correctif')
  const [note, setNote] = useState('')
  const [visible, setVisible] = useState(true)
  const [clientNote, setClientNote] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})

  function submit() {
    const minutes = parseDuration(duration)
    const next: Record<string, string> = {}
    if (projectId === '') next.project_id = 'Choisissez un projet'
    if (minutes === null) next.minutes = 'Une durée : 45, 1h30, 1,5'
    if (visible && clientNote.trim() === '') next.client_note = 'Ce que le client lira'
    setErrors(next)
    if (Object.keys(next).length > 0 || minutes === null) return

    create.mutate(
      { project_id: projectId, spent_on: day, minutes, note, intervention_kind: kind, client_visible: visible, client_note: visible ? clientNote : '' },
      {
        onSuccess: () => {
          toast.success('Intervention enregistrée')
          onClose()
        },
        onError: (error) => {
          if (error instanceof HttpError && error.code === 'VALIDATION_FAILED') {
            setErrors(Object.fromEntries(Object.entries(error.details).map(([k, v]) => [k, String(v)])))
            return
          }
          toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible')
        },
      },
    )
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>Nouvelle intervention</DialogTitle>
        </DialogHeader>
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault()
            submit()
          }}
        >
          <Field label="Projet" error={errors.project_id}>
            <Select value={projectId} onValueChange={setProjectId}>
              <SelectTrigger className="w-full" aria-label="Projet">
                <SelectValue placeholder="Choisir un projet" />
              </SelectTrigger>
              <SelectContent>
                {(projects?.items ?? []).map((p) => (
                  <SelectItem key={p.id} value={p.id}>
                    {p.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <Field label="Date">
              <Input className={CHAMP} type="date" value={day} onChange={(e) => setDay(e.target.value)} />
            </Field>
            <Field label="Durée" error={errors.minutes}>
              <Input className={CHAMP} value={duration} placeholder="45" onChange={(e) => setDuration(e.target.value)} />
            </Field>
            <Field label="Nature" error={errors.intervention_kind}>
              <Select value={kind} onValueChange={(v: InterventionKind) => setKind(v)}>
                <SelectTrigger className="w-full" aria-label="Nature">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {INTERVENTION_KINDS.map((k) => (
                    <SelectItem key={k} value={k}>
                      {INTERVENTION_KIND[k].label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          </div>
          <Field label="Note pour l’équipe">
            <Textarea value={note} rows={2} placeholder="Ce qui a été fait, et comment" onChange={(e) => setNote(e.target.value)} />
          </Field>
          <label className="flex items-center gap-2 text-[14px] text-[#1b1b1b]">
            <Checkbox checked={visible} onCheckedChange={(v) => setVisible(v === true)} />
            Visible par le client, dans son espace et son rapport mensuel
          </label>
          {visible && (
            <Field label="Ce que le client lit" error={errors.client_note}>
              <Textarea value={clientNote} rows={2} maxLength={500} placeholder="Correction du formulaire de contact" onChange={(e) => setClientNote(e.target.value)} />
            </Field>
          )}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Annuler
            </Button>
            <Button type="submit" disabled={create.isPending}>
              Enregistrer
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
