import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { TaskDrawer } from '@/components/task-drawer'
import { TaskColumns, reportError } from '@/features/tasks/board'
import { taskGlobalBoardQuery, useMoveTaskInList } from '@/features/tasks/api'

import { paramsOf } from '../taches'

export const Route = createFileRoute('/_app/pm/taches/kanban')({
  // La page de la liste n'entre pas dans les dependances : le kanban ne la lit
  // pas, et la changer ne doit rien recharger ici.
  loaderDeps: ({ search }) => ({ ...paramsOf(search), page: undefined }),
  loader: ({ context, deps }) =>
    context.queryClient.query({ ...taskGlobalBoardQuery(deps), staleTime: 'static' }),
  component: TaskKanbanPage,
})

/**
 * Vue kanban de l'ecran « Tâches ».
 *
 * Le meme tableau que dans une fiche de projet, a deux details pres : chaque
 * carte porte le nom de son projet, et les colonnes n'offrent pas de creation
 * rapide — il faudrait demander dans quel projet creer a chaque clic, ce que
 * le bouton de la barre d'outils fait proprement.
 *
 * Chaque colonne est plafonnee cote serveur et porte son total : une colonne
 * « Terminé » qui grossit sans fin ne prend plus la place des autres.
 */
function TaskKanbanPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const query = taskGlobalBoardQuery(paramsOf(search))
  const { data: board } = useQuery(query)
  const move = useMoveTaskInList(query.queryKey)

  function openTask(taskId: string | null) {
    void navigate({ search: (prev) => ({ ...prev, tache: taskId ?? undefined }), replace: taskId === null })
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <TaskColumns
        tasks={board?.items ?? []}
        totals={board?.columns}
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
