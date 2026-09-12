import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { TaskDrawer } from '@/components/task-drawer'
import { TaskColumns, reportError } from '@/features/tasks/board'
import { taskBoardQuery, useCreateTask, useMoveTask } from '@/features/tasks/api'

/**
 * La tache ouverte vit dans l'URL, comme sur le tableau de bord.
 *
 * Meme parametre, `?tache=`, et meme tiroir : une tache s'ouvre de la meme
 * facon partout dans le module, et un lien vers une tache reste un lien vers
 * une tache.
 */
const searchSchema = z.object({
  tache: z.string().optional(),
})

export const Route = createFileRoute('/_app/pm/projets/$id/kanban')({
  validateSearch: searchSchema,
  loader: ({ context, params }) =>
    context.queryClient.query({ ...taskBoardQuery(params.id), staleTime: 'static' }),
  component: ProjectTasksPage,
})

/**
 * Tableau des taches du projet.
 *
 * Les colonnes, les cartes et le glissement vivent dans `features/tasks/board`,
 * partages avec l'ecran « Taches » du module. Cette route ne garde que ce qui
 * lui est propre : le projet dont elle lit les taches, et ou vont ses
 * ecritures.
 */
function ProjectTasksPage() {
  const { id } = Route.useParams()
  const { tache } = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data: board } = useQuery(taskBoardQuery(id))
  const move = useMoveTask(id)
  const create = useCreateTask(id)

  // `replace` : ouvrir puis fermer une tache ne doit pas empiler deux entrees
  // d'historique, sinon le bouton Retour rouvrirait ce qu'on vient de fermer.
  function openTask(taskId: string | null) {
    void navigate({ search: { tache: taskId ?? undefined }, replace: taskId === null })
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {board !== undefined && board.total > board.limit && (
        <p className="px-4 pt-4 text-[12px] text-[#73757c]">
          {board.total} tâches dans ce projet, {board.limit} affichées.
        </p>
      )}

      <TaskColumns
        tasks={board?.items ?? []}
        onOpen={openTask}
        onMove={(task, status) =>
          move.mutate({ id: task.id, status }, { onError: reportError })
        }
        onCreate={(status, title) => create.mutate({ title, status }, { onError: reportError })}
      />

      <TaskDrawer taskId={tache ?? null} onClose={() => openTask(null)} />
    </div>
  )
}
