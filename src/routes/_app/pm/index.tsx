import { createFileRoute } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'framer-motion'

import { ActivityHeatmap } from '@/components/activity-heatmap'
import { usePageAside } from '@/components/layout/aside-context'
import { PageFrame } from '@/components/layout/page-frame'
import { PerformanceReview } from '@/components/performance-review'
import { SchedulePanel } from '@/components/schedule-panel'
import { StatTiles } from '@/components/stat-tiles'
import { useSlideTransition } from '@/lib/motion'
import { TeamActivity } from '@/components/team-activity'
import { TimeBillable } from '@/components/time-billable'
import { WorkloadChart } from '@/components/workload-chart'

/**
 * Tableau de bord du module Project Management.
 *
 * Le reste de l'ecran attend le modele de donnees du module : seule
 * l'activite est posee, et ses intensites sont encore figees.
 */
export const Route = createFileRoute('/_app/pm/')({
  component: ProjectManagementPage,
})

function ProjectManagementPage() {
  return (
    <PageFrame
      title="Tableau de bord"
      hasAside
    >
      <DashboardBody />
    </PageFrame>
  )
}

/**
 * Corps du tableau de bord.
 *
 * Separe de la page parce qu'il lit l'etat du panneau lateral, que `PageFrame`
 * fournit : un composant ne peut pas consommer un contexte qu'il pose
 * lui-meme.
 */
function DashboardBody() {
  const aside = usePageAside()
  const transition = useSlideTransition()

  // L'agenda passe a droite quand la ligne peut porter les deux, et revient
  // sous le contenu sinon : le replier hors de vue le rendrait invisible
  // partout ailleurs que sur un grand ecran.
  //
  // Des que les deux colonnes sont cote a cote, chacune defile pour son
  // compte : le cadre prend la hauteur disponible et bloque son propre
  // debordement, si bien que la molette agit sur la colonne survolee et non
  // sur la page entiere.
  //
  // Empilees, elles retrouvent un defilement unique : deux zones
  // independantes l'une au-dessus de l'autre, sur un telephone, ne donnent
  // qu'une page ou l'on se perd.
  return (
      <div className="flex min-h-full flex-col xl:h-full xl:min-h-0 xl:flex-row xl:overflow-hidden">
        <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 xl:min-h-0 xl:overflow-y-auto">
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

        {/* L'agenda n'ouvre plus le tiroir : ses evenements sont un decor de
            maquette, alors que le tiroir affiche desormais des taches reelles.
            Il le rouvrira quand le planning lira les vraies echeances. */}
        {/* Le repli se joue sur la largeur : l'agenda ne glisse pas par-dessus
            le contenu, il lui rend sa place. `width: auto` a l'ouverture laisse
            Framer mesurer la colonne, si bien que la meme animation vaut pour
            les 340px de la colonne et pour la pleine largeur empilee.

            `overflow-hidden` borne le panneau pendant le trajet, sans quoi son
            contenu deborderait du cadre a mesure qu'il se referme. */}
        <AnimatePresence initial={false}>
          {aside?.open !== false && (
            <motion.div
              key="agenda"
              initial={{ width: 0, opacity: 0 }}
              animate={{ width: 'auto', opacity: 1 }}
              exit={{ width: 0, opacity: 0 }}
              transition={transition}
              className="shrink-0 overflow-hidden xl:h-full"
            >
              <SchedulePanel className="border-surface-sunken w-full shrink-0 border-t xl:h-full xl:w-[340px] xl:overflow-y-auto xl:border-t-0 xl:border-l" />
            </motion.div>
          )}
        </AnimatePresence>
      </div>
  )
}
