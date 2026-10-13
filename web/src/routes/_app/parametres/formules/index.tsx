import { Add01Icon, Delete02Icon, PencilEdit02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { PageFrame } from '@/components/layout/page-frame'
import { CHAMP, Card, Field, SaveBar } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { maintenancePlansQuery, useDeletePlan, useSavePlan, type PlanValues } from '@/features/maintenance/api'
import { reportSettingsQuery, useUpdateReportSettings } from '@/features/reports/api'
import { REPORT_SECTIONS } from '@/features/maintenance/format'
import { StatusPill } from '@/features/projects/ui'
import { formatDuration, parseDuration } from '@/features/time/format'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import type { MaintenancePlan } from '@/types/api'

/**
 * Les formules de maintenance : un temps inclus par mois, non cumulable, et
 * les sections du rapport mensuel. Une formule qui a servi ne se supprime
 * pas, elle se desactive : elle reste l'histoire des projets qui l'ont eue.
 */
export const Route = createFileRoute('/_app/parametres/formules/')({
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'reports.write')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  loader: ({ context }) => context.queryClient.ensureQueryData(maintenancePlansQuery),
  component: PlansPage,
})

const OFF = { label: 'Désactivée', color: '#c4c4c4', pill: { bg: '#f3f4f4', border: '#e8e8e9', text: '#73757c' } }

