import { File02Icon, Link04Icon } from '@hugeicons/core-free-icons'
import type { ReactNode } from 'react'

import { LineChart } from '@/components/line-chart'
import { derived, formatNumber, formatPercent, formatSeconds, referrerLabel, variation } from '@/features/analytics/format'
import { Delta, Legend, TopList } from '@/features/analytics/ui'
import { monthLabel } from '@/features/maintenance/format'
import { ConsumptionBar, KindPill } from '@/features/maintenance/ui'
import { formatDuration } from '@/features/time/format'
import { formatDuration as formatIncident, formatUptime } from '@/features/uptime/format'
import { UptimeStrip } from '@/features/uptime/ui'
import type { ReportData } from '@/types/api'

/**
 * Le rapport mensuel, tel que le client le lit : l'apercu de l'agence,
 * l'archive du portail et l'impression en PDF rendent ce meme composant, a
 * partir des donnees figees du rapport. Les graphiques sont ceux de
 * l'application, en SVG.
 */
const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long' })
const DAY_SHORT = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })
const TIME = new Intl.DateTimeFormat('fr-FR', { hour: '2-digit', minute: '2-digit' })

export function ReportView({ data, editorial }: { data: ReportData; editorial: string }) {
  return (
    <article className="flex flex-col gap-6 rounded-[14px] border border-[#e8e8e9] bg-white p-5 sm:p-8 print:border-0 print:p-0">
      <header className="flex flex-col gap-1 border-b border-[#f0f0f1] pb-5">
        <p className="text-[13px] tracking-wide text-[#ff782b] uppercase">Rapport de maintenance</p>
        <h1 className="font-heading text-[28px] leading-tight font-medium text-[#1b1b1b]">{monthLabel(data.month)}</h1>
        <p className="text-[14px] text-[#73757c]">
          {data.project.name}
          {data.project.client_name !== '' && ` · ${data.project.client_name}`} · formule {data.plan.name}
        </p>
      </header>

      {editorial.trim() !== '' && (
        <Section title="Le mot du mois">
          <div className="flex flex-col gap-2 rounded-[10px] border-l-[3px] border-[#ff782b] bg-[#fff8f4] px-4 py-3">
            {editorial.split('\n').filter((p) => p.trim() !== '').map((p, i) => (
              <p key={i} className="text-[15px] leading-relaxed text-[#1b1b1b]">
                {p}
              </p>
            ))}
          </div>
        </Section>
      )}

      {data.availability !== null && <Availability section={data.availability} />}
      {data.audience !== null && <Audience section={data.audience} />}
      {data.interventions !== null && <Interventions section={data.interventions} />}
      {data.deployments !== null && <Deployments section={data.deployments} />}
    </article>
  )
}

function Section({ title, aside, children }: { title: string; aside?: ReactNode; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <header className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 className="text-[17px] font-medium text-[#1b1b1b]">{title}</h2>
        {aside}
      </header>
      {children}
    </section>
  )
}

function Figure({ label, value, aside }: { label: string; value: string; aside?: ReactNode }) {
  return (
    <div className="flex flex-col gap-1 rounded-[10px] bg-[#f6f6f7] px-3 py-2.5">
      <span className="text-[12px] text-[#73757c]">{label}</span>
      <span className="font-heading text-[20px] leading-tight font-medium whitespace-nowrap text-[#1b1b1b] tabular-nums">{value}</span>
      {aside !== undefined && <span className="text-[12px]">{aside}</span>}
    </div>
  )
}

