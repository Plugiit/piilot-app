import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, postFile, unwrap } from '@/lib/api'
import { spaceOfPath, spacesEnabled } from '@/lib/spaces'
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
 * Vrai quand le compte detient la permission.
 *
 * Sert a ne pas montrer ce qui serait refuse : le menu, les onglets, les
 * colonnes. Ce n'est jamais une protection — l'API refuse de toute facon, sur
 * chaque endpoint.
 */
export function can(user: Pick<User, 'permissions'> | undefined, permission: string): boolean {
  return user?.permissions.includes(permission) === true
}

/**
 * Espace d'accueil d'un utilisateur.
 *
 * Le back-office occupe la racine : en production il vit sur son propre
 * sous-domaine, ou `/admin` serait un doublon du domaine lui-meme. Le portail
 * garde son segment en attendant d'etre ecrit et de rejoindre le sien.
 *
 * Un interne atterrit dans les projets : sur le tableau de bord de l'agence
 * s'il le lit, sur « Mon travail » sinon — un chef de projet commence sa
 * journee par ce qui l'attend, pas par les chiffres de l'agence.
 *
 * C'est cette fonction qui decide ou atterrit une connexion, et elle est la
 * seule source de cette regle — la connexion et les gardes s'y referent toutes.
 */
export function homeFor(user: User): '/pm' | '/pm/mon-travail' | '/client' {
  if (!isInternal(user)) return '/client'

  // Quand l'administration a son domaine, seul un administrateur y entre : un
  // membre de l'equipe qui lirait le tableau de bord commence quand meme par
  // son travail.
  if (spacesEnabled() && user.role !== 'admin') return '/pm/mon-travail'

  return can(user, 'dashboard.read') ? '/pm' : '/pm/mon-travail'
}

/**
 * Vrai quand une adresse est hors de portee du compte parce qu'elle releve du
 * domaine d'administration, reserve aux administrateurs. Sert a ne pas
 * proposer dans le menu ce qui renverrait aussitot ailleurs.
 */
export function outOfReach(user: Pick<User, 'role'> | undefined, to: string): boolean {
  return spacesEnabled() && user?.role !== 'admin' && spaceOfPath(to) === 'admin'
}

/**
 * Mise a jour du compte connecte.
 *
 * Le cache de session est reecrit avec la reponse plutot qu'invalide : le
 * serveur rend le profil complet, le relire aussitot ferait un aller-retour
 * pour rien — et l'avatar de la barre laterale clignoterait entre-temps.
 */
export function useUpdateProfile() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (body: {
      firstname?: string
      lastname?: string
      email?: string
    }) => unwrap(await api.PATCH('/api/v1/auth/me', { body })).user,
    onSuccess: (user) => queryClient.setQueryData(sessionQuery.queryKey, user),
  })
}

/**
 * Changement de mot de passe.
 *
 * Le serveur revoque toutes les sessions, celle-ci comprise : il n'y a plus de
 * session a rafraichir apres coup, l'appelant doit renvoyer vers la connexion.
 */
export function useChangePassword() {
  return useMutation({
    mutationFn: async (body: { current_password: string; new_password: string }) =>
      unwrap(await api.POST('/api/v1/auth/me/password', { body })),
  })
}

/** Depot de la photo de profil. */
export function useUploadAvatar() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (file: File) => {
      const response = await postFile<{ user: User }>(
        '/api/v1/auth/me/avatar',
        'file',
        file,
      )

      return response.user
    },
    onSuccess: (user) => queryClient.setQueryData(sessionQuery.queryKey, user),
  })
}

/** Retrait de la photo de profil. */
export function useRemoveAvatar() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () => unwrap(await api.DELETE('/api/v1/auth/me/avatar')).user,
    onSuccess: (user) => queryClient.setQueryData(sessionQuery.queryKey, user),
  })
}
