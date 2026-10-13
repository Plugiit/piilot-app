import { Alert02Icon, ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'

/**
 * L'avertissement d'un projet dont le site est surveille sans statistiques :
 * le client ne voit que sa disponibilite. Le lien mene au choix du site
 * Umami, pour qui peut modifier le projet.
 */
export function MissingStatisticsNotice({ projectId, canEdit }: { projectId: string; canEdit: boolean }) {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-[10px] bg-[#fff4e5] px-3 py-2.5 text-[14px] text-[#8a4b00]">
      <HugeiconsIcon icon={Alert02Icon} size={18} strokeWidth={1.8} className="shrink-0" />
      <p className="min-w-0 flex-1">
        Le site de production est surveillé, mais aucun site Umami n’est lié : le client ne voit que sa disponibilité, pas son
        audience.
      </p>
      {canEdit && (
        <Link
          to="/pm/projets/$id/parametres/statistiques"
          params={{ id: projectId }}
          className="flex shrink-0 items-center gap-1 font-medium underline underline-offset-2"
        >
          Lier le site Umami
          <HugeiconsIcon icon={ArrowRight01Icon} size={14} strokeWidth={1.8} />
        </Link>
      )}
    </div>
  )
}
