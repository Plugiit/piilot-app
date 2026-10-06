import { useEffect } from 'react'

import { useMaintenance } from '@/lib/maintenance'

/**
 * Ecran « mise a jour en cours », par-dessus la page.
 *
 * N'apparait que si le serveur est absent plus longtemps que la passerelle ne
 * sait faire attendre une requete : une mise a jour ordinaire bascule sans
 * coupure, et personne ne le voit. Il interroge la passerelle toutes les deux
 * secondes et recharge la page des qu'une instance peut servir.
 */
export function MaintenanceScreen() {
  const maintenance = useMaintenance()

  useEffect(() => {
    if (!maintenance) return

    let timer: number
    const poll = async () => {
      try {
        const res = await fetch('/health/gateway', { cache: 'no-store' })
        const status = (await res.json()) as { ready?: boolean }
        if (status.ready === true) {
          window.location.reload()
          return
        }
      } catch {
        // Passerelle injoignable elle aussi : on retente.
      }
      timer = window.setTimeout(() => void poll(), 2000)
    }
    timer = window.setTimeout(() => void poll(), 2000)

    return () => window.clearTimeout(timer)
  }, [maintenance])

  if (!maintenance) return null

  return (
    <div className="fixed inset-0 z-[100] grid place-items-center bg-[#f5f5f5] p-4">
      <div role="status" className="w-full max-w-[380px] rounded-[12px] border border-[#e4e4e4] bg-white p-6">
        <p className="flex items-center gap-2 text-[16px] font-medium text-[#1b1b1b]">
          <span aria-hidden className="size-1.5 animate-pulse rounded-full bg-brand motion-reduce:animate-none" />
          Mise à jour en cours
        </p>
        <p className="mt-1 text-[13px] text-[#73757c]">
          Piilot revient dans un instant. La page se rechargera d’elle-même.
        </p>
      </div>
    </div>
  )
}
