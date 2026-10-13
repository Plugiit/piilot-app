import { createFileRoute } from '@tanstack/react-router'

import { MonitorsCard } from '@/features/uptime/settings'

/** Onglet « Sites » : ce que Kuma surveille, et leur disponibilite. */
export const Route = createFileRoute('/_app/parametres/modules/uptime/sites')({
  component: () => (
    <div className="flex flex-col gap-3 p-4">
      <MonitorsCard />
    </div>
  ),
})
