import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { ActivityHeatmap } from '@/components/activity-heatmap'
import { PageFrame } from '@/components/layout/page-frame'
import { PerformanceReview } from '@/components/performance-review'
import { SchedulePanel } from '@/components/schedule-panel'
import { TeamActivity } from '@/components/team-activity'
import { TimeBillable } from '@/components/time-billable'
import { WorkloadChart } from '@/components/workload-chart'

/**
 * Tableau de bord du module Project Management.
 *
 * Le reste de l'ecran attend le modele de donnees du module : seule
 * l'activite est posee, et ses intensites sont encore figees.
 */
/**
 * La tache ouverte vit dans l'URL, pas dans un `useState` du panneau.
 *
 * Une tache ouverte est un etat qu'on partage, qu'on met en favori et qu'on
 * quitte avec le bouton Retour du navigateur : trois choses qu'un etat local
 * ne sait pas faire. Le panneau lateral reste un panneau — d'ou un parametre
 * de recherche et non une route enfant : il se superpose au tableau de bord,
 * il ne le remplace pas.
 */
const searchSchema = z.object({
  tache: z.string().optional(),
})

export const Route = createFileRoute('/_app/pm/')({
  validateSearch: searchSchema,
  component: ProjectManagementPage,
})

function ProjectManagementPage() {
  const { tache } = Route.useSearch()
  const navigate = Route.useNavigate()

  // `replace` : ouvrir puis fermer une tache ne doit pas empiler deux entrees
  // d'historique, sinon le bouton Retour rouvrirait ce qu'on vient de fermer.
  function openTask(id: string | null) {
    void navigate({ search: { tache: id ?? undefined }, replace: id === null })
  }

  return (
    <PageFrame title="Tableau de bord" description="Projets, jalons, temps passé et livrables.">
      {/* L'agenda passe a droite quand la ligne peut porter les deux, et
          revient sous le contenu sinon : le replier hors de vue le rendrait
          invisible partout ailleurs que sur un grand ecran. */}
      <div className="flex min-h-full flex-col xl:flex-row">
        <div className="flex min-w-0 flex-1 flex-col gap-4 p-4">
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

        <SchedulePanel
          openedTaskId={tache}
          onOpenTask={openTask}
          className="w-full shrink-0 border-t border-[#efefef] xl:w-[340px] xl:border-t-0 xl:border-l"
        />
      </div>
    </PageFrame>
  )
}
