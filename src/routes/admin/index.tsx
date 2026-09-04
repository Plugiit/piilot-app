import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { ErrorState } from '@/components/layout/error-state'
import { PageFrame } from '@/components/layout/page-frame'
import { StatCard } from '@/components/stat-card'
import { dashboardQuery } from '@/features/projects/api'

export const Route = createFileRoute('/admin/')({
  // Le loader precharge les agregats : combine au prefetch au survol du
  // lien, les donnees sont deja en cache quand la vue se monte.
  // `staleTime: 'static'` rend le cache existant sans le revalider — c'est
  // useQuery, dans le composant, qui porte la politique de fraicheur.
  loader: ({ context }) => context.queryClient.query({ ...dashboardQuery, staleTime: 'static' }),
  // L'erreur remplace le composant de la route : sans la frame ici, le chassis
  // de la page disparaitrait avec lui et l'erreur flotterait sur le fond.
  errorComponent: (props) => (
    <DashboardFrame>
      <ErrorState {...props} />
    </DashboardFrame>
  ),
  component: DashboardPage,
})

/**
 * Chassis du tableau de bord.
 *
 * Partage par la vue et par son etat d'erreur : l'accueil ne depend d'aucun
 * agregat, il n'a donc pas de raison de disparaitre quand l'endpoint echoue.
 * Le tenir en un seul endroit evite qu'il ne s'affiche que dans la branche qui
 * ne s'execute pas — les endpoints metier repondent encore 501.
 */
function DashboardFrame({ children }: { children: ReactNode }) {
  const { user } = Route.useRouteContext()

  return (
    <PageFrame
      title={`Bon retour, ${user.firstname} !`}
      description={
        <>
          Vous avez <span className="font-semibold text-[#ff782b]">2 tâches</span> assignées
          aujourd'hui.
        </>
      }
    >
      {children}
    </PageFrame>
  )
}

function DashboardPage() {
  const { data, isPending } = useQuery(dashboardQuery)

  return (
    <DashboardFrame>
      <div className="p-4 sm:p-6">
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <StatCard label="Projets actifs" value={data?.active_projects} loading={isPending} />
          <StatCard label="Tâches en cours" value={data?.tasks_in_progress} loading={isPending} />
          <StatCard
            label="Tâches en retard"
            value={data?.overdue_tasks}
            loading={isPending}
            tone="danger"
          />
          <StatCard label="Tickets ouverts" value={data?.open_tickets} loading={isPending} />
        </div>
      </div>
    </DashboardFrame>
  )
}
