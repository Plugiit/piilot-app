import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, apiUrl, postFile, unwrap } from '@/lib/api'
import type { Attachment, ProjectDetail, ProjectPriority, ProjectStatus } from '@/types/api'

export interface ProjectListParams {
  page: number
  pageSize: number
  search?: string
  status?: ProjectStatus
  /** Client dont on ne veut que les projets. L'API l'accepte sous `client_id`. */
  clientId?: string
  sort?: 'due' | 'name' | 'progress' | 'budget'
  dir?: 'asc' | 'desc'
}

/**
 * Cles de cache du module.
 *
 * Regroupees ici plutot qu'ecrites a la main dans chaque hook : une mutation
 * doit pouvoir invalider « tout ce qui touche aux projets » sans deviner la
 * forme exacte des cles posees ailleurs.
 */
export const projectKeys = {
  all: ['projects'] as const,
  lists: () => [...projectKeys.all, 'list'] as const,
  list: (params: ProjectListParams) => [...projectKeys.lists(), params] as const,
  detail: (id: string) => [...projectKeys.all, 'detail', id] as const,
  favorites: () => [...projectKeys.all, 'favorites'] as const,
}

/**
 * Raccourcis de la barre laterale.
 *
 * Cache long : la barre est montee sur tous les ecrans du back-office, et une
 * etoile ne bouge que quand on la clique — auquel cas la mutation invalide
 * cette cle elle-meme, sans attendre la peremption.
 */
export const favoriteProjectsQuery = queryOptions({
  queryKey: projectKeys.favorites(),
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/projects/favorites')),
  staleTime: 5 * 60_000,
})

/**
 * Liste des projets, paginee cote serveur.
 *
 * `keepPreviousData` garde la page precedente affichee pendant le chargement
 * de la suivante : la table ne clignote pas et ne saute pas en hauteur au
 * changement de page.
 */
export function projectListQuery(params: ProjectListParams) {
  return queryOptions({
    queryKey: projectKeys.list(params),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/projects', {
          params: {
            query: {
              page: params.page,
              page_size: params.pageSize,
              search: params.search,
              status: params.status,
              client_id: params.clientId,
              sort: params.sort,
              dir: params.dir,
            },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/** En-tete d'un projet : le chassis le charge, ses deux onglets le partagent. */
export function projectDetailQuery(id: string) {
  return queryOptions({
    queryKey: projectKeys.detail(id),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/projects/{id}', { params: { path: { id } } })),
  })
}

/** Clients de l'agence, pour le champ « Client » du formulaire de projet. */
export function clientListQuery(search?: string) {
  return queryOptions({
    queryKey: ['clients', 'list', search ?? ''] as const,
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/clients', { params: { query: { search } } })),
  })
}

export interface CreateProjectValues {
  name: string
  client_id?: string
  client_name?: string
  status?: ProjectStatus
  progress?: number
  hours_sold?: number
  starts_on?: string | null
  due_on?: string | null
  team_ids?: string[]
  service_ids?: string[]
}

/**
 * Creation d'un projet.
 *
 * Invalide toutes les listes plutot que d'inserer la ligne a la main : le rang
 * du nouveau projet depend du tri et des filtres en cours, et le deviner cote
 * client donnerait une ligne au mauvais endroit jusqu'au prochain chargement.
 */
export function useCreateProject() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: CreateProjectValues) =>
      unwrap(await api.POST('/api/v1/admin/projects', { body: values })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: projectKeys.lists() })
    },
  })
}

export interface UpdateProjectValues {
  name?: string
  description?: string
  priority?: ProjectPriority
  client_id?: string
  status?: ProjectStatus
  progress?: number
  hours_sold?: number
  starts_on?: string | null
  due_on?: string | null
  figma_url?: string
  prod_url?: string
  preprod_url?: string
  /** Liste entiere des services ; absente pour ne rien changer. */
  service_ids?: string[]
}

