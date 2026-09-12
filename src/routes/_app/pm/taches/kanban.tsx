import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { TaskDrawer } from '@/components/task-drawer'
import { TaskColumns, reportError } from '@/features/tasks/board'
import { taskListQuery, useMoveTaskInList } from '@/features/tasks/api'

import { paramsOf } from '../taches'

export const Route = createFileRoute('/_app/pm/taches/kanban')({
  component: TaskKanbanPage,
})

/**
 * Vue kanban de l'ecran « Tâches ».
 *
 * Le meme tableau que dans une fiche de projet, a deux details pres : chaque
 * carte porte le nom de son projet, et les colonnes n'offrent pas de creation
 * rapide — il faudrait demander dans quel projet creer a chaque clic, ce que
 * le bouton de la barre d'outils fait proprement.
 */
function TaskKanbanPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const params = paramsOf(search)
  const { data: list } = useQuery(taskListQuery(params))
  const move = useMoveTaskInList(params)

  function openTask(taskId: string | null) {
    void navigate({ search: (prev) => ({ ...prev, tache: taskId ?? undefined }), replace: taskId === null })
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <TaskColumns
        tasks={list?.items ?? []}
        onOpen={openTask}
        onMove={(task, status) =>
          move.mutate(
            { id: task.id, projectId: task.project_id, status },
            { onError: reportError },
          )
        }
      />

      <TaskDrawer taskId={search.tache ?? null} onClose={() => openTask(null)} />
    </div>
  )
}
