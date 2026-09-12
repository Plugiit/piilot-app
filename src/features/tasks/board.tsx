import {
  Attachment02Icon,
  Calendar03Icon,
  Message01Icon,
  MoreHorizontalIcon,
  PlusSignIcon,
  ArrowRight02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { motion } from 'framer-motion'
import { useRef, useState } from 'react'
import { toast } from 'sonner'

import { HoverMenuContent, HoverMenuItem } from '@/components/hover-menu'
import { DropdownMenu, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import {
  DONE_COLOR,
  PROGRESS_COLOR,
  TASK_STATUS,
  TASK_STATUS_ORDER,
  parseApiDate,
} from '@/features/projects/format'
import { Avatars, PriorityTag } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { TaskStatus, TaskSummary } from '@/types/api'

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

/**
 * Tableau des taches, partage par la fiche d'un projet et par l'ecran du
 * module.
 *
 * Les deux dessinent exactement le meme kanban : memes colonnes, memes cartes,
 * meme glissement. Ce qui les separe tient en deux points, et ce sont les deux
 * seules choses que ce fichier parametre — la carte dit de quel projet elle
 * vient quand elle sort de sa fiche, et la creation par colonne n'existe que
 * la ou le projet cible est connu.
 *
 * Le reste vivait dans la route du projet ; l'y laisser aurait fait recopier
 * quatre cents lignes de glisser-deposer dans l'ecran global.
 */

/**
 * Une tache telle que le tableau la dessine.
 *
 * `project_name` est facultatif : dans une fiche de projet la reponse ne le
 * porte pas, et l'afficher reviendrait a repeter le titre de la page sur
 * chaque carte.
 */
export type BoardTask = TaskSummary & { project_name?: string }

/** Position du pointeur, quel que soit le type d'evenement rendu par le geste. */
function pointerPosition(event: MouseEvent | TouchEvent | PointerEvent) {
  if ('clientX' in event) return { x: event.clientX, y: event.clientY }

  const touch = event.changedTouches[0]

  return touch === undefined ? null : { x: touch.clientX, y: touch.clientY }
}

/** Signale l'echec d'une ecriture. Le succes, lui, se voit a l'ecran. */
export function reportError(error: unknown) {
  toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible')
}

/**
 * Avancement des sous-taches, en segments.
 *
 * Un segment par sous-tache plutot qu'une barre continue : sur une tache qui
 * en compte trois, une barre au tiers et une barre aux deux tiers se
 * ressemblent, alors que « une sur trois » et « deux sur trois » se comptent
 * d'un coup d'oeil.
 */
function SubtaskBar({ total, done }: { total: number; done: number }) {
  return (
    <div className="flex w-full items-start gap-1" aria-hidden>
      {Array.from({ length: total }, (_, index) => (
        <span
          key={index}
          className="h-1 min-w-px flex-1 rounded-full"
          style={{
            backgroundColor:
              index < done ? (done === total ? DONE_COLOR : PROGRESS_COLOR) : '#e8e8e9',
          }}
        />
      ))}
    </div>
  )
}

/** Un compteur de la barre d'actions : icone puis nombre. */
function CountBadge({ icon, value, label }: { icon: typeof Message01Icon; value: number; label: string }) {
  return (
    <span className="flex items-center gap-0.5 text-[12px] text-[#73757c]" title={label}>
      <HugeiconsIcon icon={icon} size={16} strokeWidth={1.6} />
      {value}
    </span>
  )
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
  today,
  onOpen,
  onMove,
  onGrab,
  onCarry,
  onRelease,
}: {
  task: BoardTask
  /** Minuit d'aujourd'hui, calcule une fois par la page. */
  today: number
  onOpen: () => void
  onMove: (status: TaskStatus) => void
  onGrab: () => void
  onCarry: (point: { x: number; y: number }) => void
  onRelease: (point: { x: number; y: number } | null) => void
}) {
  const due = parseApiDate(task.due_on)
  const late = task.status !== 'done' && due !== null && due.getTime() < today

  // Un vrai glissement ne doit pas se terminer en ouverture de tache : le
  // relachement au-dessus du titre declencherait sinon son clic.
  const carried = useRef(false)

  return (
    <motion.article
      drag
      // La carte revient d'elle-meme si elle est laissee hors d'une colonne.
      // Quand elle change de colonne, la mise a jour optimiste la redessine a
      // sa nouvelle place avant que l'animation de retour ne se voie.
      dragSnapToOrigin
      dragElastic={0.12}
      dragMomentum={false}
      // Souleve plutot qu'efface : la carte reste opaque et sous le curseur,
      // c'est l'ombre portee et l'echelle qui disent qu'elle est en main.
      whileDrag={{ scale: 1.03, boxShadow: '0 12px 28px -8px rgb(16 24 40 / 0.28)', zIndex: 40 }}
      onDragStart={() => {
        carried.current = true
        onGrab()
      }}
      onDrag={(event) => {
        const point = pointerPosition(event)
        if (point !== null) onCarry(point)
      }}
      onDragEnd={(event) => {
        onRelease(pointerPosition(event))
        // Rendu au tour suivant : le clic de fin de geste part avant.
        window.setTimeout(() => {
          carried.current = false
        }, 0)
      }}
      className="group relative flex cursor-grab flex-col gap-3 rounded-[10px] bg-white p-3 transition-shadow hover:shadow-[0_1px_4px_0_rgb(16_24_40/0.10)] active:cursor-grabbing"
    >
      {/* La colonne dit deja le statut : la vignette porte donc la priorite,
          qui elle ne se lit nulle part ailleurs sur le tableau. */}
      <div className="flex min-w-0 items-center gap-2">
        <PriorityTag priority={task.priority} />

        {/* Hors de la fiche d'un projet, une carte doit dire d'ou elle vient :
            quatre colonnes de taches melangees ne se lisent pas autrement. */}
        {task.project_name !== undefined && (
          <span className="truncate text-[12px] text-[#73757c]">{task.project_name}</span>
        )}
      </div>

      <button
        type="button"
        onClick={() => {
          if (!carried.current) onOpen()
        }}
        className="flex w-full flex-col gap-0.5 text-left"
      >
        <span className="line-clamp-1 text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">
          {task.title}
        </span>
        {task.description !== '' && (
          <span className="line-clamp-1 text-[14px] leading-[1.5] text-[#73757c]">
            {task.description}
          </span>
        )}
      </button>

      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between gap-2">
          <span
            className={cn(
              'flex items-center gap-0.5 text-[12px] whitespace-nowrap',
              late ? 'font-medium text-[#e5484d]' : 'text-[#73757c]',
            )}
          >
            <HugeiconsIcon icon={Calendar03Icon} size={16} strokeWidth={1.6} />
            {due === null ? 'Sans échéance' : DATE_FORMAT.format(due)}
          </span>

          {task.subtasks_total > 0 && (
            <span className="shrink-0 text-[12px] text-[#73757c] tabular-nums">
              {task.subtasks_done}/{task.subtasks_total}
            </span>
          )}
        </div>

        {task.subtasks_total > 0 && (
          <SubtaskBar total={task.subtasks_total} done={task.subtasks_done} />
        )}
      </div>

      <div className="h-px w-full bg-[#e8e8e9]" />

      <footer className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2.5">
          <CountBadge icon={Message01Icon} value={task.comments_count} label="Commentaires" />
          <CountBadge icon={Attachment02Icon} value={task.attachments_count} label="Pièces jointes" />
        </div>

        <div className="flex items-center gap-2">
          <Avatars people={task.assignees} max={3} size={20} />

          {/* Le glissement ne suffit pas : il n'existe pas au clavier. Ce menu
              est la meme action par un autre chemin, et non un doublon de
              confort. */}
          <DropdownMenu>
            <DropdownMenuTrigger
              aria-label={`Déplacer « ${task.title} »`}
              className="flex shrink-0 cursor-pointer items-center justify-center rounded-[6px] p-0.5 text-[#a2a3a7] transition-colors hover:bg-[#f3f4f4] hover:text-[#1b1b1b] aria-expanded:bg-[#f3f4f4]"
            >
              <HugeiconsIcon icon={MoreHorizontalIcon} size={18} strokeWidth={1.8} />
            </DropdownMenuTrigger>

            <HoverMenuContent align="end" className="min-w-[180px]">
              {TASK_STATUS_ORDER.filter((status) => status !== task.status).map((status) => (
                <HoverMenuItem key={status} onSelect={() => onMove(status)}>
                  <HugeiconsIcon
                    icon={ArrowRight02Icon}
                    size={14}
                    strokeWidth={1.6}
                    className="text-[#a2a3a7]"
                  />
                  <span className="truncate">{TASK_STATUS[status].label}</span>
                </HoverMenuItem>
              ))}
            </HoverMenuContent>
          </DropdownMenu>
        </div>
      </footer>
    </motion.article>
  )
}


/**
 * Les quatre colonnes, dans l'ordre du flux.
 *
 * Deposer une carte ailleurs change son statut, c'est tout ce que le geste
 * veut dire. Le composant ne decide de rien : il signale les gestes, l'ecran
 * appelant decide de l'ecriture — c'est ce qui lui permet de servir un cache
 * par projet d'un cote, un cache global de l'autre.
 */
export function TaskColumns({
  tasks,
  onOpen,
  onMove,
  onCreate,
}: {
  tasks: BoardTask[]
  onOpen: (taskId: string) => void
  onMove: (task: BoardTask, status: TaskStatus) => void
  /**
   * Creation depuis l'en-tete d'une colonne. Absente quand l'ecran ne sait pas
   * dans quel projet creer — l'ecran global, qui traverse les projets : mieux
   * vaut pas de bouton qu'un bouton qui demande « lequel ? » a chaque clic.
   */
  onCreate?: (status: TaskStatus, title: string) => void
}) {
  // Minuit d'aujourd'hui, fige a l'arrivee sur l'ecran : lu pendant le rendu,
  // « maintenant » donnerait une valeur differente a chaque repeinture.
  const [today] = useState(() => {
    const midnight = new Date()
    midnight.setHours(0, 0, 0, 0)

    return midnight.getTime()
  })

  const [dragging, setDragging] = useState<string | null>(null)
  const [over, setOver] = useState<TaskStatus | null>(null)

  // Rectangles des colonnes, releves au moment ou l'on en a besoin.
  //
  // Le geste n'est plus un glisser-deposer HTML : c'est au code de dire
  // au-dessus de quelle colonne se trouve le pointeur. Les mesurer a chaque
  // deplacement plutot que les memoriser garde le compte juste quand le
  // tableau defile pendant le geste.
  const columnBoxes = useRef(new Map<TaskStatus, HTMLElement>())

  function columnAt(point: { x: number; y: number }): TaskStatus | null {
    for (const [status, element] of columnBoxes.current) {
      const box = element.getBoundingClientRect()

      if (
        point.x >= box.left &&
        point.x < box.right &&
        point.y >= box.top &&
        point.y < box.bottom
      ) {
        return status
      }
    }

    return null
  }

  function release(task: BoardTask, point: { x: number; y: number } | null) {
    setDragging(null)
    setOver(null)

    if (point === null) return

    const target = columnAt(point)
    if (target === null || target === task.status) return

    onMove(task, target)
  }

  /** Colonne dont le champ de creation est ouvert, et sa saisie. */
  const [drafting, setDrafting] = useState<{ status: TaskStatus; title: string } | null>(null)

  function submitDraft() {
    if (drafting === null || onCreate === undefined) return

    const title = drafting.title.trim()
    const status = drafting.status

    setDrafting(null)

    if (title === '') return

    onCreate(status, title)
  }

  return (
    /* `items-start` : chaque colonne s'arrete a son contenu, comme dans la
       maquette. Etirees a la meme hauteur, celles qui portent peu de taches
       affichaient une longue trainee grise sous leur derniere carte. */
    <div className="flex items-start gap-4 overflow-x-auto p-4">
      {TASK_STATUS_ORDER.map((status) => {
        const column = TASK_STATUS[status]
        const items = tasks.filter((task) => task.status === status)

        return (
          <section
            key={status}
            ref={(element) => {
              if (element === null) columnBoxes.current.delete(status)
              else columnBoxes.current.set(status, element)
            }}
            className={cn(
              'flex w-[362px] shrink-0 flex-col gap-1 rounded-[12px] border bg-[#f3f4f4] p-1 transition-colors',
              // La bordure reste presente mais transparente : la colorer au
              // survol ne doit pas decaler la colonne d'un pixel.
              over === status && dragging !== null ? 'border-brand' : 'border-transparent',
            )}
          >
            <header className="flex items-center gap-2.5 px-3 py-2">
              <div className="flex min-w-0 flex-1 items-center gap-2">
                <h2 className="truncate text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">
                  {column.label}
                </h2>
                <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-[#4770e4] text-[12px] text-white tabular-nums">
                  {items.length}
                </span>
              </div>

              {onCreate !== undefined && (
                <button
                  type="button"
                  onClick={() => setDrafting({ status, title: '' })}
                  aria-label={`Ajouter une tâche dans « ${column.label} »`}
                  className="flex shrink-0 cursor-pointer items-center justify-center text-[#73757c] transition-colors hover:text-[#1b1b1b]"
                >
                  <HugeiconsIcon icon={PlusSignIcon} size={18} strokeWidth={1.8} />
                </button>
              )}

              <span aria-hidden className="text-[#a2a3a7]">
                <HugeiconsIcon icon={MoreHorizontalIcon} size={18} strokeWidth={1.8} />
              </span>
            </header>

            {/* `min-h` et non une hauteur : une colonne vide doit rester une
                cible de depot, mais elle n'a pas a occuper l'ecran. */}
            <div className="flex min-h-[80px] flex-col gap-1">
              {drafting?.status === status && (
                <textarea
                  autoFocus
                  rows={2}
                  value={drafting.title}
                  onChange={(event) => setDrafting({ status, title: event.target.value })}
                  onBlur={submitDraft}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' && !event.shiftKey) {
                      event.preventDefault()
                      submitDraft()
                    }
                    if (event.key === 'Escape') {
                      event.stopPropagation()
                      setDrafting(null)
                    }
                  }}
                  placeholder="Intitulé de la tâche"
                  aria-label="Nouvelle tâche"
                  className="w-full resize-none rounded-[10px] border border-[#1b1b1b] bg-white p-3 text-[14px] leading-[1.5] text-[#1b1b1b] outline-none"
                />
              )}

              {/* Pas d'animation de sortie : une carte qui change de colonne
                  quitte cette liste pour l'autre, et la faire disparaitre en
                  fondu donnait l'impression qu'on venait de la perdre. */}
              {items.map((task) => (
                <TaskCard
                  key={task.id}
                  task={task}
                  today={today}
                  onOpen={() => onOpen(task.id)}
                  onMove={(next) => onMove(task, next)}
                  onGrab={() => setDragging(task.id)}
                  onCarry={(point) => {
                    const next = columnAt(point)
                    setOver((current) => (current === next ? current : next))
                  }}
                  onRelease={(point) => release(task, point)}
                />
              ))}
            </div>

            {onCreate !== undefined && (
              <button
                type="button"
                onClick={() => setDrafting({ status, title: '' })}
                className="flex cursor-pointer items-center justify-center gap-2 rounded-[12px] border border-dashed border-[#d0d1d3] bg-[#f3f4f4] px-4 py-3 text-[14px] font-medium text-[#73757c] transition-colors hover:border-[#a2a3a7] hover:text-[#1b1b1b]"
              >
                <HugeiconsIcon icon={PlusSignIcon} size={20} strokeWidth={1.8} />
                Ajouter une tâche
              </button>
            )}
          </section>
        )
      })}
    </div>
  )
}
