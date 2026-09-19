import { Download04Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { toast } from 'sonner'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { clientListQuery, peopleQuery, projectListQuery } from '@/features/projects/api'
import { serviceOptionsQuery } from '@/features/services/api'
import { formatDuration } from '@/features/time/format'
import { presetPeriod, type Period } from '@/features/time/period'
import {
  timeReportEntriesQuery,
  timeReportQuery,
  useExportTimeReport,
  type ReportFilters,
  type ReportGroupBy,
} from '@/features/time/reports'
import { HttpError } from '@/lib/api'

import { ReportChart } from './-chart'
import { ReportEntries } from './-entries'
import { ReportGroups } from './-groups'
import { PeriodPicker } from './-period-picker'

const day = z.string().regex(/^\d{4}-\d{2}-\d{2}$/)

/**
 * Tout l'etat de l'ecran vit dans l'adresse : un rapport se partage par son
 * lien, et le bouton Retour defait le dernier filtre.
 */
const searchSchema = z.object({
  from: day.optional().catch(undefined),
  to: day.optional().catch(undefined),
  project_id: z.string().optional(),
  user_id: z.string().optional(),
  service_id: z.string().optional(),
  client_id: z.string().optional(),
  billable: z.enum(['true', 'false']).optional().catch(undefined),
  group_by: z.enum(['project', 'user', 'service', 'client']).catch('project'),
  group_page: z.number().int().min(1).catch(1),
  page: z.number().int().min(1).catch(1),
})

type Search = z.infer<typeof searchSchema>

export const Route = createFileRoute('/_app/pm/temps/rapports/')({
  validateSearch: searchSchema,
  component: RapportsTempsPage,
})

/** Axe du tableau qui correspond a chaque filtre : cliquer une ligne le pose. */
const PICK_FILTER: Record<ReportGroupBy, keyof Search> = {
  project: 'project_id',
  user: 'user_id',
  service: 'service_id',
  client: 'client_id',
}

const PERCENT = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 0 })

function RapportsTempsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  // Sans periode dans l'adresse, le mois en cours : c'est le rapport qu'on
  // ouvre pour boucler le mois.
  const month = presetPeriod('month')
  const period: Period = {
    from: search.from ?? month.from,
    to: search.to ?? month.to,
  }

  const filters: ReportFilters = {
    ...period,
    project_id: search.project_id,
    user_id: search.user_id,
    service_id: search.service_id,
    client_id: search.client_id,
    billable: search.billable === undefined ? undefined : search.billable === 'true',
  }

  const report = useQuery(timeReportQuery(filters, search.group_by, search.group_page))
  const entries = useQuery(timeReportEntriesQuery(filters, search.page))

  const { data: projects } = useQuery(
    projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }),
  )
  const { data: people } = useQuery(peopleQuery)
  const { data: services } = useQuery(serviceOptionsQuery())
  const { data: clients } = useQuery(clientListQuery())

  const exportCsv = useExportTimeReport()

  /** Un filtre change la vue entiere : retour en premiere page des deux listes. */
  function setFilters(patch: Partial<Search>) {
    void navigate({
      search: (prev) => ({ ...prev, ...patch, group_page: 1, page: 1 }),
      replace: true,
    })
  }

  function setPeriod(next: Period) {
    const isMonth = next.from === month.from && next.to === month.to
    setFilters({
      from: isMonth ? undefined : next.from,
      to: isMonth ? undefined : next.to,
    })
  }

  const projectOptions: Option[] = (projects?.items ?? []).map((p) => ({
    value: p.id,
    label: p.name,
  }))
  const peopleOptions: Option[] = (people?.items ?? []).map((p) => ({
    value: p.id,
    label: `${p.firstname} ${p.lastname}`.trim() || 'Sans nom',
  }))
  const serviceOptions: Option[] = (services?.items ?? []).map((s) => ({
    value: s.id,
    label: s.name,
    color: s.color,
  }))
  const clientOptions: Option[] = (clients?.items ?? []).map((c) => ({
    value: c.id,
    label: c.name,
  }))
  const billableOptions: Option[] = [
    { value: 'true', label: 'Facturable', color: '#4956f4' },
    { value: 'false', label: 'Non facturable', color: '#7bd25b' },
  ]

  const active = [
    search.project_id,
    search.user_id,
    search.service_id,
    search.client_id,
    search.billable,
  ].filter(Boolean).length

  const totals = report.data?.totals
  const share = (part: number) =>
    totals === undefined || totals.minutes === 0
      ? '—'
      : `${PERCENT.format((part / totals.minutes) * 100)} %`

  return (
    <PageFrame title="Rapports de temps">
      <div className="flex flex-col gap-4 p-4">
        <div className="flex flex-wrap items-center gap-2">
          <PeriodPicker period={period} onChange={setPeriod} />

          <FilterMenu
            name="Projet"
            all="Tous les projets"
            value={search.project_id}
            options={projectOptions}
            onChange={(value) => setFilters({ project_id: value })}
          />
          <FilterMenu
            name="Personne"
            all="Toute l’équipe"
            value={search.user_id}
            options={peopleOptions}
            onChange={(value) => setFilters({ user_id: value })}
          />
          <FilterMenu
            name="Service"
            all="Tous les services"
            value={search.service_id}
            options={serviceOptions}
            onChange={(value) => setFilters({ service_id: value })}
          />
          <FilterMenu
            name="Client"
            all="Tous les clients"
            value={search.client_id}
            options={clientOptions}
            onChange={(value) => setFilters({ client_id: value })}
          />
          <FilterMenu
            name="Facturation"
            all="Tout le temps"
            value={search.billable}
            options={billableOptions}
            onChange={(value) => setFilters({ billable: value as Search['billable'] })}
          />

          {active > 0 && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() =>
                setFilters({
                  project_id: undefined,
                  user_id: undefined,
                  service_id: undefined,
                  client_id: undefined,
                  billable: undefined,
                })
              }
            >
              Effacer les filtres
            </Button>
          )}

          <Button
            variant="outline"
            size="lg"
            className="ml-auto gap-1.5 text-[13px]"
            disabled={exportCsv.isPending || totals?.entries === 0}
            onClick={() =>
              exportCsv.mutate(filters, {
                onError: (error) =>
                  toast.error(error instanceof HttpError ? error.message : 'Export impossible'),
              })
            }
          >
            <HugeiconsIcon icon={Download04Icon} size={16} strokeWidth={1.6} />
            {exportCsv.isPending ? 'Export…' : 'Exporter en CSV'}
          </Button>
        </div>

        {report.isError && (
          <p className="rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
            {report.error instanceof HttpError ? report.error.message : 'Chargement impossible'}
          </p>
        )}

        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <Tile
            label="Temps saisi"
            value={totals === undefined ? '—' : formatDuration(totals.minutes)}
            hint={
              totals === undefined ? '' : `${totals.entries} saisie${totals.entries > 1 ? 's' : ''}`
            }
          />
          <Tile
            label="Facturable"
            value={totals === undefined ? '—' : formatDuration(totals.billable_minutes)}
            hint={`${share(totals?.billable_minutes ?? 0)} du temps`}
            color="#4956f4"
          />
          <Tile
            label="Non facturable"
            value={totals === undefined ? '—' : formatDuration(totals.non_billable_minutes)}
            hint={`${share(totals?.non_billable_minutes ?? 0)} du temps · projets internes`}
            color="#7bd25b"
          />
          <Tile
            label="Équipe"
            value={
              totals === undefined
                ? '—'
                : `${totals.people} personne${totals.people > 1 ? 's' : ''}`
            }
            hint={
              totals === undefined
                ? ''
                : `sur ${totals.projects} projet${totals.projects > 1 ? 's' : ''}`
            }
          />
        </div>

        {report.data !== undefined && (
          <>
            <ReportChart report={report.data} />

            <ReportGroups
              report={report.data}
              groupBy={search.group_by}
              fetching={report.isPlaceholderData}
              onGroupBy={(group_by) =>
                void navigate({
                  search: (prev) => ({ ...prev, group_by, group_page: 1 }),
                  replace: true,
                })
              }
              onPick={(key) => setFilters({ [PICK_FILTER[search.group_by]]: key })}
              onPage={(group_page) =>
                void navigate({ search: (prev) => ({ ...prev, group_page }) })
              }
            />
          </>
        )}

        {report.isPending && (
          <div className="flex flex-col gap-4">
            <div className="h-[240px] animate-pulse rounded-[12px] bg-[#f4f4f4]" />
            <div className="h-[200px] animate-pulse rounded-[12px] bg-[#f4f4f4]" />
          </div>
        )}

        <ReportEntries
          page={entries.data}
          fetching={entries.isPlaceholderData}
          onPage={(page) => void navigate({ search: (prev) => ({ ...prev, page }) })}
        />
      </div>
    </PageFrame>
  )
}

/** Chiffre-cle de la periode, dans l'habillage des tuiles du tableau de bord. */
function Tile({
  label,
  value,
  hint,
  color,
}: {
  label: string
  value: string
  hint: string
  color?: string
}) {
  return (
    <div className="border-surface-sunken bg-surface flex min-w-0 flex-col gap-0.5 overflow-clip rounded-[12px] border p-0.5">
      <div className="flex flex-col gap-1 rounded-[10px] bg-white p-2">
        <p className="flex items-center gap-1.5 text-sm leading-[1.5] text-[#111]">
          {color !== undefined && (
            <span aria-hidden className="size-2 rounded-[2px]" style={{ backgroundColor: color }} />
          )}
          {label}
        </p>
        <p className="text-xl leading-[1.4] font-semibold text-[#111] tabular-nums">{value}</p>
      </div>
      <p className="truncate px-2 py-1.5 text-xs leading-[1.5] text-[#111]">{hint || ' '}</p>
    </div>
  )
}
