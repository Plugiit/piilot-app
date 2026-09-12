import {
  Calendar03Icon,
  CheckmarkCircle02Icon,
  Flag02Icon,
  Folder01Icon,
  UserGroupIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import type { ReactNode } from 'react'

import { TASK_STATUS, parseApiDate } from '@/features/projects/format'
import { Avatars, PriorityTag, StatusPill } from '@/features/projects/ui'
import type { BoardTask } from '@/features/tasks/board'
import { cn } from '@/lib/utils'

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

/**
 * Vue liste des taches, partagee par la fiche d'un projet et par l'ecran du
 * module.
 *
 * Un tableau construit en flex et non en `<table>` : les cellules doivent
 * pouvoir s'etirer et se replier avec la fenetre, ce qu'un tableau HTML ne
 * fait qu'au prix de largeurs fixes.
 *
 * La seule difference entre les deux ecrans est une colonne : dans une fiche
 * de projet, repeter son nom sur chaque ligne n'apprendrait rien.
 */

/**
 * Largeurs des colonnes.
 *
 * Reprises de la maquette et posees en minimums plutot qu'en tailles fixes :
 * la colonne des personnes affectees doit pouvoir s'etendre quand la fenetre
 * le permet, sans qu'aucune ne passe sous le seuil ou son contenu se replie.
 */
interface Column {
  key: string
  label: string
  icon?: IconSvgElement
  width: string
}

const TITLE: Column = { key: 'title', label: 'Libellé', width: 'min-w-[240px] flex-[2_1_240px]' }

const PROJECT: Column = {
  key: 'project',
  label: 'Projet',
  icon: Folder01Icon,
  width: 'min-w-[180px] flex-[2_1_180px]',
}

const REST: Column[] = [
  {
    key: 'assignees',
    label: 'Assigné à',
    icon: UserGroupIcon,
    width: 'min-w-[280px] flex-[3_1_280px]',
  },
  {
    key: 'status',
    label: 'Statut',
    icon: CheckmarkCircle02Icon,
    width: 'min-w-[132px] flex-[1_1_132px]',
  },
  {
    key: 'due',
    label: 'Date d’échéance',
    icon: Calendar03Icon,
    width: 'min-w-[160px] flex-[1_1_160px]',
  },
  { key: 'priority', label: 'Priorité', icon: Flag02Icon, width: 'min-w-[120px] flex-[1_1_120px]' },
]

/** Une cellule : meme gabarit dans l'en-tete et dans les rangees. */
function Cell({
  width,
  className,
  children,
}: {
  width: string
  className?: string
  children: ReactNode
}) {
  return (
    <div
      className={cn(
        'flex items-center gap-2 border-b border-[#e8e8e9] px-2.5 py-3',
        width,
        className,
      )}
    >
      {children}
    </div>
  )
}

/**
 * Case a cocher d'une rangee.
 *
 * Cocher deplace la tache en « terminé », decocher la ramene « à faire ». Ce
 * n'est pas tout a fait reversible — une tache qui etait en revue revient a
 * faire — mais c'est ce que la case promet : deux etats, pas quatre. Le
 * changement fin se fait dans le kanban.
 */
function TaskCheckbox({
  task,
  pending,
  onToggle,
}: {
  task: BoardTask
  pending: boolean
  onToggle: (done: boolean) => void
}) {
  const done = task.status === 'done'

  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={done}
      aria-label={done ? `Rouvrir ${task.title}` : `Terminer ${task.title}`}
      disabled={pending}
      onClick={() => onToggle(!done)}
      className={cn(
        'flex size-5 shrink-0 cursor-pointer items-center justify-center rounded-[6px] border transition-colors',
        done ? 'border-brand bg-brand text-white' : 'border-[#e8e8e9] bg-white hover:border-[#c9cacc]',
      )}
    >
      {done && (
        <svg viewBox="0 0 20 20" className="size-3" fill="none" aria-hidden>
          <path
            d="M4 10.5 8 14.5 16 6"
            stroke="currentColor"
            strokeWidth="2.4"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      )}
    </button>
  )
}

