import { Notification01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { createFileRoute } from '@tanstack/react-router'

import { Card } from '@/components/settings-ui'

export const Route = createFileRoute('/_app/compte/notifications')({
  component: NotificationsPage,
})

/**
 * Preferences de notification.
 *
 * L'ecran existe, les reglages non : rien n'envoie encore de message dans
 * l'application — ni courriel, ni alerte. Poser ici des interrupteurs qui ne
 * commandent rien donnerait le sentiment d'avoir choisi quelque chose, et il
 * faudrait ensuite deviner lesquels ont vraiment ete voulus le jour ou les
 * envois arrivent.
 *
 * La page dit donc ou en est le sujet. Les reglages viendront avec ce qu'ils
 * commandent.
 */
function NotificationsPage() {
  return (
    <Card title="Notifications">
      <div className="flex flex-col items-center gap-3 px-4 py-10 text-center">
        <span className="flex size-11 items-center justify-center rounded-full bg-[#f3f4f4] text-[#73757c]">
          <HugeiconsIcon icon={Notification01Icon} size={22} strokeWidth={1.6} />
        </span>

        <div className="flex flex-col gap-1">
          <p className="text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">
            Aucune notification pour l’instant
          </p>
          <p className="max-w-[420px] text-[14px] leading-[1.5] text-[#73757c]">
            L’application n’envoie encore ni courriel ni alerte. Les réglages
            apparaîtront ici quand il y aura quelque chose à régler.
          </p>
        </div>
      </div>
    </Card>
  )
}
