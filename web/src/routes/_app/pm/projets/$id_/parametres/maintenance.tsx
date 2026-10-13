import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { CHAMP, Card, Field, SaveBar } from '@/components/settings-ui'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { maintenancePlansQuery, projectMaintenanceQuery, useSetProjectMaintenance } from '@/features/maintenance/api'
import { monthLabel, monthOf } from '@/features/maintenance/format'
import { reportRecipientsQuery, useSetReportRecipients } from '@/features/reports/api'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { formatDuration } from '@/features/time/format'
import { HttpError } from '@/lib/api'
import { can, sessionQuery } from '@/lib/auth'
import type { ProjectMaintenance, ReportRecipients } from '@/types/api'

/**
 * La formule de maintenance d'un projet. Elle commence un 1er : le temps
 * inclus est mensuel. En changer ferme la precedente la veille ; les mois
 * passes gardent la leur.
 */
export const Route = createFileRoute('/_app/pm/projets/$id_/parametres/maintenance')({
  loader: ({ context, params }) =>
    Promise.all([
      context.queryClient.ensureQueryData(projectMaintenanceQuery(params.id)),
      context.queryClient.ensureQueryData(maintenancePlansQuery),
    ]),
  component: MaintenanceTab,
})

const NONE = 'aucune'
const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })

function MaintenanceTab() {
  const { id } = Route.useParams()
  const { data } = useQuery(projectMaintenanceQuery(id))
  const { data: session } = useQuery(sessionQuery)
  if (data === undefined) return null
  return (
    <div className="flex flex-col gap-3 p-4">
      <PlanForm key={data.current?.id ?? 'none'} projectId={id} view={data} canEdit={can(session, 'projects.write')} canManagePlans={can(session, 'reports.write')} />
      <RecipientsCard projectId={id} canEdit={can(session, 'projects.write')} />
      {data.history.length > 0 && (
        <Card title="Historique">
          <ul className="flex flex-col">
            {data.history.map((p) => (
              <li key={p.id} className="flex flex-wrap items-center gap-3 border-b border-[#f3f4f4] py-2 last:border-b-0">
                <span className="min-w-0 flex-1 truncate text-[14px] text-[#1b1b1b]">{p.plan_name}</span>
                <span className="text-[13px] text-[#73757c] tabular-nums">{formatDuration(p.included_minutes)} par mois</span>
                <span className="w-[260px] text-right text-[13px] text-[#73757c]">
                  depuis le {DAY.format(new Date(p.starts_on))}
                  {p.ends_on !== null && `, jusqu’au ${DAY.format(new Date(p.ends_on))}`}
                </span>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  )
}

function PlanForm({ projectId, view, canEdit, canManagePlans }: { projectId: string; view: ProjectMaintenance; canEdit: boolean; canManagePlans: boolean }) {
  const { data: plans } = useQuery(maintenancePlansQuery)
  const save = useSetProjectMaintenance(projectId)
  const initial = view.current?.plan_id ?? NONE
  const [plan, setPlan] = useState(initial)
  const [month, setMonth] = useState(monthOf(new Date()))
  const options = (plans?.items ?? []).filter((p) => p.active || p.id === initial)

  return (
    <form
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        save.mutate(
          { plan_id: plan === NONE ? null : plan, month },
          {
            onSuccess: () => toast.success(plan === NONE ? 'Le projet sort de la maintenance' : `Formule appliquée à partir de ${monthLabel(month).toLowerCase()}`),
            onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
          },
        )
      }}
    >
      <Card
        title="Formule de maintenance"
        description="Le temps inclus chaque mois, et ce que le rapport mensuel montre au client. Seules les interventions comptent sur ce temps."
      >
        {view.current !== null ? (
          <p className="px-1 text-[14px] text-[#1b1b1b]">
            <strong className="font-medium">{view.current.plan_name}</strong> · {formatDuration(view.current.included_minutes)} par mois, depuis{' '}
            {monthLabel(view.current.starts_on.slice(0, 7)).toLowerCase()}.
          </p>
        ) : (
          <p className="px-1 text-[14px] text-[#73757c]">Ce projet n’a pas de formule de maintenance.</p>
        )}
        {canEdit && (
          <div className="flex flex-col gap-3 sm:flex-row">
            <div className="min-w-0 flex-1">
              <Field label="Formule">
                <Select value={plan} onValueChange={setPlan}>
                  <SelectTrigger className="w-full" aria-label="Formule">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE}>Aucune</SelectItem>
                    {options.map((p) => (
                      <SelectItem key={p.id} value={p.id}>
                        {p.name} · {formatDuration(p.included_minutes)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {options.length === 0 && (
                  <p className="text-[13px] text-[#73757c]">
                    Aucune formule.{' '}
                    {canManagePlans && (
                      <Link to="/parametres/formules" className="underline">
                        En créer une
                      </Link>
                    )}
                  </p>
                )}
              </Field>
            </div>
            <div className="sm:w-[200px]">
              <Field label="À partir de">
                <Input className={CHAMP} type="month" value={month} onChange={(e) => setMonth(e.target.value)} />
              </Field>
            </div>
          </div>
        )}
        {canEdit && <SaveBar dirty={plan !== initial} pending={save.isPending} onCancel={() => setPlan(initial)} />}
      </Card>
    </form>
  )
}

/**
 * A qui part le rapport mensuel : les comptes du portail du client, sauf
 * ceux qu'on exclut, et des adresses en plus (un dirigeant sans compte).
 */
function RecipientsCard({ projectId, canEdit }: { projectId: string; canEdit: boolean }) {
  const { data } = useQuery(reportRecipientsQuery(projectId))
  if (data === undefined) return null
  return <RecipientsForm key={JSON.stringify(data)} projectId={projectId} view={data} canEdit={canEdit} />
}

function RecipientsForm({ projectId, view, canEdit }: { projectId: string; view: ReportRecipients; canEdit: boolean }) {
  const save = useSetReportRecipients(projectId)
  const initialExcluded = view.contacts.filter((c) => c.excluded && c.person !== null).map((c) => c.person!.id)
  const [excluded, setExcluded] = useState<string[]>(initialExcluded)
  const [extra, setExtra] = useState<string[]>(view.extra)
  const [draft, setDraft] = useState('')
  const dirty = JSON.stringify([...excluded].sort()) !== JSON.stringify([...initialExcluded].sort()) || JSON.stringify(extra) !== JSON.stringify(view.extra)

  return (
    <form
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        save.mutate(
          { excluded, extra },
          {
            onSuccess: () => toast.success('Destinataires enregistrés'),
            onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
          },
        )
      }}
    >
      <Card title="Destinataires du rapport mensuel" description="Les comptes du portail du client reçoivent le rapport ; décochez ceux qui ne doivent pas l’avoir. Ajoutez des adresses pour qui n’a pas de compte.">
        {view.contacts.length === 0 ? (
          <p className="px-1 text-[14px] text-[#73757c]">Aucun compte du portail pour ce client.</p>
        ) : (
          <ul className="flex flex-col">
            {view.contacts.map((c) => {
              const id = c.person?.id ?? c.email
              const on = !excluded.includes(id)
              return (
                <li key={c.email} className="flex items-center gap-3 border-b border-[#f3f4f4] py-2 last:border-b-0">
                  <Checkbox
                    checked={on}
                    disabled={!canEdit}
                    aria-label={`Envoyer à ${c.email}`}
                    onCheckedChange={(v) => setExcluded((list) => (v === true ? list.filter((x) => x !== id) : [...list, id]))}
                  />
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="truncate text-[14px] text-[#1b1b1b]">
                      {c.person !== null ? `${c.person.firstname} ${c.person.lastname}` : c.email}
                    </span>
                    <span className="truncate text-[12px] text-[#8d8d8d]">{c.email}</span>
                  </span>
                  {!on && <span className="text-[12px] text-[#a2a3a7]">exclu</span>}
                </li>
              )
            })}
          </ul>
        )}
        {extra.length > 0 && (
          <ul className="flex flex-col gap-1">
            {extra.map((e) => (
              <li key={e} className="flex items-center gap-2 text-[14px] text-[#1b1b1b]">
                <span className="min-w-0 flex-1 truncate">{e}</span>
                {canEdit && (
                  <Button type="button" variant="ghost" size="sm" onClick={() => setExtra((list) => list.filter((x) => x !== e))}>
                    Retirer
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
        {canEdit && extra.length < 10 && (
          <div className="flex gap-2">
            <Input
              className={CHAMP}
              type="email"
              placeholder="direction@client.fr"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && draft.includes('@')) {
                  e.preventDefault()
                  setExtra((list) => [...new Set([...list, draft.trim().toLowerCase()])])
                  setDraft('')
                }
              }}
            />
            <Button
              type="button"
              variant="outline"
              disabled={!draft.includes('@')}
              onClick={() => {
                setExtra((list) => [...new Set([...list, draft.trim().toLowerCase()])])
                setDraft('')
              }}
            >
              Ajouter
            </Button>
          </div>
        )}
        {canEdit && <SaveBar dirty={dirty} pending={save.isPending} onCancel={() => { setExcluded(initialExcluded); setExtra(view.extra) }} />}
      </Card>
    </form>
  )
}