export function TaskTable({
  tasks,
  showProject = false,
  pending = false,
  onToggle,
  onOpen,
  empty,
}: {
  tasks: BoardTask[]
  /** Ajoute la colonne « Projet ». Hors d'une fiche de projet, elle est la
   *  seule a dire d'ou vient la ligne. */
  showProject?: boolean
  pending?: boolean
  onToggle: (task: BoardTask, done: boolean) => void
  /** Ouvre le tiroir. Absent la ou il n'y en a pas : le libelle reste alors
   *  du texte, plutot qu'un bouton qui ne mene nulle part. */
  onOpen?: (taskId: string) => void
  empty: ReactNode
}) {
  const columns = showProject ? [TITLE, PROJECT, ...REST] : [TITLE, ...REST]

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-x-auto">
      <div className="flex min-w-max flex-col">
        <div className="flex items-stretch">
          {columns.map((column) => (
            <Cell key={column.key} width={column.width} className="bg-[#f3f4f4]">
              {column.icon !== undefined && (
                <HugeiconsIcon
                  icon={column.icon}
                  size={20}
                  strokeWidth={1.6}
                  className="shrink-0 text-[#73757c]"
                />
              )}
              <span className="text-[14px] text-[#73757c]">{column.label}</span>
            </Cell>
          ))}
        </div>

        {tasks.length === 0 && (
          <p className="border-b border-[#e8e8e9] px-2.5 py-8 text-center text-[14px] text-[#73757c]">
            {empty}
          </p>
        )}

        {tasks.map((task) => {
          const status = TASK_STATUS[task.status]
          const due = parseApiDate(task.due_on)

          return (
            <div key={task.id} className="flex items-stretch bg-white">
              <Cell width={TITLE.width} className="gap-3">
                <TaskCheckbox
                  task={task}
                  pending={pending}
                  onToggle={(done) => onToggle(task, done)}
                />

                {onOpen === undefined ? (
                  <span
                    className={cn(
                      'truncate text-[14px] text-[#1b1b1b]',
                      task.status === 'done' && 'text-[#a2a3a7] line-through',
                    )}
                  >
                    {task.title}
                  </span>
                ) : (
                  <button
                    type="button"
                    onClick={() => onOpen(task.id)}
                    className={cn(
                      'min-w-0 cursor-pointer truncate text-left text-[14px] text-[#1b1b1b] hover:underline',
                      task.status === 'done' && 'text-[#a2a3a7] line-through',
                    )}
                  >
                    {task.title}
                  </button>
                )}
              </Cell>

              {showProject && (
                <Cell width={PROJECT.width}>
                  <span className="truncate text-[14px] text-[#73757c]">
                    {task.project_name ?? '—'}
                  </span>
                </Cell>
              )}

              <Cell width={REST[0]!.width} className="gap-3">
                {task.assignees.length === 0 ? (
                  <span className="text-[14px] text-[#a2a3a7]">Personne</span>
                ) : (
                  task.assignees.map((person) => (
                    <span key={person.id} className="flex shrink-0 items-center gap-2">
                      <Avatars people={[person]} max={1} size={20} />
                      <span className="text-[14px] whitespace-nowrap text-[#1b1b1b]">
                        {`${person.firstname} ${person.lastname}`.trim() || 'Sans nom'}
                      </span>
                    </span>
                  ))
                )}
              </Cell>

              <Cell width={REST[1]!.width}>
                <StatusPill label={status.label} color={status.color} pill={status.pill} />
              </Cell>

              <Cell width={REST[2]!.width}>
                <span className="text-[14px] text-[#1b1b1b]">
                  {due === null ? '—' : DATE_FORMAT.format(due)}
                </span>
              </Cell>

              <Cell width={REST[3]!.width}>
                <PriorityTag priority={task.priority} />
              </Cell>
            </div>
          )
        })}
      </div>
    </div>
  )
}
