import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'

import { CommandPalette } from '@/components/command-palette'
import { AdminShell } from '@/components/layout/admin-shell'
import { homeFor, isInternal, sessionQuery } from '@/lib/auth'
import { backOfficeOf, currentSpace, spaceOfPath, spacesEnabled, spaceUrl } from '@/lib/spaces'

/**
 * Back-office de l'agence.
 *
 * Route sans segment d'URL : elle porte la garde et le chassis, mais n'ajoute
 * rien aux adresses. Ses enfants s'ecrivent donc /pm et /crm plutot que
 * /admin/pm — le prefixe n'apprend rien a personne, la sidebar dit deja ou
 * l'on est.
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
export const Route = createFileRoute('/_app')({
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

    if (spacesEnabled()) {
      // Chaque role a le domaine de son back-office : un administrateur fait
      // tout son travail sur celui de l'administration, l'equipe sur le sien.
      // Arrive sur l'autre, on repart sur le bon, a la meme adresse.
      const home = backOfficeOf(user.role)
      const current = currentSpace()
      const adminOnly = spaceOfPath(location.pathname) === 'admin'

      if (current !== null && current !== home) {
        const path = home === 'team' && adminOnly ? homeFor(user) : `${location.pathname}${location.searchStr}`
        throw redirect({ href: `${spaceUrl(home)}${path}` })
      }

      // Les ecrans d'administration n'existent pas pour l'equipe.
      if (adminOnly && user.role !== 'admin') {
        throw redirect({ to: homeFor(user) })
      }
    }

    return { user }
  },
  component: AppLayout,
})

function AppLayout() {
  const { user } = Route.useRouteContext()

  return (
    <AdminShell user={user} title="Projets">
      <Outlet />
      <CommandPalette />
    </AdminShell>
  )
}
