import { createFileRoute } from '@tanstack/react-router'

import { IncidentsCard } from '@/features/uptime/settings'

/** Onglet « Incidents » : les pannes, les plus recentes d'abord. */
export const Route = createFileRoute('/_app/parametres/modules/uptime/incidents')({
  component: () => (
    <div className="flex flex-col gap-3 p-4">
      <IncidentsCard />
    </div>
  ),
})
