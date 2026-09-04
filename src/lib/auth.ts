import { queryOptions } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { User } from '@/types/api'

/**
 * Session courante.
 *
 * Exposee en queryOptions plutot qu'en hook : les loaders de route peuvent
 * l'attendre avant de rendre, et les composants la lisent via useQuery avec la
 * meme cle — une seule requete pour les deux usages.
 */
export const sessionQuery = queryOptions({
  queryKey: ['session'],
  // L'API enveloppe le profil dans `{ user }` : les jetons vivent dans les
  // cookies httpOnly et ne figurent jamais dans le corps, mais l'enveloppe
  // laisse la place a ce que la reponse s'enrichisse sans casser le contrat.
  queryFn: async () => unwrap(await api.GET('/api/v1/auth/me')).user,
  // La session change rarement et son absence est geree par la garde de route.
  staleTime: 5 * 60_000,
  retry: false,
})

export async function login(email: string, password: string): Promise<User> {
  return unwrap(await api.POST('/api/v1/auth/login', { body: { email, password } })).user
}

export async function logout(): Promise<void> {
  await api.POST('/api/v1/auth/logout')
}

/** Roles internes : acces au back-office. */
const INTERNAL_ROLES = new Set<User['role']>(['admin', 'team'])

export function isInternal(user: User): boolean {
  return INTERNAL_ROLES.has(user.role)
}

/**
 * Espace d'accueil d'un utilisateur.
 *
 * L'application sert les deux espaces : `/admin` pour l'agence, `/client`
 * pour le portail. C'est le role qui decide ou atterrit une connexion, et
 * cette fonction est la seule source de cette regle — la racine, la
 * connexion et les gardes s'y referent toutes.
 */
export function homeFor(user: User): '/admin' | '/client' {
  return isInternal(user) ? '/admin' : '/client'
}
