import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { TaskDrawer } from '@/components/task-drawer'
import { reportError } from '@/features/tasks/board'
import { taskListQuery, useMoveTaskInList } from '@/features/tasks/api'
import { TaskTable } from '@/features/tasks/list'

import { paramsOf } from '../taches'

export const Route = createFileRoute('/_app/pm/taches/')({
  component: TaskListPage,
})

/**
 * Vue liste de l'ecran « Tâches ».
 *
 * Le meme tableau que dans une fiche de projet, avec la colonne « Projet » en
 * plus : ici les lignes viennent de partout, et sans elle on ne saurait pas
 * de quoi on parle.
 */
function TaskListPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const params = paramsOf(search)
  const { data: list } = useQuery(taskListQuery(params))
  const move = useMoveTaskInList(params)

  // `replace` a la fermeture : ouvrir puis fermer une tache ne doit pas empiler
  // deux entrees d'historique.
  function openTask(taskId: string | null) {
    void navigate({ search: (prev) => ({ ...prev, tache: taskId ?? undefined }), replace: taskId === null })
  }

  return (
    <>
      <TaskTable
        tasks={list?.items ?? []}
        showProject
        pending={move.isPending}
        onToggle={(task, done) =>
          move.mutate(
            { id: task.id, projectId: task.project_id, status: done ? 'done' : 'todo' },
            { onError: reportError },
          )
        }
        onOpen={openTask}
        empty="Aucune tâche ne correspond."
      />

      <TaskDrawer taskId={search.tache ?? null} onClose={() => openTask(null)} />
    </>
  )
}
