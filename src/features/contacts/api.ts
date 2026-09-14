import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { clientKeys } from '@/features/clients/api'
import { api, unwrap } from '@/lib/api'

/** Cles de cache des contacts. */
export const contactKeys = {
  all: ['crm', 'contacts'] as const,
  lists: () => [...contactKeys.all, 'list'] as const,
  list: (params: CrmContactParams) => [...contactKeys.all, 'list', params] as const,
  /** Contacts d'un client, tels que le menu deroulant les propose. */
  ofClient: (clientId: string, search: string) =>
    [...contactKeys.all, 'of-client', clientId, search] as const,
  /** Contacts sans entreprise, qu'un nouveau client peut adopter. */
  free: (search: string) => [...contactKeys.all, 'free', search] as const,
}

/** Colonnes triables du tableau, closes comme cote serveur. */
export type CrmContactSort = 'name' | 'client'

/** Ce que la barre d'outils de l'ecran « Contacts » sait demander. */
export interface CrmContactParams {
  search?: string
  clientId?: string
  /** Ne garde que les contacts sans entreprise. */
  onlyFree?: boolean
  sort?: CrmContactSort
  dir?: 'asc' | 'desc'
  page?: number
}

export const CONTACTS_PAGE_SIZE = 25

/** Tableau des contacts. */
export function contactListQuery(params: CrmContactParams) {
  return queryOptions({
    queryKey: contactKeys.list(params),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/crm/contacts', {
          params: {
            query: {
              search: params.search,
              client_id: params.clientId,
              only_free: params.onlyFree,
              sort: params.sort,
              dir: params.dir,
              page: params.page,
              page_size: CONTACTS_PAGE_SIZE,
            },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/**
 * Contacts d'un client, pour le menu qui designe l'interlocuteur principal.
 *
 * La recherche part au serveur plutot que de filtrer une liste deja chargee :
 * un client peut compter plus de contacts qu'on n'en affiche.
 */
export function contactsOfClientQuery(clientId: string, search: string) {
  return queryOptions({
    queryKey: contactKeys.ofClient(clientId, search),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/crm/clients/{id}/contacts', {
          params: {
            path: { id: clientId },
            query: { search: search === '' ? undefined : search },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/**
 * Contacts sans entreprise, pour le menu du formulaire de creation d'un
 * client. Un contact deja rattache n'y figure pas : il appartient a quelqu'un.
 */
export function freeContactsQuery(search: string) {
  return queryOptions({
    queryKey: contactKeys.free(search),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/crm/contacts/free', {
          params: { query: { search: search === '' ? undefined : search } },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/** Ce que le formulaire de creation envoie. Un prenom ou un nom suffit. */
export interface CreateContactValues {
  /** Absent pour un contact libre, en attente d'une entreprise. */
  client_id?: string
  firstname?: string
  lastname?: string
  role?: string
  email?: string
  phone?: string
  primary?: boolean
}

/**
 * Inscription d'un contact.
 *
 * Invalide aussi les clients : le tout premier contact d'un client devient son
 * interlocuteur principal, et la colonne de l'autre tableau change avec lui.
 */
export function useCreateContact() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: CreateContactValues) =>
      unwrap(await api.POST('/api/v1/admin/crm/contacts', { body: values })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: contactKeys.lists() })
      void queryClient.invalidateQueries({ queryKey: clientKeys.lists() })
    },
  })
}

/** Designe l'interlocuteur principal d'un client. `null` retire la designation. */
export function useSetPrimaryContact() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ clientId, contactId }: { clientId: string; contactId: string | null }) =>
      unwrap(
        await api.PUT('/api/v1/admin/crm/clients/{id}/primary-contact', {
          params: { path: { id: clientId } },
          body: { contact_id: contactId },
        }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: clientKeys.lists() })
      void queryClient.invalidateQueries({ queryKey: contactKeys.lists() })
    },
  })
}

/** Ce que le formulaire de modification envoie. */
export interface UpdateContactValues {
  firstname: string
  lastname: string
  role: string
  email: string
  phone: string
}

/** Modification d'un contact. */
export function useUpdateContact() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, values }: { id: string; values: UpdateContactValues }) =>
      unwrap(
        await api.PATCH('/api/v1/admin/crm/contacts/{id}', {
          params: { path: { id } },
          body: values,
        }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: contactKeys.all })
      void queryClient.invalidateQueries({ queryKey: clientKeys.all })
    },
  })
}

/** Suppression d'un contact. La designation qui le vise est retiree au passage. */
export function useDeleteContact() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.DELETE('/api/v1/admin/crm/contacts/{id}', { params: { path: { id } } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: contactKeys.all })
      void queryClient.invalidateQueries({ queryKey: clientKeys.all })
    },
  })
}
