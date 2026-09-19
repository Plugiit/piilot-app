import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { TicketPriority, TicketStatus, TicketTracker } from '@/types/api'

/**
 * Ce que la barre d'outils peut demander.
 *
 * Les trois vues la partagent : changer d'onglet ne remet pas les filtres a
 * zero, on regarde le meme jeu de tickets autrement.
 */
export interface TicketFilters {
  /** Porte sur le sujet et sur le numero : « 47 » retrouve le ticket #47. */
  search?: string
  status?: TicketStatus
  tracker?: TicketTracker
  priority?: TicketPriority
  projectId?: string
}

/** Cles de cache des tickets. */
export const ticketKeys = {
  all: ['tickets'] as const,
  mine: (f: TicketFilters, page: number) => [...ticketKeys.all, 'mine', f, page] as const,
  board: (f: TicketFilters) => [...ticketKeys.all, 'board', f] as const,
  project: (projectId: string, page: number) =>
    [...ticketKeys.all, 'project', projectId, page] as const,
  projectBoard: (projectId: string) =>
    [...ticketKeys.all, 'project', projectId, 'board'] as const,
}

/** Les filtres voyagent sous les noms que l'API attend. */
function queryOf(f: TicketFilters) {
  return {
    search: f.search,
    status: f.status,
    tracker: f.tracker,
    priority: f.priority,
    project_id: f.projectId,
  }
}

/** Vingt-cinq lignes par page, comme la valeur par defaut du serveur. */
export const TICKETS_PAGE_SIZE = 25

/**
 * Mes tickets.
 *
 * Aucun `assignee_id` n'est envoye : le serveur le lit dans la session. Le
 * passer en parametre laisserait demander les tickets de quelqu'un d'autre en
 * changeant l'adresse.
 *
 * `keepPreviousData` garde la page precedente pendant le chargement de la
 * suivante : le tableau ne clignote pas et ne saute pas en hauteur.
 */
export function myTicketsQuery(f: TicketFilters, page: number) {
  return queryOptions({
    queryKey: ticketKeys.mine(f, page),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/tickets/mine', {
          params: { query: { ...queryOf(f), page, page_size: TICKETS_PAGE_SIZE } },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/** Ce que le formulaire de depot envoie. */
export interface CreateTicketValues {
  project_id: string
  subject: string
  description?: string
  tracker: TicketTracker
  priority: TicketPriority
  /** Nul pour laisser le ticket a prendre. */
  assignee_id?: string | null
}

/**
 * Depot d'un ticket.
 *
 * Invalide toute la racine et non la seule page courante : un ticket depose
 * remonte en tete de liste, donc sur la page 1, quelle que soit la page d'ou
 * on l'a cree.
 */
export function useCreateTicket() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: CreateTicketValues) =>
      unwrap(await api.POST('/api/v1/admin/tickets', { body: values })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ticketKeys.all })
    },
  })
}

/**
 * Mes tickets, en kanban.
 *
 * Une seule requete sert les deux vues : regrouper par projet ou par statut est
 * une facon de lire, pas un jeu de donnees different. Bornee et non paginee,
 * comme les kanbans des taches et des clients.
 */
export function myTicketsBoardQuery(f: TicketFilters) {
  return queryOptions({
    queryKey: ticketKeys.board(f),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/tickets/mine/board', { params: { query: queryOf(f) } })),
    placeholderData: keepPreviousData,
  })
}

/**
 * Les tickets d'un projet, pour l'onglet de sa fiche.
 *
 * Tous les tickets du projet et non les seuls miens : devant un projet, la
 * question est « qui a quoi », pas « qu'ai-je a faire ». D'ou un endpoint a
 * part plutot que « /mine » filtre par projet.
 *
 * Pas de filtres : l'onglet n'a pas de barre d'outils. L'ecran « Tickets » du
 * module la porte deja, et c'est la qu'on va pour chercher.
 */
export function projectTicketsQuery(projectId: string, page: number) {
  return queryOptions({
    queryKey: ticketKeys.project(projectId, page),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/projects/{id}/tickets', {
          params: { path: { id: projectId }, query: { page, page_size: TICKETS_PAGE_SIZE } },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/** Les memes tickets en kanban, que la vue repartit par statut. */
export function projectTicketsBoardQuery(projectId: string) {
  return queryOptions({
    queryKey: ticketKeys.projectBoard(projectId),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/projects/{id}/tickets/board', {
          params: { path: { id: projectId } },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/** Fiche d'un ticket et son registre, en un appel. */
export function ticketDetailQuery(id: string) {
  return queryOptions({
    queryKey: [...ticketKeys.all, 'detail', id] as const,
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/tickets/{id}', { params: { path: { id } } })),
  })
}

/** Ce que le redacteur envoie. */
export interface PostMessageValues {
  body: string
  is_internal: boolean
  /** Nuls pour ne rien changer. */
  status?: TicketStatus | null
  priority?: TicketPriority | null
  /**
   * Vrai pour appliquer `assignee_id`. Nul avec ce drapeau remet a prendre.
   *
   * Requis et non optionnel : le contrat porte un defaut, que le generateur
   * rend donc obligatoire. L'oublier ici ferait passer `undefined` pour un
   * `false` implicite — vrai en pratique, mais que le type refuse.
   */
  change_assignee: boolean
  assignee_id?: string | null
}

/**
 * Inscrit une entree au registre.
 *
 * La reponse porte la fiche entiere : elle remplace le cache d'un bloc plutot
 * que d'ajouter la ligne a la main puis de se resynchroniser. Les listes et les
 * kanbans sont invalides en plus, puisque le statut a pu bouger.
 */
export function usePostTicketMessage(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: PostMessageValues) =>
      unwrap(
        await api.POST('/api/v1/admin/tickets/{id}/messages', {
          params: { path: { id } },
          body: values,
        }),
      ),
    onSuccess: (detail) => {
      queryClient.setQueryData([...ticketKeys.all, 'detail', id], detail)
      void queryClient.invalidateQueries({ queryKey: [...ticketKeys.all, 'mine'] })
      void queryClient.invalidateQueries({ queryKey: [...ticketKeys.all, 'board'] })
    },
  })
}

/**
 * Renomme un ticket.
 *
 * Meme reponse que l'inscription au registre — la fiche entiere — pour que le
 * cache se remplace d'un bloc. Les listes et les kanbans suivent : le sujet y
 * est la colonne qu'on lit.
 */
export function useRenameTicket(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: { subject: string }) =>
      unwrap(
        await api.PATCH('/api/v1/admin/tickets/{id}', {
          params: { path: { id } },
          body: values,
        }),
      ),
    onSuccess: (detail) => {
      queryClient.setQueryData([...ticketKeys.all, 'detail', id], detail)
      void queryClient.invalidateQueries({ queryKey: [...ticketKeys.all, 'mine'] })
      void queryClient.invalidateQueries({ queryKey: [...ticketKeys.all, 'board'] })
    },
  })
}
