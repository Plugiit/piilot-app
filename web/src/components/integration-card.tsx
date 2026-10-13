import type { ReactNode } from 'react'

/**
 * Carte d'un outil exterieur : son logo, son nom, ce qu'il apporte, et le
 * geste en pied. Partagee par les integrations du compte et les modules des
 * parametres, pour que les deux pages se lisent de la meme facon.
 */
export function IntegrationCard({
  logo,
  name,
  description,
  badge,
  action,
}: {
  logo: string
  name: string
  description: ReactNode
  /** Coin oppose au logo : l'etat d'un module. */
  badge?: ReactNode
  action: ReactNode
}) {
  return (
    <article className="flex flex-col gap-3 rounded-[12px] border border-[#e8e8e9] bg-white p-3">
      <div className="flex items-start justify-between gap-2">
        {/* La boite fait 48px, le logo s'y inscrit sans se deformer. Forcer les
            deux cotes de l'image ecrasait ceux qui ne sont pas carres — le
            Figma est deux tiers plus haut que large, Proxmox et Uptime Kuma ont
            des cadres plus larges que hauts. */}
        <span className="flex size-12 shrink-0 items-center justify-center">
          <img src={logo} alt="" className="size-full object-contain" />
        </span>
        {badge}
      </div>

      <div className="flex flex-1 flex-col gap-1">
        <h3 className="text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">{name}</h3>
        <p className="text-[12px] leading-[1.5] text-[#73757c]">{description}</p>
      </div>

      {action}
    </article>
  )
}

/** Le bouton de pied d'une carte, lien ou bouton. */
export const CARD_ACTION =
  'relative flex h-10 items-center justify-center gap-2 rounded-[12px] border border-[#e8e8e9] bg-white text-[14px] font-medium text-[#1b1b1b] shadow-[inset_0px_-2px_0px_0px_rgba(0,0,0,0.05)] transition-colors hover:bg-[#f8f8f8]'
