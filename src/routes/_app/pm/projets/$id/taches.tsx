import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { TaskDrawer } from '@/components/task-drawer'
import { taskBoardQuery, useCreateTask, useMoveTask } from '@/features/tasks/api'
import { TaskColumns, reportError } from '@/features/tasks/board'
import { TaskTable } from '@/features/tasks/list'

/**
 * Les taches du projet, en tableau ou en kanban.
 *
 * Une seule route pour les deux : `?vue` dit comment regarder, et il est
 * declare par le chassis pour que passer aux tickets garde la meme facon de
 * lire. Les deux vues lisent la meme requete — c'est le meme jeu de donnees,
 * pas deux chargements.
 *
 * `?tache` ouvre le tiroir, depuis le tableau comme depuis le kanban : une
 * tache s'ouvre de la meme facon partout dans le module, et un lien vers une
 * tache reste un lien vers une tache.
 */
const searchSchema = z.object({
  tache: z.string().optional(),
})

export const Route = createFileRoute('/_app/pm/projets/$id/taches')({
  validateSearch: searchSchema,
  loader: ({ context, params }) =>
    context.queryClient.query({ ...taskBoardQuery(params.id), staleTime: 'static' }),
  component: ProjectTasksPage,
})

function ProjectTasksPage() {
  const { id } = Route.useParams()
  const { vue, tache } = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data: board } = useQuery(taskBoardQuery(id))
  const move = useMoveTask(id)
  const create = useCreateTask(id)

  // `replace` : ouvrir puis fermer une tache ne doit pas empiler deux entrees
  // d'historique, sinon le bouton Retour rouvrirait ce qu'on vient de fermer.
  function openTask(taskId: string | null) {
    void navigate({ search: (prev) => ({ ...prev, tache: taskId ?? undefined }), replace: taskId === null })
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {vue !== 'kanban' ? (
        <TaskTable
          tasks={board?.items ?? []}
          pending={move.isPending}
          onToggle={(task, done) => move.mutate({ id: task.id, status: done ? 'done' : 'todo' })}
          onOpen={openTask}
          empty="Aucune tâche pour l’instant. Créez la première depuis « Créer une tâche »."
        />
      ) : (
        <>
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
            onCreate={(status, title) =>
              create.mutate({ title, status }, { onError: reportError })
            }
          />
        </>
      )}

      {/* Hors des deux branches : une tache s'ouvre depuis le tableau comme
          depuis le kanban, et changer de vue ne doit pas refermer le tiroir. */}
      <TaskDrawer taskId={tache ?? null} onClose={() => openTask(null)} />
    </div>
  )
}
