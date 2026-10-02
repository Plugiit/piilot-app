import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { RoleMatrix } from '@/types/api'

export const accountKeys = {
  all: ['accounts'] as const,
  lists: () => [...accountKeys.all, 'list'] as const,
  list: (params: AccountListParams) => [...accountKeys.lists(), params] as const,
  invitations: () => [...accountKeys.all, 'invitations'] as const,
  roles: () => [...accountKeys.all, 'roles'] as const,
}

/** Barre d'outils de l'onglet « Comptes », telle que l'adresse la porte. */
export interface AccountListParams {
  page: number
  search?: string
  role?: 'admin' | 'team' | 'client'
  status?: 'active' | 'disabled'
}

export const ACCOUNT_PAGE_SIZE = 25

export function accountListQuery(params: AccountListParams) {
  return queryOptions({
    queryKey: accountKeys.list(params),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/accounts', {
          params: {
            query: {
              page: params.page,
              page_size: ACCOUNT_PAGE_SIZE,
              search: params.search,
              role: params.role,
              status: params.status,
            },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

export const invitationListQuery = queryOptions({
  queryKey: accountKeys.invitations(),
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/accounts/invitations')),
})

export const roleMatrixQuery = queryOptions({
  queryKey: accountKeys.roles(),
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/roles')),
})

/**
 * Rafraichit tout l'ecran apres une ecriture : une invitation acceptee devient
 * un compte, un role change modifie les compteurs de la matrice.
 */
function useInvalidateAccounts() {
  const queryClient = useQueryClient()
  return () => void queryClient.invalidateQueries({ queryKey: accountKeys.all })
}

export interface InviteValues {
  email: string
  firstname?: string
  lastname?: string
  role: 'admin' | 'team' | 'client'
  client_id?: string | null
}

export function useInvite() {
  const invalidate = useInvalidateAccounts()
  return useMutation({
    mutationFn: async (values: InviteValues) =>
      unwrap(await api.POST('/api/v1/admin/accounts/invitations', { body: values })),
    onSuccess: invalidate,
  })
}

export function useResendInvitation() {
  const invalidate = useInvalidateAccounts()
  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(
        await api.POST('/api/v1/admin/accounts/invitations/{id}/resend', {
          params: { path: { id } },
        }),
      ),
    onSuccess: invalidate,
  })
}

export function useRevokeInvitation() {
  const invalidate = useInvalidateAccounts()
  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(
        await api.DELETE('/api/v1/admin/accounts/invitations/{id}', { params: { path: { id } } }),
      ),
    onSuccess: invalidate,
  })
}

export function useSetRole() {
  const invalidate = useInvalidateAccounts()
  return useMutation({
    mutationFn: async ({ id, role }: { id: string; role: 'admin' | 'team' }) =>
      unwrap(
        await api.PATCH('/api/v1/admin/accounts/{id}', {
          params: { path: { id } },
          body: { role },
        }),
      ),
    onSuccess: invalidate,
  })
}

export function useSetAccountEnabled() {
  const invalidate = useInvalidateAccounts()
  return useMutation({
    mutationFn: async ({ id, enabled }: { id: string; enabled: boolean }) =>
      enabled
        ? unwrap(
            await api.POST('/api/v1/admin/accounts/{id}/enable', { params: { path: { id } } }),
          )
        : unwrap(
            await api.POST('/api/v1/admin/accounts/{id}/disable', { params: { path: { id } } }),
          ),
    onSuccess: invalidate,
  })
}

export function usePasswordResetLink() {
  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(
        await api.POST('/api/v1/admin/accounts/{id}/password-reset', { params: { path: { id } } }),
      ),
  })
}

export function useSetRolePermissions() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ role, permissions }: { role: string; permissions: string[] }) =>
      unwrap(
        await api.PUT('/api/v1/admin/roles/{code}/permissions', {
          params: { path: { code: role } },
          body: { permissions },
        }),
      ),
    onSuccess: (matrix: RoleMatrix) => {
      queryClient.setQueryData(accountKeys.roles(), matrix)
      // Les permissions de l'appelant ont pu changer : la session les porte.
      void queryClient.invalidateQueries({ queryKey: ['session'] })
    },
  })
}
