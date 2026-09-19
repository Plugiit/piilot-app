import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { FolderKanban, LifeBuoy } from 'lucide-react'

import { AppShell, type NavItem } from '@/components/layout/app-shell'
import { homeFor, isInternal, sessionQuery } from '@/lib/auth'

const NAV: NavItem[] = [
  { to: '/client', label: 'Mes projets', icon: FolderKanban },
  { to: '/client/tickets', label: 'Support', icon: LifeBuoy },
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
    <AppShell nav={NAV} user={user} area="Espace client">
      <Outlet />
    </AppShell>
  )
}
