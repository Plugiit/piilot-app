import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { TaskTable } from '@/features/tasks/list'
import { taskBoardQuery, useMoveTask } from '@/features/tasks/api'

export const Route = createFileRoute('/_app/pm/projets/$id/')({
  loader: ({ context, params }) =>
    context.queryClient.query({ ...taskBoardQuery(params.id), staleTime: 'static' }),
  component: ProjectTaskListPage,
})

/**
 * Vue liste des taches du projet.
 *
 * Le tableau vit dans `features/tasks/list`, partage avec l'ecran « Taches »
 * du module. La route ne garde que le projet dont elle lit les taches — et
 * n'affiche pas la colonne « Projet » : le titre de la page le dit deja.
 */
function ProjectTaskListPage() {
  const { id } = Route.useParams()
  const { data: board } = useQuery(taskBoardQuery(id))
  const move = useMoveTask(id)

  return (
    <TaskTable
      tasks={board?.items ?? []}
      pending={move.isPending}
      onToggle={(task, done) => move.mutate({ id: task.id, status: done ? 'done' : 'todo' })}
      empty="Aucune tâche pour l’instant. Créez la première depuis « Créer une tâche »."
    />
  )
}
