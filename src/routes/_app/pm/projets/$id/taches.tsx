import { ArrowRight02Icon, DragDropVerticalIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { createFileRoute, getRouteApi } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'framer-motion'
import { useState } from 'react'
import { z } from 'zod'

import { HoverMenuContent, HoverMenuItem } from '@/components/hover-menu'
import { DropdownMenu, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { TaskDrawer } from '@/components/task-drawer'
import {
  TASK_STATUS,
  TASK_STATUS_ORDER,
  tasksOf,
  type TaskStatus,
} from '@/features/projects/fixtures'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })

const parent = getRouteApi('/_app/pm/projets/$id')

/**
 * La tache ouverte vit dans l'URL, comme sur le tableau de bord.
 *
 * Meme parametre, `?tache=`, et meme tiroir : une tache s'ouvre de la meme
 * facon partout dans le module. Un lien vers une tache reste un lien vers une
 * tache, qu'il vienne de l'agenda ou du tableau.
 */
const searchSchema = z.object({
  tache: z.string().optional(),
})

export const Route = createFileRoute('/_app/pm/projets/$id/taches')({
  validateSearch: searchSchema,
  component: ProjectTasksPage,
})

type BoardTask = ReturnType<typeof tasksOf>[number]

function inDays(offset: number) {
  const date = new Date()
  date.setHours(0, 0, 0, 0)
  date.setDate(date.getDate() + offset)

  return date
}

/**
 * Carte d'une tache.
 *
 * Le titre est un bouton, le cadre est saisissable : deux gestes distincts sur
 * le meme objet. Ouvrir passe par le clic, deplacer par le glissement — et par
 * le menu, sans quoi le tableau ne serait utilisable qu'a la souris.
 */
function TaskCard({
  task,
  onOpen,
  onMove,
  onDragStart,
  onDragEnd,
  dragged,
}: {
  task: BoardTask
  onOpen: () => void
  onMove: (status: TaskStatus) => void
  onDragStart: () => void
  onDragEnd: () => void
  dragged: boolean
}) {
  const due = inDays(task.offset)
  const late = task.status !== 'done' && task.offset < 0

  return (
    <article
      draggable
      onDragStart={(event) => {
        // Le format `text/plain` est le seul que tous les navigateurs
        // acceptent sans ceremonie ; l'identifiant suffit, le reste se
        // retrouve dans l'etat.
        event.dataTransfer.setData('text/plain', task.id)
        event.dataTransfer.effectAllowed = 'move'
        onDragStart()
      }}
      onDragEnd={onDragEnd}
      className={cn(
        'group flex cursor-grab flex-col gap-2 rounded-[8px] border border-[#ebebeb] bg-white p-3 shadow-[0_1px_2px_0_rgb(16_24_40/0.05)] transition-[border-color,opacity] hover:border-[#c4c4c4] active:cursor-grabbing',
        // La carte saisie s'efface sans quitter la colonne : la retirer
        // ferait remonter les suivantes sous le curseur, et la place ou on
        // voulait la reposer aurait bouge.
        dragged && 'opacity-40',
      )}
    >
      <button type="button" onClick={onOpen} className="flex w-full flex-col gap-1.5 text-left">
        <p className="text-[13px] leading-[1.4] font-medium text-[#111]">{task.title}</p>
        <span className="text-[12px] text-[#777]">{task.tag}</span>
      </button>

      <footer className="flex items-center gap-2">
        <span
          className="flex size-6 shrink-0 items-center justify-center rounded-[6px] text-[10px] font-semibold text-[#0f172a]"
          style={{ backgroundColor: task.assignee.tint }}
          title={task.assignee.name}
        >
          {task.assignee.initials}
        </span>

        <span
          className={cn(
            'text-[12px] whitespace-nowrap tabular-nums',
            late ? 'font-medium text-[#e5484d]' : 'text-[#8d8d8d]',
          )}
        >
          {DATE_FORMAT.format(due)}
        </span>

        <span className="text-[12px] text-[#c4c4c4]">·</span>
        <span className="text-[12px] text-[#8d8d8d] tabular-nums">{task.hours} h</span>

        {/* Le glissement ne suffit pas : il n'existe pas au clavier. Ce menu
            est la meme action par un autre chemin, et non un doublon de
            confort. */}
        <DropdownMenu>
          <DropdownMenuTrigger
            aria-label={`Déplacer « ${task.title} »`}
            className="ml-auto flex shrink-0 cursor-pointer items-center justify-center rounded-[6px] p-1 text-[#c4c4c4] transition-colors hover:bg-[#f8f8f8] hover:text-[#777] focus-visible:opacity-100 aria-expanded:bg-[#f8f8f8]"
          >
            <HugeiconsIcon icon={DragDropVerticalIcon} size={16} strokeWidth={1.5} />
          </DropdownMenuTrigger>

          <HoverMenuContent align="end" className="min-w-[180px]">
            {TASK_STATUS_ORDER.filter((status) => status !== task.status).map((status) => (
              <HoverMenuItem key={status} onSelect={() => onMove(status)}>
                <HugeiconsIcon
                  icon={ArrowRight02Icon}
                  size={14}
                  strokeWidth={1.6}
                  className="text-[#999]"
                />
                <span className="truncate">{TASK_STATUS[status].label}</span>
              </HoverMenuItem>
            ))}
          </HoverMenuContent>
        </DropdownMenu>
      </footer>
    </article>
  )
}