function Availability({ section }: { section: NonNullable<ReportData['availability']> }) {
  return (
    <Section title="Disponibilité">
      {section.sites.map((s) => (
        <div key={s.name + s.environment} className="flex flex-col gap-3">
          <p className="text-[14px] text-[#1b1b1b]">
            {s.name}
            <span className="text-[#8d8d8d]"> · {s.environment === 'prod' ? 'production' : 'préproduction'}</span>
          </p>
          <div className="grid grid-cols-2 gap-2 break-inside-avoid sm:grid-cols-3">
            <Figure label="Disponibilité" value={formatUptime(s.uptime)} />
            <Figure label="Temps de réponse moyen" value={s.avg_response_ms === null ? '—' : `${s.avg_response_ms} ms`} />
            <Figure label="Certificat SSL" value={s.cert_days === null ? '—' : `${s.cert_days} jours`} />
          </div>
          <UptimeStrip days={s.days} />
        </div>
      ))}
      {section.incidents.length === 0 ? (
        <p className="text-[14px] text-[#006f1f]">Aucun incident ce mois-ci.</p>
      ) : (
        <ul className="flex flex-col">
          {section.incidents.map((i) => (
            <li key={i.started_at + i.site} className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 border-b border-[#f3f4f4] py-2 text-[14px] last:border-b-0">
              <span className="w-[130px] text-[13px] text-[#73757c]">
                {DAY.format(new Date(i.started_at))}, {TIME.format(new Date(i.started_at))}
              </span>
              <span className="min-w-0 flex-1 text-[#1b1b1b]">
                Interruption de {i.site}
                {i.message !== '' && <span className="text-[#73757c]"> · {i.message}</span>}
              </span>
              <span className="text-[13px] text-[#4b4b4f] tabular-nums">
                {i.duration_seconds === null ? 'en cours à la fin du mois' : formatIncident(i.duration_seconds)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Section>
  )
}

function Audience({ section }: { section: NonNullable<ReportData['audience']> }) {
  if (section.unavailable || section.data === null) {
    return (
      <Section title="Audience">
        <p className="text-[14px] text-[#73757c]">Les chiffres d’audience n’étaient pas disponibles à la préparation de ce rapport.</p>
      </Section>
    )
  }
  const d = section.data
  const cur = derived(d.totals)
  const prev = derived(d.previous)
  return (
    <Section title="Audience" aside={<span className="text-[13px] text-[#73757c]">{section.domain}</span>}>
      <div className="grid grid-cols-2 gap-2 break-inside-avoid sm:grid-cols-4">
        <Figure label="Visiteurs" value={formatNumber(d.totals.visitors)} aside={<Delta value={variation(d.totals.visitors, d.previous.visitors)} />} />
        <Figure label="Pages vues" value={formatNumber(d.totals.pageviews)} aside={<Delta value={variation(d.totals.pageviews, d.previous.pageviews)} />} />
        <Figure
          label="Taux de rebond"
          value={cur.bounceRate === null ? '—' : formatPercent(cur.bounceRate)}
          aside={<Delta value={variation(cur.bounceRate, prev.bounceRate)} lowerIsBetter />}
        />
        <Figure label="Durée moyenne" value={cur.avgDuration === null ? '—' : formatSeconds(cur.avgDuration)} aside={<Delta value={variation(cur.avgDuration, prev.avgDuration)} />} />
      </div>
      <p className="text-[12px] text-[#8d8d8d]">Variations par rapport au mois précédent.</p>
      {d.series.length > 0 && (
        <div className="flex flex-col gap-2 break-inside-avoid">
          <span className="flex gap-4 text-[12px] text-[#73757c]">
            <Legend color="#4956f4" label="Visiteurs" />
            <Legend color="#ff782b" label="Pages vues" />
          </span>
          <LineChart
            points={d.series}
            series={[
              { key: 'visitors', label: 'Visiteurs', color: '#4956f4', area: true },
              { key: 'pageviews', label: 'Pages vues', color: '#ff782b' },
            ]}
            value={(p, key) => (key === 'visitors' ? p.visitors : p.pageviews)}
            formatX={(at) => DAY_SHORT.format(new Date(at))}
            formatTooltipX={(at) => DAY.format(new Date(at))}
            height={200}
          />
        </div>
      )}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <TopList icon={File02Icon} title="Pages les plus vues" unit="vues" rows={d.pages.slice(0, 6)} label={(v) => (v === '' ? '/' : v)} />
        <TopList icon={Link04Icon} title="Sources de trafic" unit="visiteurs" rows={d.referrers.slice(0, 6)} label={referrerLabel} />
      </div>
    </Section>
  )
}

function Interventions({ section }: { section: NonNullable<ReportData['interventions']> }) {
  const internal = section.consumed_minutes - section.visible_minutes
  return (
    <Section title="Interventions">
      <ConsumptionBar consumed={section.consumed_minutes} included={section.included_minutes} />
      {internal > 0 && <p className="text-[12px] text-[#8d8d8d]">Dont {formatDuration(internal)} d’interventions techniques internes.</p>}
      {section.items.length === 0 ? (
        <p className="text-[14px] text-[#73757c]">Aucune intervention ce mois-ci.</p>
      ) : (
        <ul className="flex flex-col">
          {section.items.map((i, n) => (
            <li key={n} className="flex flex-wrap items-baseline gap-x-3 gap-y-1 border-b border-[#f3f4f4] py-2 last:border-b-0">
              <span className="w-[80px] text-[13px] text-[#73757c]">{DAY_SHORT.format(new Date(i.spent_on))}</span>
              <span className="min-w-0 flex-1 text-[14px] text-[#1b1b1b]">{i.note}</span>
              <KindPill kind={i.kind} />
              <span className="w-[56px] text-right text-[13px] text-[#4b4b4f] tabular-nums">{formatDuration(i.minutes)}</span>
            </li>
          ))}
        </ul>
      )}
    </Section>
  )
}

function Deployments({ section }: { section: NonNullable<ReportData['deployments']> }) {
  return (
    <Section title="Mises en ligne">
      <ul className="flex flex-col">
        {section.items.map((d) => (
          <li key={d.name + d.deployed_at} className="flex flex-wrap items-baseline gap-x-3 border-b border-[#f3f4f4] py-2 text-[14px] last:border-b-0">
            <span className="w-[80px] text-[13px] text-[#73757c]">{DAY_SHORT.format(new Date(d.deployed_at))}</span>
            <span className="min-w-0 flex-1 text-[#1b1b1b]">{d.name}</span>
            {d.environment !== '' && <span className="text-[13px] text-[#73757c]">{d.environment}</span>}
          </li>
        ))}
      </ul>
    </Section>
  )
}
