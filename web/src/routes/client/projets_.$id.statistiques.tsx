import {
  ArrowLeft01Icon,
  BrowserIcon,
  ChartLineData01Icon,
  ComputerIcon,
  Cursor01Icon,
  File02Icon,
  Globe02Icon,
  Link04Icon,
  SmartPhone01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'

import { Delta, Legend, Panel, TopList } from '@/features/analytics/ui'
import { WorldMap } from '@/features/analytics/world-map'
import { z } from 'zod'

import { LineChart } from '@/components/line-chart'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { activeVisitorsQuery, statisticsQuery, type PeriodKey } from '@/features/analytics/api'
import {
  browserLabel,
  countryLabel,
  derived,
  deviceLabel,
  formatNumber,
  formatPercent,
  formatSeconds,
  plainLabel,
  referrerLabel,
  variation,
  localDate,
} from '@/features/analytics/format'
import { portalProjectQuery } from '@/features/portal/api'
import { StatTile } from '@/features/portal/dashboard'
import { HttpError } from '@/lib/api'
import type { StatisticsData } from '@/types/api'

/**
 * Les statistiques du site d'un projet, pour le client : son audience sur
 * une periode, comparee a la precedente. Les chiffres viennent du cache de
 * Piilot, tenu a jour par une tache de fond ; une periode jamais demandee se
 * prepare en quelques secondes.
 */
const searchSchema = z.object({
  periode: z.enum(['7d', '30d', 'month', 'prev_month', '12m', 'custom']).optional().catch(undefined),
  du: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional().catch(undefined),
  au: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional().catch(undefined),
})

export const Route = createFileRoute('/client/projets_/$id/statistiques')({
  validateSearch: searchSchema,
  loader: ({ context, params }) => context.queryClient.ensureQueryData(portalProjectQuery(params.id)),
  component: StatisticsPage,
})

const PERIODS: { key: PeriodKey; label: string }[] = [
  { key: '7d', label: '7 derniers jours' },
  { key: '30d', label: '30 derniers jours' },
  { key: 'month', label: 'Mois en cours' },
  { key: 'prev_month', label: 'Mois précédent' },
  { key: '12m', label: '12 derniers mois' },
  { key: 'custom', label: 'Période personnalisée' },
]

const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })
const DAY_LONG = new Intl.DateTimeFormat('fr-FR', { weekday: 'long', day: 'numeric', month: 'long' })
const MONTH = new Intl.DateTimeFormat('fr-FR', { month: 'short', year: '2-digit' })
const MONTH_LONG = new Intl.DateTimeFormat('fr-FR', { month: 'long', year: 'numeric' })
const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

function StatisticsPage() {
  const { id } = Route.useParams()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const period: PeriodKey = search.periode ?? '30d'
  const custom = period === 'custom' && search.du !== undefined && search.au !== undefined

  const { data: project } = useQuery(portalProjectQuery(id))
  const stats = useQuery({ ...statisticsQuery(id, custom || period !== 'custom' ? period : '30d', search.du, search.au), enabled: period !== 'custom' || custom })

  if (project === undefined) return null

  const notAvailable = stats.error instanceof HttpError && stats.error.status === 404
  if (!project.statistics || notAvailable) {
    return (
      <Shell projectId={id} projectName={project.name}>
        <Notice>Les statistiques ne sont pas disponibles pour ce projet. L’agence peut les activer pour votre site.</Notice>
      </Shell>
    )
  }

  const view = stats.data
  const data = view?.status === 'ready' ? view.data : null

  return (
    <Shell
      projectId={id}
      projectName={project.name}
      domain={view?.website.domain}
      actions={
        <>
          <ActiveBadge projectId={id} />
          <PeriodPicker
            period={period}
            from={search.du}
            to={search.au}
            onChange={(next) => void navigate({ search: next, replace: true })}
          />
        </>
      }
    >
      {stats.error instanceof HttpError && stats.error.status === 429 && <Notice tone="warn">{stats.error.message}</Notice>}
      {period === 'custom' && !custom && <Notice>Choisissez les dates de la période.</Notice>}
      {view?.status === 'error' && <Notice tone="warn">{view.message} Nous réessayons automatiquement.</Notice>}
      {(view === undefined || view.status === 'pending') && period !== 'custom' ? <Preparing /> : null}
      {view?.status === 'pending' && period === 'custom' && <Preparing />}
      {stats.error !== null && !(stats.error instanceof HttpError && (stats.error.status === 404 || stats.error.status === 429)) && (
        <Notice tone="warn">Les statistiques sont momentanément indisponibles. Réessayez dans quelques minutes.</Notice>
      )}

      {data !== null && data !== undefined && <Dashboard data={data} />}
      {view?.fetched_at !== null && view?.fetched_at !== undefined && (
        <p className="text-[12px] text-[#8d8d8d]">
          Chiffres mis à jour le {WHEN.format(new Date(view.fetched_at))}, mesurés par Umami. Heures de {data?.period.timezone ?? 'l’agence'}.
        </p>
      )}
    </Shell>
  )
}

