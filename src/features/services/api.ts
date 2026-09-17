import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'

export const serviceKeys = {
  all: ['services'] as const,
  list: (search: string, page: number) => [...serviceKeys.all, 'list', search, page] as const,
}

/** Vingt-cinq lignes par page, comme la valeur par defaut du serveur. */
export const SERVICES_PAGE_SIZE = 25

/**
 * Le referentiel, par ordre alphabetique.
 *
 * `keepPreviousData` garde la page precedente pendant le chargement de la
 * suivante : le tableau ne clignote pas et ne saute pas en hauteur.
 */
export function serviceListQuery(search: string, page: number) {
  return queryOptions({
    queryKey: serviceKeys.list(search, page),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/services', {
          params: {
            query: {
              search: search === '' ? undefined : search,
              page,
              page_size: SERVICES_PAGE_SIZE,
            },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/**
 * Le referentiel entier, pour les menus deroulants.
 *
 * Borne haut plutot que pagine : un selecteur qui n'offrirait que les
 * vingt-cinq premiers services laisserait les suivants inatteignables. Un
 * referentiel de prestations ne depasse pas cet ordre de grandeur.
 */
export function serviceOptionsQuery() {
  return queryOptions({
    queryKey: [...serviceKeys.all, 'options'] as const,
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/services', { params: { query: { page: 1, page_size: 100 } } }),
      ),
    staleTime: 5 * 60 * 1000,
  })
}

/** Ce que le formulaire envoie, a la creation comme a la modification. */
export interface ServiceValues {
  name: string
  description: string
  color: string
}

export function useCreateService() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: ServiceValues) =>
      unwrap(await api.POST('/api/v1/admin/services', { body: values })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: serviceKeys.all })
    },
  })
}

export function useUpdateService(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: ServiceValues) =>
      unwrap(
        await api.PATCH('/api/v1/admin/services/{id}', {
          params: { path: { id } },
          body: values,
        }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: serviceKeys.all })
    },
  })
}

export function useDeleteService(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () =>
      unwrap(await api.DELETE('/api/v1/admin/services/{id}', { params: { path: { id } } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: serviceKeys.all })
    },
  })
}
