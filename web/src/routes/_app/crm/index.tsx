import { createFileRoute, redirect } from '@tanstack/react-router'

/**
 * Racine du module CRM.
 *
 * Elle n'a pas d'ecran propre : a trois entrees, un tableau de bord n'aurait
 * rien a montrer que la liste des clients ne montre deja. La redirection garde
 * une URL de module utilisable — /crm mene quelque part.
 */
export const Route = createFileRoute('/_app/crm/')({
  beforeLoad: () => {
    throw redirect({ to: '/crm/clients' })
  },
})
