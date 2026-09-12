import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { projectKeys } from '@/features/projects/api'
import { api, unwrap } from '@/lib/api'
import type { TaskBoard, TaskList, TaskPriority, TaskStatus } from '@/types/api'

/**
 * Cles de cache des taches.
 *
 * Le tableau est indexe par projet, le detail par tache : une carte deplacee
 * doit rafraichir le tableau de son projet, et le panneau ouvert sur cette
 * tache, sans toucher aux autres.
 */
export const taskKeys = {
  all: ['tasks'] as const,
  board: (projectId: string) => [...taskKeys.all, 'board', projectId] as const,
  lists: () => [...taskKeys.all, 'list'] as const,
  list: (params: TaskListParams) => [...taskKeys.lists(), params] as const,
  detail: (id: string) => [...taskKeys.all, 'detail', id] as const,
  comments: (id: string) => [...taskKeys.all, 'comments', id] as const,
}

/** Ce que la barre d'outils de l'ecran « Taches » sait reduire. */
export interface TaskListParams {
  search?: string
  status?: TaskStatus
  priority?: TaskPriority
  projectId?: string
}

/**
 * Taches de toute l'agence.
 *
 * Une seule requete pour les deux vues : la liste et le kanban montrent le
 * meme jeu de taches, range autrement. Changer d'onglet ne recharge donc rien.
 *
 * `keepPreviousData` garde l'affichage precedent pendant qu'un filtre change :
 * sans lui, le tableau se viderait entre deux frappes de la recherche.
 */
