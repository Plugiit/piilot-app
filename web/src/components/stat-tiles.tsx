import { ArrowDownRight01Icon, ArrowUpRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'

import { AreaCurve } from '@/components/area-curve'
import { StatCard } from '@/components/stat-card'
import { dashboardQuery } from '@/features/projects/api'
import { PROJECT_STATUS, PROJECT_STATUS_ORDER } from '@/features/projects/format'
import { cn } from '@/lib/utils'
import type { DashboardMetric, DashboardMonth, StatusCount } from '@/types/api'

const NUMBER = new Intl.NumberFormat('fr-FR')
const PERCENT = new Intl.NumberFormat('fr-FR', {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
  signDisplay: 'exceptZero',
})

/**
 * Tuile de chiffre-cle.
 *
 * Deux etages dans un meme cadre : la valeur sur une carte blanche, la
 * comparaison dans le creux gris en dessous. C'est le creux qui separe les
 * deux lectures — ce qu'on vaut aujourd'hui, et le sens dans lequel on va.
 */
function StatTile({
  label,
  metric,
  format,
  loading,
  chart,
}: {
  label: string
  metric: DashboardMetric | undefined
  /** Rend la valeur : un compte n'a pas d'unite, des heures en ont une. */
  format: (value: number) => string
  loading: boolean
  chart?: ReactNode
}) {
  const change = metric?.change ?? null
  const rising = change !== null && change >= 0

  return (
    <StatCard
      chart={chart}
      label={label}
      value={loading || metric === undefined ? '—' : format(metric.value)}
      footer={
        <>
          <p className="min-w-px flex-1 text-xs leading-[1.5] text-[#111]">
            {/* Sans periode precedente, il n'y a rien a comparer : le dire vaut
                mieux qu'afficher une progression depuis zero, qui serait toujours
                spectaculaire et jamais informative. */}
            {change === null ? 'Pas encore de comparaison' : 'Que le mois dernier'}
          </p>

          {change !== null && (
            <div className="flex shrink-0 items-center gap-1">
              <HugeiconsIcon
                icon={rising ? ArrowUpRight01Icon : ArrowDownRight01Icon}
                size={12}
                strokeWidth={2}
                className={rising ? 'text-[#006f1f]' : 'text-[#ff4345]'}
              />
              <p
                className={cn(
                  'text-xs leading-[1.5] whitespace-nowrap tabular-nums',
                  rising ? 'text-[#006f1f]' : 'text-[#ff4345]',
                )}
              >
                {PERCENT.format(change)} %
              </p>
            </div>
          )}
        </>
      }
    />
  )
}

/**
 * Rangee de chiffres-cles du module.
 *
 * Pas de chiffre d'affaires : la facturation ne fait pas partie de ce projet.
 * Les heures vendues sont la donnee la plus proche qui lui appartienne, et
 * elles se lisent de la meme facon — ce que l'agence s'est engagee a produire.
 */
const MONTH = new Intl.DateTimeFormat('fr-FR', { month: 'long', year: 'numeric' })

/**
 * Les projets par statut, en une barre empilee : un stock se lit par sa
 * composition, pas par une courbe. Les couleurs sont celles des vignettes de
 * statut ; la legende nomme les trois plus gros segments.
 */
function StatusBar({ counts }: { counts: StatusCount[] }) {
  const total = counts.reduce((sum, c) => sum + c.projects, 0)
  const ordered = PROJECT_STATUS_ORDER.map((status) => ({
    status,
    projects: counts.find((c) => c.status === status)?.projects ?? 0,
  })).filter((c) => c.projects > 0)
  const legend = [...ordered].sort((a, b) => b.projects - a.projects).slice(0, 3)

  return (
    <div
      role="img"
      aria-label={ordered.map((c) => `${c.projects} ${PROJECT_STATUS[c.status].label.toLowerCase()}`).join(', ')}
      className="flex size-full flex-col justify-end gap-2 px-2 pb-2"
    >
      <p className="flex min-w-0 items-center gap-3 overflow-hidden text-[11px] leading-none whitespace-nowrap text-[#73757c]">
        {legend.map((c) => (
          <span key={c.status} className="flex items-center gap-1">
            <span aria-hidden className="size-1.5 shrink-0 rounded-full" style={{ backgroundColor: PROJECT_STATUS[c.status].color }} />
            <span className="tabular-nums">{c.projects}</span> {PROJECT_STATUS[c.status].label.toLowerCase()}
          </span>
        ))}
      </p>
      <div className="flex h-2 w-full gap-0.5 overflow-hidden rounded-full bg-[#f3f4f4]">
        {total > 0 &&
          ordered.map((c) => (
            <span key={c.status} className="h-full" style={{ width: `${(c.projects / total) * 100}%`, backgroundColor: PROJECT_STATUS[c.status].color }} />
          ))}
      </div>
    </div>
  )
}

/**
 * Les heures vendues mois par mois, en barres : un volume par periode. Les
 * barres partent du bas exact de la carte ; le mois en cours prend l'orange,
 * les autres sa teinte claire. Un mois vide garde un trait, pour que les
 * douze se comptent.
 */
function MonthBars({ months }: { months: DashboardMonth[] }) {
  const max = Math.max(1, ...months.map((m) => m.hours_sold))

  return (
    <div
      role="img"
      aria-label={`Heures vendues par mois : ${months.map((m) => `${MONTH.format(new Date(m.month))} ${NUMBER.format(m.hours_sold)} h`).join(', ')}`}
      className="flex size-full items-end gap-1 px-2"
    >
      {months.map((m, i) => (
        <span
          key={m.month}
          className="min-w-px flex-1 rounded-t-[3px]"
          style={{
            height: m.hours_sold === 0 ? 2 : `${Math.max(8, (m.hours_sold / max) * 100)}%`,
            backgroundColor: m.hours_sold === 0 ? '#ebebeb' : i === months.length - 1 ? '#ff782b' : '#ffc9a8',
          }}
        />
      ))}
    </div>
  )
}

export function StatTiles() {
  const { data, isPending } = useQuery(dashboardQuery)

  return (
    <div className="flex w-full items-start gap-4">
      <StatTile
        label="Nombre de projets"
        metric={data?.projects}
        format={(value) => NUMBER.format(value)}
        loading={isPending}
        chart={data !== undefined && <StatusBar counts={data.projects_by_status} />}
      />
      <StatTile
        label="Nombre de clients"
        metric={data?.clients}
        format={(value) => NUMBER.format(value)}
        loading={isPending}
        // Le portefeuille a la fin de chaque mois : une courbe qui monte dit
        // que l'agence gagne des clients.
        chart={
          data !== undefined && (
            <AreaCurve
              values={data.months.map((m) => m.clients_total)}
              color="#4956f4"
              label={`Clients en portefeuille sur douze mois : ${data.months.map((m) => m.clients_total).join(', ')}`}
            />
          )
        }
      />
      <StatTile
        label="Heures vendues"
        metric={data?.hours_sold}
        format={(value) => `${NUMBER.format(value)} h`}
        loading={isPending}
        chart={data !== undefined && <MonthBars months={data.months} />}
      />
    </div>
  )
}