/** Modification d'un projet. */
export function useUpdateProject(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: UpdateProjectValues) =>
      unwrap(
        await api.PATCH('/api/v1/admin/projects/{id}', {
          params: { path: { id } },
          body: values,
        }),
      ),
    onSuccess: (project) => {
      queryClient.setQueryData(projectKeys.detail(id), project)
      void queryClient.invalidateQueries({ queryKey: projectKeys.lists() })
    },
  })
}

/** Suppression logique d'un projet. */
export function useDeleteProject() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.DELETE('/api/v1/admin/projects/{id}', { params: { path: { id } } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: projectKeys.all })
    },
  })
}

/** Agregats du tableau de bord, precalcules et caches cote serveur. */
export const dashboardQuery = queryOptions({
  queryKey: ['dashboard'],
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/dashboard')),
})

/**
 * Comptes internes, pour les champs d'affectation.
 *
 * Cache long : l'equipe d'une agence ne change pas pendant qu'on remplit un
 * formulaire, et cette liste est ouverte par chaque menu d'affectation.
 */
export const peopleQuery = queryOptions({
  queryKey: ['users', 'internal'] as const,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/users')),
  staleTime: 5 * 60_000,
})

/**
 * Etoile d'un projet.
 *
 * L'etat bascule dans le cache avant la reponse : une etoile qui attend un
 * aller-retour pour s'allumer donne l'impression d'un clic manque. L'echec
 * remet la valeur precedente.
 */
export function useToggleFavorite(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (on: boolean) => {
      const path = '/api/v1/admin/projects/{id}/favorite' as const
      const options = { params: { path: { id } } }

      unwrap(on ? await api.PUT(path, options) : await api.DELETE(path, options))
    },
    onMutate: async (on) => {
      await queryClient.cancelQueries({ queryKey: projectKeys.detail(id) })
      const previous = queryClient.getQueryData<ProjectDetail>(projectKeys.detail(id))

      if (previous !== undefined) {
        queryClient.setQueryData<ProjectDetail>(projectKeys.detail(id), {
          ...previous,
          is_favorite: on,
        })
      }

      return { previous }
    },
    onError: (_error, _on, context) => {
      if (context?.previous !== undefined) {
        queryClient.setQueryData(projectKeys.detail(id), context.previous)
      }
    },
    // L'etoile ne decore pas la fiche : elle remonte le projet en tete de liste
    // et le pose dans les raccourcis. Les deux se relisent donc, dans un sens
    // comme dans l'autre — y compris apres un echec, ou l'ordre du serveur
    // reste la verite.
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: projectKeys.lists() })
      void queryClient.invalidateQueries({ queryKey: projectKeys.favorites() })
    },
  })
}

/** Depot d'une piece jointe sur le projet. */
export function useUploadProjectFile(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (file: File) =>
      postFile<Attachment>(`/api/v1/admin/projects/${id}/files`, 'file', file),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: projectKeys.detail(id) })
    },
  })
}

/** Suppression d'une piece jointe. */
export function useDeleteProjectFile(projectId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (fileId: string) =>
      unwrap(await api.DELETE('/api/v1/admin/files/{id}', { params: { path: { id: fileId } } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: projectKeys.detail(projectId) })
    },
  })
}

/**
 * Adresse de telechargement d'une piece jointe.
 *
 * Commune aux projets et aux taches : l'endpoint l'est aussi, une piece jointe
 * se lit par son seul identifiant.
 */
export function fileUrl(fileId: string): string {
  return apiUrl(`/api/v1/admin/files/${fileId}`)
}

/**
 * Remplace l'equipe d'un projet.
 *
 * L'API prend la liste complete et non un ajout ou un retrait : deux ecrans
 * ouverts cote a cote ne peuvent pas se marcher dessus en composant chacun sa
 * moitie de la liste.
 */
export function useSetTeam(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (userIDs: string[]) =>
      unwrap(
        await api.PUT('/api/v1/admin/projects/{id}/team', {
          params: { path: { id } },
          body: { user_ids: userIDs },
        }),
      ),
    onSuccess: (project) => {
      queryClient.setQueryData(projectKeys.detail(id), project)
      void queryClient.invalidateQueries({ queryKey: projectKeys.lists() })
    },
  })
}
