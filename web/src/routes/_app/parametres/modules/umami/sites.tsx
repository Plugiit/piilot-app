import { createFileRoute } from '@tanstack/react-router'

import { AnalyticsWebsitesCard } from '@/features/analytics/settings'

/** Onglet « Sites » : ce qu'Umami mesure, et a quels projets c'est lie. */
export const Route = createFileRoute('/_app/parametres/modules/umami/sites')({
  component: () => (
    <div className="flex flex-col gap-3 p-4">
      <AnalyticsWebsitesCard />
    </div>
  ),
})
