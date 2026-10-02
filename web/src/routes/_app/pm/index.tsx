import { createFileRoute, redirect } from '@tanstack/react-router'

import { ActivityHeatmap } from '@/components/activity-heatmap'
import { PageFrame } from '@/components/layout/page-frame'
import { PerformanceReview } from '@/components/performance-review'
import { StatTiles } from '@/components/stat-tiles'
import { TeamActivity } from '@/components/team-activity'
import { TimeBillable } from '@/components/time-billable'
import { WorkloadChart } from '@/components/workload-chart'
import { can } from '@/lib/auth'

/**
 * Tableau de bord du module Project Management.
 *
 * Tous les blocs lisent le meme appel (`GET /admin/dashboard`) : TanStack
 * Query le partage entre eux, l'ecran ne coute qu'une requete.
 *
 * L'agenda qui occupait la colonne de droite a ete retire : ses evenements
 * etaient un decor de maquette. Il reviendra avec le planning, quand il
 * pourra lire de vraies echeances.
 */
export const Route = createFileRoute('/_app/pm/')({
  // Le tableau de bord de l'agence est un outil de direction : l'equipe commence sa journee par « Mon travail ».
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'dashboard.read')) throw redirect({ to: '/pm/mon-travail', replace: true })
  },
  component: ProjectManagementPage,
})

function ProjectManagementPage() {
  return (
    <PageFrame title="Tableau de bord">
      <div className="flex min-h-full flex-col gap-4 p-4">
        <StatTiles />
        <ActivityHeatmap />
        {/* Deux colonnes et pas trois : chaque bloc porte un axe gradue ou
            des lignes titrees, qui se tronquent des un tiers de grille. La
            heatmap reste seule sur sa ligne, c'est la seule a s'etendre. */}
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <TimeBillable />
          <PerformanceReview />
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <WorkloadChart />
          <TeamActivity />
        </div>
      </div>
    </PageFrame>
  )
}