function Shell({
  projectId,
  projectName,
  domain,
  actions,
  children,
}: {
  projectId: string
  projectName: string
  domain?: string
  actions?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3">
        <Link to="/client/projets/$id" params={{ id: projectId }} className="flex items-center gap-1 text-[14px] text-[#73757c] hover:text-[#1b1b1b]">
          <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
          {projectName}
        </Link>
        <header className="flex flex-wrap items-end justify-between gap-3">
          <div className="flex min-w-0 flex-col gap-0.5">
            <h1 className="font-heading text-[24px] leading-tight font-medium text-[#1b1b1b]">Statistiques</h1>
            {domain !== undefined && domain !== '' && <p className="text-[14px] text-[#73757c]">{domain}</p>}
          </div>
          <div className="flex flex-wrap items-center gap-2">{actions}</div>
        </header>
      </div>
      {children}
    </div>
  )
}

function Notice({ children, tone }: { children: ReactNode; tone?: 'warn' }) {
  return (
    <p
      className={`rounded-[14px] border p-4 text-[14px] ${tone === 'warn' ? 'border-[#ffe8b7] bg-[#fffaf0] text-[#9a6a00]' : 'border-[#e8e8e9] bg-white text-[#4b4b4f]'}`}
    >
      {children}
    </p>
  )
}

function Preparing() {
  return (
    <div className="flex flex-col gap-3" aria-busy>
      <p className="text-[14px] text-[#73757c]">Préparation des chiffres de cette période…</p>
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        {[0, 1, 2, 3, 4].map((i) => (
          <div key={i} className="h-[104px] animate-pulse rounded-[12px] bg-[#f3f4f4]" />
        ))}
      </div>
      <div className="h-[300px] animate-pulse rounded-[14px] bg-[#f3f4f4]" />
    </div>
  )
}

/** Les visiteurs du moment, relus toutes les trente secondes. */
function ActiveBadge({ projectId }: { projectId: string }) {
  const { data } = useQuery(activeVisitorsQuery(projectId))
  if (data === undefined || data.visitors === null) return null
  return (
    <span className="flex h-8 items-center gap-2 rounded-full border border-[#afe3ca] bg-[#f1fbf6] px-3 text-[13px] text-[#006f1f]">
      <span aria-hidden className="relative flex size-2">
        <span className="absolute inline-flex size-full animate-ping rounded-full bg-[#0db471] opacity-60" />
        <span className="relative inline-flex size-2 rounded-full bg-[#0db471]" />
      </span>
      <span className="tabular-nums">{data.visitors}</span> visiteur{data.visitors > 1 ? 's' : ''} en ce moment
    </span>
  )
}

