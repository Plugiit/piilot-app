import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'

import { AdminShell } from '@/components/layout/admin-shell'
import { homeFor, isInternal, sessionQuery } from '@/lib/auth'

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

    return { user }
  },
  component: AppLayout,
})

function AppLayout() {
  const { user } = Route.useRouteContext()

  return (
    <AdminShell user={user} title="Projets">
      <Outlet />
    </AdminShell>
  )
}
