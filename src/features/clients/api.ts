import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { ClientStatus, CrmClient } from '@/types/api'

/** Cles de cache du module CRM. */
export const clientKeys = {
  all: ['crm', 'clients'] as const,
  lists: () => [...clientKeys.all, 'list'] as const,
  list: (params: CrmClientParams) => [...clientKeys.lists(), params] as const,
}

/** Colonnes triables du tableau, closes comme cote serveur. */
export type CrmClientSort = 'name' | 'projects' | 'created'

/** Ce que la barre d'outils de l'ecran « Clients » sait demander. */
export interface CrmClientParams {
  search?: string
  /** Etape du pipeline. Non pose, la liste porte toutes les etapes. */
  status?: ClientStatus
  /** Identifiant du membre de l'agence qui suit le client. */
  managerId?: string
  /** Non pose, le filtre n'existe pas : la liste porte alors tous les clients. */
  hasPortal?: boolean
  sort?: CrmClientSort
  dir?: 'asc' | 'desc'
  page?: number
}

/** Vingt-cinq lignes par page, comme la valeur par defaut du serveur. */
export const CLIENTS_PAGE_SIZE = 25

/**
 * Tableau des clients.
 *
 * `keepPreviousData` garde l'affichage precedent pendant qu'un filtre change :
 * sans lui, le tableau se viderait entre deux frappes de la recherche.
 */
export function clientListQuery(params: CrmClientParams) {
  return queryOptions({
    queryKey: clientKeys.list(params),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/crm/clients', {
          params: {
            query: {
              search: params.search,
              status: params.status,
              manager_id: params.managerId,
              has_portal: params.hasPortal,
              sort: params.sort,
              dir: params.dir,
              page: params.page,
              page_size: CLIENTS_PAGE_SIZE,
            },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/**
 * Ce que le formulaire de creation envoie.
 *
 * `contact_id` designe un contact libre a adopter, ou vaut null quand le
 * client naît sans interlocuteur.
 */
export interface CreateClientValues {
  name: string
  contact_id?: string | null
}

/**
 * Inscription d'un client.
 *
 * Invalide toutes les listes et non la seule page courante : un client cree
 * peut atterrir sur n'importe quelle page selon le tri et les filtres poses.
 */
export function useCreateClient() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: CreateClientValues) =>
      unwrap(await api.POST('/api/v1/admin/crm/clients', { body: values })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: clientKeys.lists() })
      // Le contact adopte n'est plus libre : les menus qui le proposaient
      // doivent cesser de le faire.
      void queryClient.invalidateQueries({ queryKey: ['crm', 'contacts'] })
    },
  })
}

/** Fiche d'un client : tout ce que l'ecran affiche, en un appel. */
export function clientDetailQuery(id: string) {
  return queryOptions({
    queryKey: [...clientKeys.all, 'detail', id] as const,
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/crm/clients/{id}', { params: { path: { id } } })),
  })
}

/**
 * Ce que la modification envoie.
 *
 * Tous les champs voyagent ensemble, comme cote serveur : n'en envoyer qu'une
 * partie effacerait le reste. Un appelant qui ne change qu'une valeur part donc
 * des valeurs actuelles du client.
 */
export interface UpdateClientValues {
  name: string
  status: ClientStatus
  account_manager_id?: string | null
  website?: string
  phone?: string
  address?: string
  postal_code?: string
  city?: string
  country?: string
  siret?: string
  vat_number?: string
}

/** Construit le corps de modification a partir d'un client, champs inchanges. */
export function valuesOfClient(client: CrmClient): UpdateClientValues {
  return {
    name: client.name,
    status: client.status,
    account_manager_id: client.account_manager?.id ?? null,
    website: client.website,
    phone: client.phone,
    address: client.address,
    postal_code: client.postal_code,
    city: client.city,
    country: client.country,
    siret: client.siret,
    vat_number: client.vat_number,
  }
}

/** Modification de la fiche d'un client. */
export function useUpdateClient() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, values }: { id: string; values: UpdateClientValues }) =>
      unwrap(
        await api.PATCH('/api/v1/admin/crm/clients/{id}', {
          params: { path: { id } },
          body: values,
        }),
      ),
    onSuccess: (_data, { id }) => {
      void queryClient.invalidateQueries({ queryKey: clientKeys.lists() })
      void queryClient.invalidateQueries({ queryKey: [...clientKeys.all, 'detail', id] })
      // Le nom du client s'affiche aussi dans le tableau des contacts.
      void queryClient.invalidateQueries({ queryKey: ['crm', 'contacts'] })
    },
  })
}

/** Suppression d'un client. Refusee tant qu'il porte des projets ou des comptes. */
export function useDeleteClient() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.DELETE('/api/v1/admin/crm/clients/{id}', { params: { path: { id } } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: clientKeys.lists() })
      // Ses contacts partent avec lui.
      void queryClient.invalidateQueries({ queryKey: ['crm', 'contacts'] })
    },
  })
}

/**
 * Kanban commercial.
 *
 * Borne et non pagine, comme le tableau des taches : un kanban se lit en entier
 * ou pas du tout. `truncated` dit quand la borne a coupe.
 */
export function clientBoardQuery(params: Pick<CrmClientParams, 'search' | 'managerId'>) {
  return queryOptions({
    queryKey: [...clientKeys.all, 'board', params] as const,
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/crm/clients/board', {
          params: { query: { search: params.search, manager_id: params.managerId } },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/** Deplacement d'une carte dans le pipeline. */
export function useMoveClientStatus() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: ClientStatus }) =>
      unwrap(
        await api.PUT('/api/v1/admin/crm/clients/{id}/status', {
          params: { path: { id } },
          body: { status },
        }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: clientKeys.all })
    },
  })
}