export function taskListQuery(params: TaskListParams) {
  return queryOptions({
    queryKey: taskKeys.list(params),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/tasks', {
          params: {
            query: {
              search: params.search,
              status: params.status,
              priority: params.priority,
              project_id: params.projectId,
            },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/** Tableau des taches d'un projet. */
export function taskBoardQuery(projectId: string) {
  return queryOptions({
    queryKey: taskKeys.board(projectId),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/projects/{id}/tasks', {
          params: { path: { id: projectId } },
        }),
      ),
  })
}

/** Detail d'une tache : ce que le panneau lateral affiche a son ouverture. */
export function taskDetailQuery(id: string) {
  return queryOptions({
    queryKey: taskKeys.detail(id),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/tasks/{id}', { params: { path: { id } } })),
  })
}

/**
 * Commentaires d'une tache.
 *
 * Requete a part, comme cote serveur : ils vivent dans un onglet, et les
 * charger a l'ouverture du panneau ferait payer a chaque consultation une
 * liste que la plupart ne regardent pas.
 */
export function taskCommentsQuery(id: string) {
  return queryOptions({
    queryKey: taskKeys.comments(id),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/tasks/{id}/comments', { params: { path: { id } } })),
  })
}

export interface CreateTaskValues {
  title: string
  description?: string
  status?: TaskStatus
  tag?: string
  priority?: TaskPriority
  starts_on?: string | null
  due_on?: string | null
  hours?: number | null
  note?: string
  assignee_ids?: string[]
}

/**
 * Rafraichit ce qu'une ecriture sur une tache a pu changer.
 *
 * Le detail, le tableau du projet, et l'en-tete du projet : les compteurs de
 * taches y sont affiches, et ils bougent des qu'une tache change de colonne.
 */
function useTaskInvalidation() {
  const queryClient = useQueryClient()

  return (taskId: string | null, projectId: string | null) => {
    void queryClient.invalidateQueries({ queryKey: taskKeys.lists() })

    if (taskId !== null) {
      void queryClient.invalidateQueries({ queryKey: taskKeys.detail(taskId) })
    }
    if (projectId !== null) {
      void queryClient.invalidateQueries({ queryKey: taskKeys.board(projectId) })
      void queryClient.invalidateQueries({ queryKey: projectKeys.detail(projectId) })
      void queryClient.invalidateQueries({ queryKey: projectKeys.lists() })
    }
  }
}

/** Creation d'une tache dans un projet. */
export function useCreateTask(projectId: string) {
  const invalidate = useTaskInvalidation()

  return useMutation({
    mutationFn: async (values: CreateTaskValues) =>
      unwrap(
        await api.POST('/api/v1/admin/projects/{id}/tasks', {
          params: { path: { id: projectId } },
          body: values,
        }),
      ),
    onSuccess: () => invalidate(null, projectId),
  })
}

export interface UpdateTaskValues {
  title?: string
  description?: string
  tag?: string
  note?: string
  hours?: number | null
  starts_on?: string | null
  due_on?: string | null
  priority?: TaskPriority
}

/**
 * Modification d'un champ du panneau.
 *
 * La reponse remplace le detail en cache plutot que de declencher une seconde
 * lecture : l'API renvoie la tache complete, la relire serait un aller-retour
 * pour une valeur qu'on tient deja.
 */
export function useUpdateTask(taskId: string, projectId: string) {
  const queryClient = useQueryClient()
  const invalidate = useTaskInvalidation()

  return useMutation({
    mutationFn: async (values: UpdateTaskValues) =>
      unwrap(
        await api.PATCH('/api/v1/admin/tasks/{id}', {
          params: { path: { id: taskId } },
          body: values,
        }),
      ),
    onSuccess: (task) => {
      queryClient.setQueryData(taskKeys.detail(taskId), task)
      invalidate(null, projectId)
    },
  })
}

/** Deplacement d'une tache dans le tableau. */
export function useMoveTask(projectId: string) {
  const queryClient = useQueryClient()
  const invalidate = useTaskInvalidation()

  return useMutation({
    mutationFn: async ({
      id,
      status,
      position,
    }: {
      id: string
      status: TaskStatus
      position?: number
    }) =>
      unwrap(
        await api.POST('/api/v1/admin/tasks/{id}/move', {
          params: { path: { id } },
          body: { status, position },
        }),
      ),
    // La carte change de colonne avant que le serveur ait repondu.
    //
    // Sans cela, elle revenait a sa place le temps de l'aller-retour puis
    // sautait dans la bonne colonne : on croyait avoir rate son geste. Le
    // cache est remis en etat si l'ecriture echoue.
    onMutate: async ({ id, status }) => {
      await queryClient.cancelQueries({ queryKey: taskKeys.board(projectId) })

      const previous = queryClient.getQueryData<TaskBoard>(taskKeys.board(projectId))

      if (previous !== undefined) {
        queryClient.setQueryData<TaskBoard>(taskKeys.board(projectId), {
          ...previous,
          items: previous.items.map((task) => (task.id === id ? { ...task, status } : task)),
        })
      }

      return { previous }
    },
    onError: (_error, _variables, context) => {
      if (context?.previous !== undefined) {
        queryClient.setQueryData(taskKeys.board(projectId), context.previous)
      }
    },
    onSuccess: (task) => {
      queryClient.setQueryData(taskKeys.detail(task.id), task)
    },
    // Dans les deux cas : la position exacte et les compteurs viennent du
    // serveur, l'estimation optimiste ne portait que la colonne.
    onSettled: () => invalidate(null, projectId),
  })
}

/**
 * Deplacement d'une tache depuis l'ecran global.
 *
 * Meme endpoint que dans un projet, autre cache : la mise a jour optimiste
 * porte ici sur la liste filtree et non sur le tableau d'un projet. C'est la
 * seule chose qui distingue les deux — et la raison de ne pas avoir tordu
 * `useMoveTask` pour servir les deux ecrans avec un parametre de plus.
 *
 * Le projet de la tache voyage dans les variables : une liste qui traverse
 * les projets ne peut pas le tenir de son contexte, alors qu'il faut bien
 * rafraichir le tableau et les compteurs du projet touche.
 */
export function useMoveTaskInList(params: TaskListParams) {
  const queryClient = useQueryClient()
  const invalidate = useTaskInvalidation()

  return useMutation({
    mutationFn: async ({ id, status }: { id: string; projectId: string; status: TaskStatus }) =>
      unwrap(
        await api.POST('/api/v1/admin/tasks/{id}/move', {
          params: { path: { id } },
          body: { status },
        }),
      ),
    onMutate: async ({ id, status }) => {
      const key = taskKeys.list(params)

      await queryClient.cancelQueries({ queryKey: key })
      const previous = queryClient.getQueryData<TaskList>(key)

      if (previous !== undefined) {
        queryClient.setQueryData<TaskList>(key, {
          ...previous,
          items: previous.items.map((task) => (task.id === id ? { ...task, status } : task)),
        })
      }

      return { previous }
    },
    onError: (_error, _variables, context) => {
      if (context?.previous !== undefined) {
        queryClient.setQueryData(taskKeys.list(params), context.previous)
      }
    },
    onSuccess: (task) => {
      queryClient.setQueryData(taskKeys.detail(task.id), task)
    },
    onSettled: (_data, _error, variables) => invalidate(null, variables.projectId),
  })
}

/** Remplacement des personnes affectees. */
export function useSetAssignees(taskId: string, projectId: string) {
  const queryClient = useQueryClient()
  const invalidate = useTaskInvalidation()

  return useMutation({
    mutationFn: async (userIds: string[]) =>
      unwrap(
        await api.PUT('/api/v1/admin/tasks/{id}/assignees', {
          params: { path: { id: taskId } },
          body: { user_ids: userIds },
        }),
      ),
    onSuccess: (task) => {
      queryClient.setQueryData(taskKeys.detail(taskId), task)
      invalidate(null, projectId)
    },
  })
}

/** Suppression d'une tache. */
export function useDeleteTask(projectId: string) {
  const invalidate = useTaskInvalidation()

  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.DELETE('/api/v1/admin/tasks/{id}', { params: { path: { id } } })),
    onSuccess: () => invalidate(null, projectId),
  })
}

