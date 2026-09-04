import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { PageHeader } from '@/components/layout/page-header'
import { StatCard } from '@/components/ui/stat-card'
import { dashboardQuery } from '@/features/projects/api'

export const Route = createFileRoute('/admin/')({
  // Le loader precharge les agregats : combine au prefetch au survol du
  // lien, les donnees sont deja en cache quand la vue se monte.
  // `staleTime: 'static'` rend le cache existant sans le revalider — c'est
  // useQuery, dans le composant, qui porte la politique de fraicheur.
  loader: ({ context }) => context.queryClient.query({ ...dashboardQuery, staleTime: 'static' }),
  component: DashboardPage,
})

function DashboardPage() {
  const { data, isPending } = useQuery(dashboardQuery)

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Tableau de bord" subtitle="Vue d'ensemble de l'activité" />

      <div className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6">
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <StatCard label="Projets actifs" value={data?.active_projects} loading={isPending} />
          <StatCard label="Tâches en cours" value={data?.tasks_in_progress} loading={isPending} />
          <StatCard label="Tâches en retard" value={data?.overdue_tasks} loading={isPending} tone="danger" />
          <StatCard label="Tickets ouverts" value={data?.open_tickets} loading={isPending} />
        </div>
      </div>
    </div>
  )
}
