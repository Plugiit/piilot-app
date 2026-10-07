import { MailSend01Icon, ShieldUserIcon, UserMultipleIcon } from '@hugeicons/core-free-icons'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { useState } from 'react'

import { PageFrame } from '@/components/layout/page-frame'
import { TabBar, type Tab } from '@/components/layout/tab-bar'
import { invitationListQuery } from '@/features/accounts/api'
import { InviteDialog } from '@/features/accounts/invite-dialog'
import { AccountsToolbarSlot } from '@/features/accounts/toolbar'
import { can, sessionQuery } from '@/lib/auth'

/**
 * Ecran « Comptes et rôles » : qui a acces a Piilot, et avec quels droits.
 *
 * Meme chassis que les taches : la barre d'outils d'abord — la recherche et
 * les filtres de l'onglet ouvert, puis « Inviter » au bout —, les onglets
 * dessous, et le contenu sur toute la largeur.
 *
 * La recherche et les filtres appartiennent a l'onglet « Comptes », mais la
 * barre est au chassis : l'onglet y projette les siens par un portail (voir
 * features/accounts/toolbar), les deux autres n'y mettent rien. « Inviter »
 * reste, lui, sur tous les onglets.
 */
export const Route = createFileRoute('/_app/parametres/comptes')({
  // Ecran d'administration : sans le droit, on retombe sur les referentiels.
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'roles.read')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  component: AccountsLayout,
})

function AccountsLayout() {
  const { data: session } = useQuery(sessionQuery)
  const canManage = session?.permissions.includes('users.write') === true
  const { data: invitations } = useQuery(invitationListQuery)
  const [slot, setSlot] = useState<HTMLDivElement | null>(null)

  const tabs: Tab[] = [
    { to: '/parametres/comptes', label: 'Comptes', icon: UserMultipleIcon },
    {
      to: '/parametres/comptes/invitations',
      label: 'Invitations',
      icon: MailSend01Icon,
      badge: invitations?.items.length,
    },
    { to: '/parametres/comptes/roles', label: 'Rôles', icon: ShieldUserIcon },
  ]

  return (
    <PageFrame title="Comptes et rôles">
      <div className="flex min-h-full flex-col">
        <div className="flex flex-wrap items-center gap-2 p-4">
          {/* Ce que l'onglet ouvert y met : recherche et filtres, ou rien. */}
          <div ref={setSlot} className="flex min-w-0 flex-1 flex-wrap items-center gap-2" />
          {canManage && <InviteDialog />}
        </div>

        <div className="flex shrink-0 items-center border-b border-[#e8e8e9] pl-4">
          <TabBar tabs={tabs} layoutId="accounts-tab" />
        </div>

        <AccountsToolbarSlot.Provider value={slot}>
          <Outlet />
        </AccountsToolbarSlot.Provider>
      </div>
    </PageFrame>
  )
}
