import { createFileRoute, redirect } from '@tanstack/react-router'

/**
 * L'adresse nue des parametres mene au premier referentiel.
 *
 * Le module n'a pas d'accueil a lui : ses pages sont des referentiels qu'on
 * vient tenir a jour, et une page de garde qui ne ferait que relister le menu
 * du panneau serait un ecran de plus a traverser.
 *
 * `replace` : l'etape intermediaire ne doit pas s'inscrire dans l'historique,
 * sinon le bouton Retour y repasserait pour rebondir aussitot.
 */
export const Route = createFileRoute('/_app/parametres/')({
  beforeLoad: () => {
    throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
})
