import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { analyticsSettingsQuery } from '@/features/analytics/api'
import { AnalyticsSettingsForm } from '@/features/analytics/settings'

/** Onglet « Connexion » : l'instance, l'authentification, la version, la fraicheur. */
export const Route = createFileRoute('/_app/parametres/modules/umami/')({
  component: ConnectionTab,
})

function ConnectionTab() {
  const { data } = useQuery(analyticsSettingsQuery)
  if (data === undefined) return null

  return (
    <div className="flex flex-col gap-3 p-4">
      <AnalyticsSettingsForm key={`${data.base_url}${data.auth_mode}${data.username}${data.api_version}${data.version_pinned}${data.cache_minutes}${data.enabled}`} settings={data} />
    </div>
  )
}
