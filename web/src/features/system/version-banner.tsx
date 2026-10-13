import { useQuery } from '@tanstack/react-query'
import { useRouter } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { liveVersionQuery } from '@/features/system/api'

/** Delai entre l'annonce et le rechargement : le temps de lire une ligne. */
const RELOAD_DELAY = 2000

/** Un champ ou l'on ecrit : recharger maintenant perdrait la saisie. */
function typing(): boolean {
  const el = document.activeElement
  if (!(el instanceof HTMLElement)) return false
  return el.isContentEditable || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement ||
    (el instanceof HTMLInputElement && !['button', 'checkbox', 'radio', 'submit', 'reset', 'range', 'color', 'file'].includes(el.type))
}

/**
 * Rechargement automatique apres une mise a jour.
 *
 * Un onglet reste ouvert garde l'ancien front ; des que le serveur annonce une
 * autre version, l'onglet se recharge de lui-meme, au premier moment sur :
 *
 *  - onglet en arriere-plan : tout de suite, personne ne le regarde ;
 *  - au prochain changement de page : la page demandee arrive directement
 *    dans la nouvelle version ;
 *  - curseur dans un champ : on attend qu'il en sorte, pour ne pas perdre ce
 *    qui s'ecrit ;
 *  - sinon : une ligne l'annonce, et la page se recharge deux secondes apres.
 *
 * Un morceau du front devenu introuvable, lui, recharge la page de lui-meme
 * (voir main.tsx). Rien en developpement, ou le front et l'API ne portent pas
 * de version de release.
 */
export function VersionBanner() {
  const { data: server } = useQuery({ ...liveVersionQuery, enabled: !import.meta.env.DEV })
  const router = useRouter()
  const [reloading, setReloading] = useState(false)

  const outdated = server !== undefined && server !== '' && server !== 'dev' && server !== __APP_VERSION__

  useEffect(() => {
    if (!outdated) return

    let timer: number | undefined
    const reload = () => window.location.reload()

    // Un rechargement differe, annule si l'on se remet a ecrire entre-temps.
    const schedule = () => {
      window.clearTimeout(timer)
      if (document.hidden) return reload()
      if (typing()) return
      setReloading(true)
      timer = window.setTimeout(() => (typing() ? setReloading(false) : reload()), RELOAD_DELAY)
    }

    const onVisibility = () => document.hidden && reload()
    const onFocusOut = () => window.setTimeout(schedule, 0)
    const onFocusIn = () => {
      if (typing()) {
        window.clearTimeout(timer)
        setReloading(false)
      }
    }
    // Apres un changement de page, la nouvelle adresse se recharge entiere.
    const unsubscribe = router.subscribe('onResolved', reload)

    document.addEventListener('visibilitychange', onVisibility)
    document.addEventListener('focusout', onFocusOut)
    document.addEventListener('focusin', onFocusIn)
    schedule()

    return () => {
      window.clearTimeout(timer)
      unsubscribe()
      document.removeEventListener('visibilitychange', onVisibility)
      document.removeEventListener('focusout', onFocusOut)
      document.removeEventListener('focusin', onFocusIn)
    }
  }, [outdated, router])

  if (!outdated || !reloading) return null

  return (
    <div
      role="status"
      className="fixed bottom-4 left-1/2 z-50 flex -translate-x-1/2 items-center gap-2 rounded-[12px] border border-[#e8e8e9] bg-white px-4 py-2 text-[13px] text-[#1b1b1b] shadow-[0_12px_28px_-8px_rgb(16_24_40/0.28)]"
    >
      <span aria-hidden className="bg-brand size-1.5 animate-pulse rounded-full motion-reduce:animate-none" />
      Piilot <strong>{server}</strong> est installé : la page se recharge.
    </div>
  )
}
