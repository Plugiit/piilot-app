import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { TaskDrawer } from '@/components/task-drawer'
import { Button } from '@/components/ui/button'
import { reportError } from '@/features/tasks/board'
import { TASK_PAGE_SIZE, taskListQuery, useMoveTaskInList } from '@/features/tasks/api'
import { TaskTable } from '@/features/tasks/list'

import { paramsOf } from '../taches'

export const Route = createFileRoute('/_app/pm/taches/')({
  // Seuls les filtres et la page declenchent un rechargement : ouvrir une
  // tache change l'adresse sans rien changer a la liste.
  loaderDeps: ({ search }) => paramsOf(search),
  loader: ({ context, deps }) =>
    context.queryClient.query({ ...taskListQuery(deps), staleTime: 'static' }),
  component: TaskListPage,
})

/**
 * Vue liste de l'ecran « Tâches ».
 *
 * Le meme tableau que dans une fiche de projet, avec la colonne « Projet » en
 * plus : ici les lignes viennent de partout, et sans elle on ne saurait pas
 * de quoi on parle. Paginee cote serveur, comme toute liste.
 */
function TaskListPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const params = paramsOf(search)
  const query = taskListQuery(params)
  const { data: list } = useQuery(query)
  const move = useMoveTaskInList(query.queryKey)

  const page = search.page ?? 1
  const total = list?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / TASK_PAGE_SIZE))

  // `replace` a la fermeture : ouvrir puis fermer une tache ne doit pas empiler
  // deux entrees d'historique.
  function openTask(taskId: string | null) {
    void navigate({ search: (prev) => ({ ...prev, tache: taskId ?? undefined }), replace: taskId === null })
  }

  function goTo(next: number) {
    void navigate({ search: (prev) => ({ ...prev, page: next === 1 ? undefined : next }) })
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

      <div className="flex flex-wrap items-center justify-between gap-2 p-4">
        <p className="text-[12px] text-[#777]">
          {total} tâche{total > 1 ? 's' : ''} · page {page} sur {totalPages}
        </p>

        <div className="flex gap-2">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => goTo(page - 1)}>
            Précédent
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => goTo(page + 1)}
          >
            Suivant
          </Button>
        </div>
      </div>

      <TaskDrawer taskId={search.tache ?? null} onClose={() => openTask(null)} />
    </>
  )
}
