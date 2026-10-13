import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { uptimeSettingsQuery } from '@/features/uptime/api'
import { SettingsForm, WebhookCard } from '@/features/uptime/settings'

/** Onglet « Connexion » : l'instance Kuma, la cle, l'intervalle, le webhook. */
export const Route = createFileRoute('/_app/parametres/modules/uptime/')({
  component: ConnectionTab,
})

function ConnectionTab() {
  const { data } = useQuery(uptimeSettingsQuery)
  if (data === undefined) return null

  return (
    <div className="flex flex-col gap-3 p-4">
      <SettingsForm key={`${data.base_url}${data.poll_minutes}${data.enabled}`} settings={data} />
      <WebhookCard settings={data} />
    </div>
  )
}