/**
 * Tableau des taches du projet.
 *
 * Les colonnes sont les statuts, dans l'ordre du flux. La charge s'affiche a
 * cote du compte : dix taches d'une heure et deux taches de trois jours ne
 * pesent pas pareil, et seul le compte le cacherait.
 */
function ProjectTasksPage() {
  const project = parent.useLoaderData()
  const { tache } = Route.useSearch()
  const navigate = Route.useNavigate()
  const transition = useSlideTransition()

  // Les deplacements vivent dans l'ecran tant qu'aucun endpoint ne les
  // enregistre. L'etat part des fixtures du projet ouvert : changer de projet
  // remonte le composant, donc repart de ses propres taches.
  const [tasks, setTasks] = useState<BoardTask[]>(() => tasksOf(project))
  const [dragging, setDragging] = useState<string | null>(null)
  const [over, setOver] = useState<TaskStatus | null>(null)

  function move(id: string, status: TaskStatus) {
    setTasks((previous) =>
      previous.map((task) => (task.id === id ? { ...task, status } : task)),
    )
  }

  // `replace` : ouvrir puis fermer une tache ne doit pas empiler deux entrees
  // d'historique, sinon le bouton Retour rouvrirait ce qu'on vient de fermer.
  function openTask(id: string | null) {
    void navigate({ search: { tache: id ?? undefined }, replace: id === null })
  }

  const opened = tasks.find((task) => task.id === tache) ?? null

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex gap-3 overflow-x-auto p-4">
        {TASK_STATUS_ORDER.map((status) => {
          const column = tasks.filter((task) => task.status === status)
          const hours = column.reduce((sum, task) => sum + task.hours, 0)
          const tone = TASK_STATUS[status]

          return (
            <section
              key={status}
              onDragOver={(event) => {
                // Sans ce `preventDefault`, le navigateur refuse le depot :
                // par defaut, aucune zone n'accepte ce qu'on lui amene.
                event.preventDefault()
                event.dataTransfer.dropEffect = 'move'
                setOver(status)
              }}
              onDragLeave={(event) => {
                // Le survol d'une carte fille declenche un `dragleave` sur la
                // colonne : sans ce test, la colonne clignoterait a chaque
                // carte traversee.
                if (!event.currentTarget.contains(event.relatedTarget as Node)) setOver(null)
              }}
              onDrop={(event) => {
                event.preventDefault()
                const id = event.dataTransfer.getData('text/plain')

                if (id !== '') move(id, status)

                setOver(null)
                setDragging(null)
              }}
              className={cn(
                'flex w-[300px] shrink-0 flex-col rounded-[12px] border bg-[#f8f8f8] px-2 pt-3 pb-2 transition-colors',
                over === status && dragging !== null
                  ? 'border-[#ff782b] bg-[#fef3eb]'
                  : 'border-[#ebebeb]',
              )}
            >
              <header className="flex items-center gap-2 px-2 pb-3">
                <span
                  aria-hidden
                  className="size-2 shrink-0 rounded-full"
                  style={{ backgroundColor: tone.color }}
                />
                <h2 className="text-[14px] leading-[1.5] font-medium tracking-[0.56px] text-[#606060]">
                  {tone.label}
                </h2>
                <span className="text-[12px] text-[#999] tabular-nums">{column.length}</span>
                <span className="ml-auto text-[12px] text-[#999] tabular-nums">{hours} h</span>
              </header>

              {/* `min-h` et non une hauteur : une colonne vide doit rester une
                  cible de depot, mais elle n'a pas a occuper l'ecran. */}
              <div className="flex min-h-[120px] flex-1 flex-col gap-2">
                <AnimatePresence initial={false} mode="popLayout">
                  {column.map((task) => (
                    <motion.div
                      key={task.id}
                      layout
                      initial={{ opacity: 0, scale: 0.97 }}
                      animate={{ opacity: 1, scale: 1 }}
                      exit={{ opacity: 0, scale: 0.97 }}
                      transition={transition}
                    >
                      <TaskCard
                        task={task}
                        dragged={dragging === task.id}
                        onOpen={() => openTask(task.id)}
                        onMove={(next) => move(task.id, next)}
                        onDragStart={() => setDragging(task.id)}
                        onDragEnd={() => {
                          setDragging(null)
                          setOver(null)
                        }}
                      />
                    </motion.div>
                  ))}
                </AnimatePresence>

                {column.length === 0 && (
                  <p className="px-2 py-6 text-center text-[12px] text-[#c4c4c4]">
                    Rien ici pour le moment.
                  </p>
                )}
              </div>
            </section>
          )
        })}
      </div>

      <TaskDrawer
        task={
          opened === null
            ? null
            : {
                id: opened.id,
                title: opened.title,
                category: opened.tag,
                color: TASK_STATUS[opened.status].color,
                when: DATE_FORMAT.format(inDays(opened.offset)),
                status: opened.status,
                project: project.name,
                assignee: opened.assignee,
                hours: opened.hours,
              }
        }
        onClose={() => openTask(null)}
      />
    </div>
  )
}