/** Ajout d'une ligne a cocher. */
export function useAddSubtask(taskId: string) {
  const invalidate = useTaskInvalidation()

  return useMutation({
    mutationFn: async (label: string) =>
      unwrap(
        await api.POST('/api/v1/admin/tasks/{id}/subtasks', {
          params: { path: { id: taskId } },
          body: { label },
        }),
      ),
    onSuccess: () => invalidate(taskId, null),
  })
}

/** Renommage, cochage ou deplacement d'une sous-tache. */
export function useUpdateSubtask(taskId: string) {
  const invalidate = useTaskInvalidation()

  return useMutation({
    mutationFn: async ({
      id,
      label,
      done,
      position,
    }: {
      id: string
      label?: string
      done?: boolean
      position?: number
    }) =>
      unwrap(
        await api.PATCH('/api/v1/admin/subtasks/{id}', {
          params: { path: { id } },
          body: { label, done, position },
        }),
      ),
    onSuccess: () => invalidate(taskId, null),
  })
}

/** Suppression d'une sous-tache. */
export function useDeleteSubtask(taskId: string) {
  const invalidate = useTaskInvalidation()

  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.DELETE('/api/v1/admin/subtasks/{id}', { params: { path: { id } } })),
    onSuccess: () => invalidate(taskId, null),
  })
}

/** Publication d'un commentaire. */
export function useAddComment(taskId: string) {
  const queryClient = useQueryClient()
  const invalidate = useTaskInvalidation()

  return useMutation({
    mutationFn: async (body: string) =>
      unwrap(
        await api.POST('/api/v1/admin/tasks/{id}/comments', {
          params: { path: { id: taskId } },
          body: { body },
        }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: taskKeys.comments(taskId) })
      // Le journal de la tache gagne une ligne « commented » : le panneau doit
      // la voir apparaitre sans qu'on le rouvre.
      invalidate(taskId, null)
    },
  })
}
