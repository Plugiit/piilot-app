import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'

import { PortalShell, type PortalNavItem } from '@/components/layout/portal-shell'
import { homeFor, isInternal, sessionQuery } from '@/lib/auth'

const NAV: PortalNavItem[] = [
  { to: '/client', label: 'Mes projets' },
  { to: '/client/tickets', label: 'Support' },
]

/**
 * Portail client.
 *
 * Surface exposee a l'exterieur de l'agence : tout endpoint consomme ici doit
 * filtrer sur le client de l'appelant cote serveur. La garde ci-dessous ne
 * fait qu'eviter d'afficher des ecrans hors sujet.
 *
 * Un compte interne est renvoye vers le back-office : il n'a pas de client
 * rattache, le portail n'aurait rien a lui montrer.
 */
export const Route = createFileRoute('/client')({
  beforeLoad: async ({ context, location }) => {
    let user

    try {
      user = await context.queryClient.query({ ...sessionQuery, staleTime: 'static' })
    } catch {
      throw redirect({ to: '/login', search: { redirect: location.href } })
    }

    if (isInternal(user)) {
      throw redirect({ to: homeFor(user) })
    }

    return { user }
  },
  component: ClientLayout,
})

function ClientLayout() {
  const { user } = Route.useRouteContext()

  return (
    <PortalShell nav={NAV} user={user}>
      <Outlet />
    </PortalShell>
  )
}