function PeriodPicker({
  period,
  from,
  to,
  onChange,
}: {
  period: PeriodKey
  from?: string
  to?: string
  onChange: (next: { periode: PeriodKey; du?: string; au?: string }) => void
}) {
  const [open, setOpen] = useState(period === 'custom' && (from === undefined || to === undefined))
  const today = new Date().toISOString().slice(0, 10)
  const [du, setDu] = useState(from ?? '')
  const [au, setAu] = useState(to ?? today)

  return (
    <div className="flex items-center gap-2">
      <Select
        value={period}
        onValueChange={(v: PeriodKey) => {
          if (v === 'custom') {
            setOpen(true)
            onChange({ periode: 'custom', du: from, au: to })
            return
          }
          onChange({ periode: v })
        }}
      >
        <SelectTrigger size="sm" className="h-8 w-[200px] bg-white text-[13px]" aria-label="Période">
          <SelectValue />
        </SelectTrigger>
        <SelectContent align="end">
          {PERIODS.map((p) => (
            <SelectItem key={p.key} value={p.key}>
              {p.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {period === 'custom' && (
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <Button type="button" variant="outline" size="sm" className="h-8 text-[13px] tabular-nums">
              {from !== undefined && to !== undefined ? `${DAY.format(localDate(from))} → ${DAY.format(localDate(to))}` : 'Choisir les dates'}
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" className="flex w-[280px] flex-col gap-3 p-3">
            <label className="flex flex-col gap-1 text-[13px] text-[#4b4b4f]">
              Du
              <Input type="date" value={du} max={au || today} onChange={(e) => setDu(e.target.value)} />
            </label>
            <label className="flex flex-col gap-1 text-[13px] text-[#4b4b4f]">
              Au
              <Input type="date" value={au} min={du} max={today} onChange={(e) => setAu(e.target.value)} />
            </label>
            <p className="text-[12px] text-[#8d8d8d]">Vingt-quatre mois au plus.</p>
            <Button
              type="button"
              size="sm"
              disabled={du === '' || au === '' || du > au}
              onClick={() => {
                setOpen(false)
                onChange({ periode: 'custom', du, au })
              }}
            >
              Appliquer
            </Button>
          </PopoverContent>
        </Popover>
      )}
    </div>
  )
}

function Dashboard({ data }: { data: StatisticsData }) {
  const cur = derived(data.totals)
  const prev = derived(data.previous)
  const empty = data.totals.visits === 0 && data.totals.pageviews === 0

  const tiles: { label: string; value: string; delta: number | null; lowerIsBetter?: boolean }[] = [
    { label: 'Visiteurs', value: formatNumber(data.totals.visitors), delta: variation(data.totals.visitors, data.previous.visitors) },
    { label: 'Visites', value: formatNumber(data.totals.visits), delta: variation(data.totals.visits, data.previous.visits) },
    { label: 'Pages vues', value: formatNumber(data.totals.pageviews), delta: variation(data.totals.pageviews, data.previous.pageviews) },
    {
      label: 'Taux de rebond',
      value: cur.bounceRate === null ? '—' : formatPercent(cur.bounceRate),
      delta: variation(cur.bounceRate, prev.bounceRate),
      lowerIsBetter: true,
    },
    { label: 'Durée moyenne', value: cur.avgDuration === null ? '—' : formatSeconds(cur.avgDuration), delta: variation(cur.avgDuration, prev.avgDuration) },
  ]

  const unit = data.period.unit
  const formatX = (at: string) => (unit === 'month' ? MONTH.format(localDate(at)) : unit === 'hour' ? `${localDate(at).getHours()} h` : DAY.format(localDate(at)))
  const formatTooltipX = (at: string) =>
    unit === 'month' ? MONTH_LONG.format(localDate(at)) : unit === 'hour' ? `${DAY_LONG.format(localDate(at))}, ${localDate(at).getHours()} h` : DAY_LONG.format(localDate(at))

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        {tiles.map((t) => (
          <StatTile
            key={t.label}
            label={t.label}
            value={t.value}
            footer="Évolution"
            aside={<Delta value={t.delta} lowerIsBetter={t.lowerIsBetter} />}
          />
        ))}
      </div>

      <Panel
        icon={ChartLineData01Icon}
        title="Visiteurs et pages vues"
        aside={
          <span className="flex items-center gap-4 text-[12px] text-[#73757c]">
            <Legend color="#4956f4" label="Visiteurs" />
            <Legend color="#ff782b" label="Pages vues" />
          </span>
        }
      >
        {empty ? (
          <p className="py-16 text-center text-[14px] text-[#73757c]">Aucune visite sur cette période.</p>
        ) : (
          <LineChart
            points={data.series}
            series={[
              { key: 'visitors', label: 'Visiteurs', color: '#4956f4', area: true },
              { key: 'pageviews', label: 'Pages vues', color: '#ff782b' },
            ]}
            value={(p, key) => (key === 'visitors' ? p.visitors : p.pageviews)}
            formatX={formatX}
            formatTooltipX={formatTooltipX}
          />
        )}
      </Panel>

      <CountriesPanel rows={data.countries} />

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <TopList icon={File02Icon} title="Pages les plus vues" unit="vues" rows={data.pages} label={(v) => (v === '' ? '/' : v)} />
        <TopList icon={Link04Icon} title="Sources de trafic" unit="visiteurs" rows={data.referrers} label={referrerLabel} />
        <TopList icon={SmartPhone01Icon} title="Appareils" unit="visiteurs" rows={data.devices} label={deviceLabel} />
        <TopList icon={BrowserIcon} title="Navigateurs" unit="visiteurs" rows={data.browsers} label={browserLabel} />
        <TopList icon={ComputerIcon} title="Systèmes" unit="visiteurs" rows={data.os} label={plainLabel} />
        {data.events.length > 0 && <TopList icon={Cursor01Icon} title="Évènements" unit="fois" rows={data.events} label={plainLabel} />}
      </div>
    </div>
  )
}

/**
 * Les pays : la carte du monde, et a cote les dix premiers. Umami en rend
 * cinquante, pour que la carte ne s'arrete pas au dixieme.
 */
function CountriesPanel({ rows }: { rows: StatisticsData['countries'] }) {
  const top = rows.slice(0, 10)
  const max = Math.max(1, ...top.map((r) => r.count))
  return (
    <Panel
      icon={Globe02Icon}
      title="Pays"
      aside={<span className="text-[12px] text-[#8d8d8d]">visiteurs</span>}
      contentClassName="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]"
    >
      <WorldMap rows={rows} />
      {top.length === 0 ? (
        <p className="py-6 text-center text-[13px] text-[#73757c]">Rien sur cette période.</p>
      ) : (
        <ul className="flex flex-col gap-1">
          {top.map((r) => {
            const c = countryLabel(r.value)
            return (
              <li key={r.value} className="relative flex items-center gap-3 overflow-hidden rounded-[8px] px-2 py-1.5 text-[13px] text-[#1b1b1b]">
                <span aria-hidden className="absolute inset-y-0 left-0 rounded-[8px] bg-[#fff2ea]" style={{ width: `${(r.count / max) * 100}%` }} />
                <span className="relative flex min-w-0 flex-1 items-center gap-2">
                  <span aria-hidden>{c.flag}</span>
                  <span className="truncate">{c.name}</span>
                </span>
                <span className="relative shrink-0 tabular-nums">{formatNumber(r.count)}</span>
              </li>
            )
          })}
        </ul>
      )}
    </Panel>
  )
}
