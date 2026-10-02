import { MailSend01Icon, ShieldUserIcon, UserMultipleIcon } from '@hugeicons/core-free-icons'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'

import { PageFrame } from '@/components/layout/page-frame'
import { TabBar, type Tab } from '@/components/layout/tab-bar'
import { invitationListQuery } from '@/features/accounts/api'
import { InviteDialog } from '@/features/accounts/invite-dialog'
import { can, sessionQuery } from '@/lib/auth'

/**
 * Ecran « Comptes et rôles » : qui a acces a Piilot, et avec quels droits.
 *
 * Trois onglets plutot que trois ecrans : on passe de l'un a l'autre dans une
 * meme tache — inviter quelqu'un, verifier que l'invitation est partie, regler
 * ce que son role permet.
 *
 * Le bouton « Inviter » vit dans le chassis : on invite depuis n'importe quel
 * onglet. Il n'apparait que pour qui porte `users.write` ; le serveur le
 * verifie de toute facon.
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
        {/* Centre et non aligne en bas : le bouton se tient a egale distance
            des deux filets qui encadrent la barre d'onglets. */}
        <div className="flex shrink-0 items-center justify-between gap-3 border-b border-[#e8e8e9] px-4">
          <TabBar tabs={tabs} layoutId="accounts-tab" />
          {canManage && <InviteDialog />}
        </div>

        <Outlet />
      </div>
    </PageFrame>
  )
}
