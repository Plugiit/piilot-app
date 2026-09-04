import { createFileRoute, redirect } from '@tanstack/react-router'

import { homeFor, sessionQuery } from '@/lib/auth'

/**
 * Racine de l'application.
 *
 * Elle n'affiche rien : elle aiguille vers l'espace de l'utilisateur. Une
 * meme URL sert donc d'entree aux deux publics, ce que permet le choix d'une
 * application unique.
 */
export const Route = createFileRoute('/')({
  beforeLoad: async ({ context }) => {
    try {
      const user = await context.queryClient.query({ ...sessionQuery, staleTime: 'static' })
      throw redirect({ to: homeFor(user) })
    } catch (error) {
      // `redirect` leve pour interrompre le chargement : le relayer tel quel,
      // sinon l'aiguillage serait avale par ce catch et tout finirait sur la
      // page de connexion.
      if (isRedirect(error)) {
        throw error
      }

      throw redirect({ to: '/login', search: {} })
    }
  },
})

function isRedirect(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'to' in error
}
