import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { FolderKanban, LayoutDashboard } from 'lucide-react'

import { AppShell, type NavItem } from '@/components/layout/app-shell'
import { homeFor, isInternal, sessionQuery } from '@/lib/auth'

const NAV: NavItem[] = [
  { to: '/admin', label: 'Tableau de bord', icon: LayoutDashboard },
  { to: '/admin/projects', label: 'Projets', icon: FolderKanban },
]

/**
 * Back-office de l'agence.
 *
 * La garde vit dans `beforeLoad` : elle s'execute avant le rendu ET avant les
 * loaders enfants, donc une session absente redirige sans qu'aucune requete
 * metier ne parte. Un compte client authentifie est renvoye vers son portail
 * plutot que vers la connexion — il est identifie, simplement pas au bon
 * endroit.
 *
 * Cette garde ne protege rien a elle seule : elle evite d'afficher des ecrans
 * inutiles. C'est l'API qui refuse les donnees, sur chaque endpoint.
 */
export const Route = createFileRoute('/admin')({
  beforeLoad: async ({ context, location }) => {
    let user

    try {
      user = await context.queryClient.query({ ...sessionQuery, staleTime: 'static' })
    } catch {
      throw redirect({ to: '/login', search: { redirect: location.href } })
    }

    if (!isInternal(user)) {
      throw redirect({ to: homeFor(user) })
    }

    return { user }
  },
  component: AdminLayout,
})

function AdminLayout() {
  const { user } = Route.useRouteContext()

  return (
    <AppShell nav={NAV} user={user} area="Agence">
      <Outlet />
    </AppShell>
  )
}
