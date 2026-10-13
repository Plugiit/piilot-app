import { Cancel01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { SearchPicker } from '@/components/search-picker'
import { Card, SaveBar } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { projectUptimeQuery, uptimeMonitorsQuery, useSetProjectSites } from '@/features/uptime/api'
import { ENVIRONMENT, formatUptime } from '@/features/uptime/format'
import { IncidentList, MonitorStatus, UptimeStrip } from '@/features/uptime/ui'
import { HttpError } from '@/lib/api'
import { can, sessionQuery } from '@/lib/auth'
import type { ProjectUptime, UptimeMonitor } from '@/types/api'

/**
 * Les sites d'un projet : ceux qu'Uptime Kuma surveille pour lui, en
 * production et en preproduction. Facultatif — un projet sans site n'a
 * simplement pas de disponibilite a montrer.
 */
export const Route = createFileRoute('/_app/pm/projets/$id_/parametres/disponibilite')({
  loader: ({ context, params }) => context.queryClient.ensureQueryData(projectUptimeQuery(params.id)),
  component: AvailabilityPage,
})

type Environment = keyof typeof ENVIRONMENT

/** Un site choisi, avec ce qu'il faut pour l'afficher avant l'enregistrement. */
interface Choice {
  monitor_id: string
  environment: Environment
  name: string
  url: string
}

function choicesOf(view: ProjectUptime): Choice[] {
  return view.sites.map((s) => ({ monitor_id: s.monitor_id, environment: s.environment, name: s.name, url: s.url }))
}

function AvailabilityPage() {
  const { id } = Route.useParams()
  const { data } = useQuery(projectUptimeQuery(id))
  const { data: session } = useQuery(sessionQuery)

  if (data === undefined) return null

  if (!data.module_enabled && data.sites.length === 0) {
    return (
      <div className="flex flex-col gap-3 p-4">
        <Card title="Disponibilité" description="Reliez le projet aux sites qu’Uptime Kuma surveille : leur disponibilité et leurs incidents s’afficheront ici.">
          <p className="px-1 text-[14px] text-[#73757c]">
            Le module de disponibilité n’est pas activé.
            {can(session, 'users.write') && (
              <>
                {' '}
                <Link to="/parametres/modules/uptime" className="text-[#1b1b1b] underline">
                  L’activer dans les paramètres
                </Link>
                .
              </>
            )}
          </p>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-3 p-4">
      <SitesForm key={data.sites.map((s) => s.monitor_id + s.environment).join()} projectId={id} view={data} canEdit={can(session, 'projects.write')} />
      {data.sites.length > 0 && (
        <>
          <Card title="Trente derniers jours" description="Une case par jour. Les relèves ratées n’y comptent ni pour ni contre.">
            <div className="flex flex-col gap-5">
              {data.sites.map((s) => (
                <div key={s.monitor_id} className="flex flex-col gap-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-[14px] font-medium text-[#1b1b1b]">{s.name}</span>
                    <span className="text-[12px] text-[#73757c]">{ENVIRONMENT[s.environment]}</span>
                    {!s.present && <span className="text-[12px] text-[#a2a3a7]">retiré de Kuma</span>}
                    <span className="ml-auto flex items-center gap-3">
                      {s.cert_days !== null && (
                        <span className={`text-[12px] tabular-nums ${s.cert_days < 14 ? 'font-medium text-[#e5484d]' : 'text-[#73757c]'}`}>
                          Certificat : {s.cert_days} j
                        </span>
                      )}
                      <span className="text-[13px] font-medium text-[#1b1b1b] tabular-nums">{formatUptime(s.uptime_30d)}</span>
                      <MonitorStatus status={s.status} />
                    </span>
                  </div>
                  <UptimeStrip days={s.days} />
                </div>
              ))}
            </div>
          </Card>
          <Card title="Derniers incidents">
            <IncidentList items={data.incidents} />
          </Card>
        </>
      )}
    </div>
  )
}

function SitesForm({ projectId, view, canEdit }: { projectId: string; view: ProjectUptime; canEdit: boolean }) {
  const save = useSetProjectSites(projectId)
  const initial = choicesOf(view)
  const [choices, setChoices] = useState(initial)
  const dirty = JSON.stringify(choices.map((c) => [c.monitor_id, c.environment])) !== JSON.stringify(initial.map((c) => [c.monitor_id, c.environment]))

  function add(m: UptimeMonitor) {
    // Le premier site est la production ; les suivants, la preproduction.
    setChoices((list) => [...list, { monitor_id: m.id, environment: list.some((c) => c.environment === 'prod') ? 'preprod' : 'prod', name: m.name, url: m.url }])
  }

  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        save.mutate(
          choices.map((c) => ({ monitor_id: c.monitor_id, environment: c.environment })),
          {
            onSuccess: () => toast.success('Sites enregistrés'),
            onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
          },
        )
      }}
    >
      <Card title="Sites du projet" description="Choisissez parmi les sites qu’Uptime Kuma surveille. Facultatif, jusqu’à dix sites.">
        {canEdit && choices.length < 10 && <SitePicker exclude={choices.map((c) => c.monitor_id)} onPick={add} />}
        {choices.length === 0 ? (
          <p className="px-1 text-[14px] text-[#73757c]">Aucun site lié pour l’instant.</p>
        ) : (
          <ul className="flex flex-col">
            {choices.map((c) => (
              <li key={c.monitor_id} className="flex flex-wrap items-center gap-3 border-b border-[#f3f4f4] py-2 last:border-b-0">
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="truncate text-[14px] text-[#1b1b1b]">{c.name}</span>
                  {c.url !== '' && <span className="truncate text-[12px] text-[#8d8d8d]">{c.url}</span>}
                </span>
                <Select
                  value={c.environment}
                  disabled={!canEdit}
                  onValueChange={(v: Environment) => setChoices((list) => list.map((x) => (x.monitor_id === c.monitor_id ? { ...x, environment: v } : x)))}
                >
                  <SelectTrigger size="sm" className="h-8 w-[150px] text-[13px]" aria-label={`Environnement de ${c.name}`}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent align="end">
                    <SelectItem value="prod">{ENVIRONMENT.prod}</SelectItem>
                    <SelectItem value="preprod">{ENVIRONMENT.preprod}</SelectItem>
                  </SelectContent>
                </Select>
                {canEdit && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Retirer ${c.name}`}
                    onClick={() => setChoices((list) => list.filter((x) => x.monitor_id !== c.monitor_id))}
                  >
                    <HugeiconsIcon icon={Cancel01Icon} size={16} strokeWidth={1.8} />
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
        {canEdit && <SaveBar dirty={dirty} pending={save.isPending} onCancel={() => setChoices(initial)} />}
      </Card>
    </form>
  )
}

/** La recherche des sites, vingt resultats au plus. */
function useMonitorSearch(text: string, enabled: boolean) {
  const { data, isFetching } = useQuery({ ...uptimeMonitorsQuery(text, 1, 20), enabled })
  return { items: data?.items ?? [], fetching: isFetching }
}

/** Le choix d'un site parmi ceux que Kuma surveille. */
function SitePicker({ exclude, onPick }: { exclude: string[]; onPick: (m: UptimeMonitor) => void }) {
  return (
    <SearchPicker
      placeholder="Rechercher un site à lier"
      useItems={useMonitorSearch}
      keyOf={(m) => m.id}
      filter={(m) => m.present && !exclude.includes(m.id)}
      onPick={onPick}
      render={(m) => (
        <>
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="truncate text-[14px]">{m.name}</span>
            {m.url !== '' && <span className="truncate text-[12px] text-[#8d8d8d]">{m.url}</span>}
          </span>
          <MonitorStatus status={m.status} />
        </>
      )}
    />
  )
}
