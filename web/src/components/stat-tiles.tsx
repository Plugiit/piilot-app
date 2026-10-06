import { ArrowDownRight01Icon, ArrowUpRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'

import { StatCard } from '@/components/stat-card'
import { dashboardQuery } from '@/features/projects/api'
import { cn } from '@/lib/utils'
import type { DashboardMetric } from '@/types/api'

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
}: {
  label: string
  metric: DashboardMetric | undefined
  /** Rend la valeur : un compte n'a pas d'unite, des heures en ont une. */
  format: (value: number) => string
  loading: boolean
}) {
  const change = metric?.change ?? null
  const rising = change !== null && change >= 0

  return (
    <StatCard
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
export function StatTiles() {
  const { data, isPending } = useQuery(dashboardQuery)

  return (
    <div className="flex w-full items-start gap-4">
      <StatTile
        label="Nombre de projets"
        metric={data?.projects}
        format={(value) => NUMBER.format(value)}
        loading={isPending}
      />
      <StatTile
        label="Nombre de clients"
        metric={data?.clients}
        format={(value) => NUMBER.format(value)}
        loading={isPending}
      />
      <StatTile
        label="Heures vendues"
        metric={data?.hours_sold}
        format={(value) => `${NUMBER.format(value)} h`}
        loading={isPending}
      />
    </div>
  )
}
