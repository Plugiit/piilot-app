import { createFileRoute, redirect } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'framer-motion'

import { ActivityHeatmap } from '@/components/activity-heatmap'
import { usePageAside } from '@/components/layout/aside-context'
import { PageFrame } from '@/components/layout/page-frame'
import { PerformanceReview } from '@/components/performance-review'
import { SchedulePanel } from '@/components/schedule-panel'
import { StatTiles } from '@/components/stat-tiles'
import { TeamActivity } from '@/components/team-activity'
import { TimeBillable } from '@/components/time-billable'
import { WorkloadChart } from '@/components/workload-chart'
import { can } from '@/lib/auth'
import { useSlideTransition } from '@/lib/motion'

/**
 * Tableau de bord du module Project Management.
 *
 * Tous les blocs lisent le meme appel (`GET /admin/dashboard`) : TanStack
 * Query le partage entre eux, l'ecran ne coute qu'une requete.
 *
 * L'agenda occupe la colonne de droite : la semaine en cours, avec les
 * jalons et les echeances des projets, et ses propres taches. Il lit le
 * planning, comme l'ecran du meme nom ; il se replie depuis l'en-tete.
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
    <PageFrame title="Tableau de bord" hasAside>
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
  // sous le contenu sinon. Cote a cote, chaque colonne defile pour son compte ;
  // empilees, elles retrouvent un defilement unique.
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

      {/* Le repli se joue sur la largeur : l'agenda rend sa place au contenu
          au lieu de glisser par-dessus. `width: auto` a l'ouverture laisse
          Framer mesurer la colonne. */}
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
