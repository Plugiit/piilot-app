import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { Button } from '@/components/ui/button'
import { liveVersionQuery } from '@/features/system/api'

/**
 * Bandeau « nouvelle version en ligne », pour tout le monde.
 *
 * Apres un deploiement, un onglet reste ouvert garde l'ancien front. Le
 * bandeau le dit et propose de recharger, sans forcer : un formulaire en cours
 * de saisie serait perdu. Un morceau du front devenu introuvable, lui,
 * recharge la page de lui-meme (voir main.tsx).
 *
 * Rien en developpement, ou le front et l'API ne portent pas de version de
 * release.
 */
export function VersionBanner() {
  const { data: server } = useQuery({ ...liveVersionQuery, enabled: !import.meta.env.DEV })
  const [dismissed, setDismissed] = useState<string | null>(null)

  const outdated =
    server !== undefined && server !== '' && server !== 'dev' && server !== __APP_VERSION__

  if (!outdated || dismissed === server) return null

  return (
    <div
      role="status"
      className="fixed bottom-4 left-1/2 z-50 flex -translate-x-1/2 items-center gap-3 rounded-[12px] border border-[#e8e8e9] bg-white py-2 pr-2 pl-4 text-[13px] text-[#1b1b1b] shadow-[0_12px_28px_-8px_rgb(16_24_40/0.28)]"
    >
      <p>
        Piilot <strong>{server}</strong> est en ligne.
      </p>
      <Button size="sm" onClick={() => window.location.reload()}>
        Recharger
      </Button>
      <Button size="sm" variant="ghost" onClick={() => setDismissed(server)}>
        Plus tard
      </Button>
    </div>
  )
}
