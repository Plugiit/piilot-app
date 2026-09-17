import { createFileRoute, redirect } from '@tanstack/react-router'

/**
 * L'adresse nue d'un projet mene a ses taches.
 *
 * Les deux onglets ont chacun leur route depuis que la vue est passee dans
 * `?vue` : sans cette redirection, ouvrir un projet afficherait son en-tete
 * au-dessus d'un vide.
 *
 * `replace` : l'etape intermediaire ne doit pas s'inscrire dans l'historique,
 * sinon le bouton Retour y repasserait pour rebondir aussitot.
 */
export const Route = createFileRoute('/_app/pm/projets/$id/')({
  beforeLoad: ({ params }) => {
    throw redirect({ to: '/pm/projets/$id/taches', params, replace: true })
  },
})