function PlansPage() {
  const { data } = useQuery(maintenancePlansQuery)
  const [editing, setEditing] = useState<MaintenancePlan | 'new' | null>(null)
  const remove = useDeletePlan()

  return (
    <PageFrame title="Formules de maintenance">
      <div className="flex flex-col gap-3 p-4">
        <Card
          title="Formules de maintenance"
          description="Le temps inclus chaque mois — il ne se reporte pas d’un mois sur l’autre — et ce que le rapport mensuel montre au client. Seules les saisies marquées « intervention » comptent sur le temps inclus."
        >
          {(data?.items ?? []).length === 0 ? (
            <p className="py-6 text-center text-[14px] text-[#73757c]">Aucune formule pour l’instant.</p>
          ) : (
            <ul className="flex flex-col">
              {(data?.items ?? []).map((p) => (
                <li key={p.id} className="flex flex-wrap items-center gap-3 border-b border-[#f3f4f4] py-2.5 last:border-b-0">
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex items-center gap-2 text-[14px] font-medium text-[#1b1b1b]">
                      {p.name}
                      {!p.active && <StatusPill {...OFF} />}
                    </span>
                    <span className="truncate text-[12px] text-[#8d8d8d]">
                      {formatDuration(p.included_minutes)} par mois ·{' '}
                      {p.sections.map((s) => REPORT_SECTIONS.find((r) => r.key === s)?.label ?? s).join(', ') || 'aucune section'}
                    </span>
                  </span>
                  <Button type="button" variant="ghost" size="icon-sm" aria-label={`Modifier ${p.name}`} onClick={() => setEditing(p)}>
                    <HugeiconsIcon icon={PencilEdit02Icon} size={16} strokeWidth={1.8} />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Supprimer ${p.name}`}
                    onClick={() =>
                      remove.mutate(p.id, {
                        onSuccess: () => toast.success('Formule supprimée'),
                        onError: (e) => toast.error(e instanceof HttpError ? e.message : 'Suppression impossible'),
                      })
                    }
                  >
                    <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.8} />
                  </Button>
                </li>
              ))}
            </ul>
          )}
          <Button type="button" variant="outline" size="sm" className="gap-1.5 self-start" onClick={() => setEditing('new')}>
            <HugeiconsIcon icon={Add01Icon} size={14} strokeWidth={1.8} />
            Nouvelle formule
          </Button>
        </Card>
        <SendSettings />
      </div>
      {editing !== null && <PlanDialog plan={editing === 'new' ? null : editing} onClose={() => setEditing(null)} />}
    </PageFrame>
  )
}

function PlanDialog({ plan, onClose }: { plan: MaintenancePlan | null; onClose: () => void }) {
  const save = useSavePlan(plan?.id ?? null)
  const [name, setName] = useState(plan?.name ?? '')
  const [duration, setDuration] = useState(plan === null ? '2h' : formatDuration(plan.included_minutes))
  const [sections, setSections] = useState<PlanValues['sections']>(plan?.sections ?? ['availability', 'audience', 'interventions', 'deployments'])
  const [active, setActive] = useState(plan?.active ?? true)
  const [errors, setErrors] = useState<Record<string, string>>({})

  function submit() {
    const minutes = parseDuration(duration)
    if (minutes === null) {
      setErrors({ included_minutes: 'Une durée : 2h, 1h30, 90' })
      return
    }
    save.mutate(
      { name, included_minutes: minutes, sections, active },
      {
        onSuccess: () => {
          toast.success(plan === null ? 'Formule créée' : 'Formule enregistrée')
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
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{plan === null ? 'Nouvelle formule' : 'Modifier la formule'}</DialogTitle>
        </DialogHeader>
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault()
            submit()
          }}
        >
          <Field label="Nom" error={errors.name}>
            <Input className={CHAMP} value={name} placeholder="Sérénité" onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field label="Temps inclus par mois" hint="Non cumulable : ce qui n’est pas consommé ne se reporte pas." error={errors.included_minutes}>
            <Input className={CHAMP} value={duration} placeholder="2h" onChange={(e) => setDuration(e.target.value)} />
          </Field>
          <Field label="Sections du rapport" hint="Une section sans données ce mois-là n’apparaît pas." error={errors.sections}>
            <div className="grid grid-cols-2 gap-2">
              {REPORT_SECTIONS.map((s) => (
                <label key={s.key} className="flex items-center gap-2 text-[14px] text-[#1b1b1b]">
                  <Checkbox
                    checked={sections.includes(s.key)}
                    onCheckedChange={(v) => setSections((list) => (v === true ? [...list, s.key] : list.filter((x) => x !== s.key)))}
                  />
                  {s.label}
                  {s.soon && <span className="text-[12px] text-[#a2a3a7]">bientôt</span>}
                </label>
              ))}
            </div>
          </Field>
          <label className="flex items-center gap-2 text-[14px] text-[#1b1b1b]">
            <Checkbox checked={active} onCheckedChange={(v) => setActive(v === true)} />
            Proposée aux projets
          </label>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              Annuler
            </Button>
            <Button type="submit" disabled={save.isPending}>
              Enregistrer
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

/**
 * L'envoi des rapports. Automatique, il valide et envoie seul les rapports du
 * mois le jour dit : desactive par defaut, un rapport part apres relecture.
 */
function SendSettings() {
  const { data } = useQuery(reportSettingsQuery)
  if (data === undefined) return null
  return <SendSettingsForm key={JSON.stringify(data)} initial={data} />
}

function SendSettingsForm({ initial }: { initial: { auto_send: boolean; send_day: number; wait_days: number } }) {
  const save = useUpdateReportSettings()
  const [values, setValues] = useState(initial)
  const dirty = JSON.stringify(values) !== JSON.stringify(initial)
  return (
    <form
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        save.mutate(values, {
          onSuccess: () => toast.success('Réglages enregistrés'),
          onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
        })
      }}
    >
      <Card
        title="Rapports mensuels"
        description="Le 1er de chaque mois, un brouillon de rapport est préparé pour chaque projet sous formule. L’agence le relit, le valide puis l’envoie."
      >
        <div className="flex flex-col gap-3 sm:flex-row">
          <div className="sm:w-[260px]">
            <Field label="Attente des chiffres d’audience" hint="Jours d’attente de la photo du mois d’Umami avant de préparer le rapport sans elle.">
              <Input className={CHAMP} type="number" min={0} max={10} value={values.wait_days} onChange={(e) => setValues((v) => ({ ...v, wait_days: Number(e.target.value) }))} />
            </Field>
          </div>
        </div>
        <label className="flex items-start gap-3 px-1 text-[14px] text-[#1b1b1b]">
          <Checkbox className="mt-0.5" checked={values.auto_send} onCheckedChange={(v) => setValues((s) => ({ ...s, auto_send: v === true }))} />
          <span className="flex flex-col gap-0.5">
            Envoyer automatiquement, sans relecture
            <span className="text-[13px] text-[#73757c]">Les rapports du mois encore en brouillon sont validés et envoyés tels quels le jour choisi.</span>
          </span>
        </label>
        {values.auto_send && (
          <div className="sm:w-[260px]">
            <Field label="Jour d’envoi">
              <Input className={CHAMP} type="number" min={1} max={28} value={values.send_day} onChange={(e) => setValues((v) => ({ ...v, send_day: Number(e.target.value) }))} />
            </Field>
          </div>
        )}
        <SaveBar dirty={dirty} pending={save.isPending} onCancel={() => setValues(initial)} />
      </Card>
    </form>
  )
}
