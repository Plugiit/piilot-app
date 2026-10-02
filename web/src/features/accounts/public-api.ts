import { queryOptions } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'

/**
 * Appels des pages publiques : invitation, mot de passe oublie,
 * reinitialisation. Aucun n'exige de session.
 *
 * `retry: false` sur les lectures : un lien expire ou deja utilise ne le sera
 * pas moins a la troisieme tentative, et l'ecran doit l'expliquer tout de
 * suite.
 */

export function invitationQuery(token: string) {
  return queryOptions({
    queryKey: ['public', 'invitation', token] as const,
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/auth/invitations/{token}', { params: { path: { token } } })),
    retry: false,
    staleTime: Infinity,
  })
}

export async function acceptInvitation(
  token: string,
  values: { firstname: string; lastname: string; password: string },
) {
  return unwrap(
    await api.POST('/api/v1/auth/invitations/{token}/accept', {
      params: { path: { token } },
      body: values,
    }),
  ).user
}

export const forgotConfigQuery = queryOptions({
  queryKey: ['public', 'forgot-config'] as const,
  queryFn: async () => unwrap(await api.GET('/api/v1/auth/password/forgot')),
  staleTime: Infinity,
})

export async function requestPasswordReset(email: string) {
  unwrap(await api.POST('/api/v1/auth/password/forgot', { body: { email } }))
}

export function passwordResetQuery(token: string) {
  return queryOptions({
    queryKey: ['public', 'password-reset', token] as const,
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/auth/password/reset/{token}', { params: { path: { token } } })),
    retry: false,
    staleTime: Infinity,
  })
}

export async function resetPassword(token: string, password: string) {
  return unwrap(
    await api.POST('/api/v1/auth/password/reset/{token}', {
      params: { path: { token } },
      body: { password },
    }),
  )
}
