import {
  AddCircleIcon,
  ArrowDown01Icon,
  Calendar01Icon,
  Cancel01Icon,
  Comment01Icon,
  Delete02Icon,
  Edit02Icon,
  HourglassIcon,
  Loading03Icon,
  PlusSignIcon,
  SquareArrowUpLeftIcon,
  Tag01Icon,
  TaskDaily02Icon,
  Tick02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'framer-motion'
import * as React from 'react'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import { DashboardCard } from '@/components/dashboard-card'
import { HoverMenuContent, HoverMenuItem } from '@/components/hover-menu'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Checkbox } from '@/components/ui/checkbox'
import { DropdownMenu, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Sheet, SheetClose, SheetContent, SheetDescription, SheetTitle } from '@/components/ui/sheet'
import { peopleQuery } from '@/features/projects/api'
import { parseApiDate, tintOf } from '@/features/projects/format'
import {
  taskCommentsQuery,
  taskDetailQuery,
  useAddComment,
  useAddSubtask,
  useDeleteSubtask,
  useMoveTask,
  useSetAssignees,
  useUpdateSubtask,
  useUpdateTask,
} from '@/features/tasks/api'
import { HttpError } from '@/lib/api'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'
import type { Person, TaskActivity, TaskDetail, TaskStatus } from '@/types/api'

const STATUS_ORDER: TaskStatus[] = ['todo', 'progress', 'review', 'done']

const STATUS_LABEL: Record<TaskStatus, string> = {
  todo: 'À faire',
  progress: 'En cours',
  review: 'En revue',
  done: 'Terminé',
}

/**
 * Pastilles de statut.
 *
 * L'encre porte le drapeau important : l'entree de menu de shadcn recolore
 * tous ses descendants au survol (`focus:**:text-accent-foreground`), et son
 * selecteur l'emporte sur une simple classe. Sans cela, survoler « Terminé »
 * dans le menu en effacerait le vert — la couleur dit ici l'etat, elle ne
 * decore pas.
 */
const BADGE_TONE: Record<TaskStatus, string> = {
  todo: 'border-[#ebebeb] bg-[#f8f8f8] text-[#777]!',
  progress: 'border-[#ffd9c2] bg-[#fef3eb] text-[#c95901]!',
  review: 'border-[#f5e3a3] bg-[#fdf8e7] text-[#a16207]!',
  done: 'border-[#e1f4df] bg-[#f1faf0] text-[#4fbb46]!',
}

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})
const DATE_TIME = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})
const RELATIVE = new Intl.RelativeTimeFormat('fr-FR', { numeric: 'auto' })

/**
 * Date du jour au format « AAAA-MM-JJ ».
 *
 * Passer par `toISOString` la basculerait en UTC : a l'est de Greenwich, un
 * 12 juin choisi le matin ressortirait le 11.
 */
function toISO(date: Date) {
  return [
    date.getFullYear(),
    String(date.getMonth() + 1).padStart(2, '0'),
    String(date.getDate()).padStart(2, '0'),
  ].join('-')
}

/** Jours entiers d'ici a l'echeance, negatif une fois qu'elle est passee. */
function daysUntil(date: Date) {
  const today = new Date()
  today.setHours(0, 0, 0, 0)

  return Math.round((date.getTime() - today.getTime()) / 86_400_000)
}

/** Nom affichable d'une personne, meme quand le compte n'a pas d'etat civil. */
function nameOf(person: Person) {
  return `${person.firstname} ${person.lastname}`.trim() || 'Sans nom'
}

/** Signale l'echec d'une ecriture. Le succes, lui, se voit a l'ecran. */
function reportError(error: unknown) {
  toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible')
}

/** Bouton d'action : contour, fond blanc, halo de 2px. */
function ToolButton({ icon, children, ...props }: React.ComponentProps<'button'> & { icon: IconSvgElement; children: ReactNode }) {
  return (
    <button
      type="button"
      className="flex shrink-0 cursor-pointer items-center justify-center gap-1 overflow-clip rounded-[6px] border border-[#ebebeb] bg-white px-2 py-1 shadow-[0_1px_2px_0_rgb(16_24_40/0.05)] transition-colors hover:bg-[#f8f8f8]"
      {...props}
    >
      <HugeiconsIcon icon={icon} size={16} strokeWidth={1.6} className="text-[#777]" />
      <span className="overflow-hidden text-[14px] leading-[20px] font-medium text-ellipsis whitespace-nowrap text-[#777]">
        {children}
      </span>
    </button>
  )
}

/** Pastille d'avatar : le dessin y met une photo, on garde ses initiales. */
function Avatar({ person, size }: { person: Person; size: 20 | 32 }) {
  return (
    <span
      className="flex shrink-0 items-center justify-center rounded-full border-[0.5px] border-[#ebebeb] font-medium text-[#111]"
      style={{
        backgroundColor: tintOf(person.id),
        width: size,
        height: size,
        fontSize: size === 20 ? 8 : 11,
      }}
      title={nameOf(person)}
    >
      {person.initials}
    </span>
  )
}

/**
 * Chevron d'un champ deroulant.
 *
 * Il pivote a l'ouverture : sans cela, rien ne distingue un menu ouvert d'un
 * menu ferme une fois le regard revenu sur le champ.
 */
function Chevron({ open }: { open: boolean }) {
  const transition = useSlideTransition()

  return (
    <motion.span
      aria-hidden
      animate={{ rotate: open ? 180 : 0 }}
      transition={transition}
      className="flex shrink-0 items-center text-[#777]"
    >
      <HugeiconsIcon icon={ArrowDown01Icon} size={20} strokeWidth={1.5} />
    </motion.span>
  )
}

/** Coche de l'entree deja choisie dans un menu. */
function Tick() {
  return (
    <HugeiconsIcon
      icon={Tick02Icon}
      size={16}
      strokeWidth={2}
      className="ml-auto shrink-0 text-[#777]"
    />
  )
}

/** Intitule sur 160px, valeur sur le reste. */
function Field({
  label,
  align = 'center',
  children,
}: {
  label: string
  align?: 'center' | 'start'
  children: ReactNode
}) {
  return (
    <div className={cn('flex w-full gap-2', align === 'center' ? 'items-center' : 'items-start')}>
      <p className="w-[160px] shrink-0 overflow-hidden text-[14px] leading-[20px] font-medium text-ellipsis whitespace-nowrap text-[#777]">
        {label}
      </p>
      {children}
    </div>
  )
}

function Badge({ tone, children }: { tone: TaskStatus; children: ReactNode }) {
  return (
    <span
      className={cn(
        'flex h-[22px] shrink-0 items-center justify-center rounded-[8px] border p-[6px] text-[12px] font-semibold whitespace-nowrap',
        BADGE_TONE[tone],
      )}
    >
      {children}
    </span>
  )
}

/**
 * Cale la hauteur d'un champ de saisie sur son contenu.
 *
 * `height: auto` avant la mesure, sans quoi `scrollHeight` renvoie la hauteur
 * deja posee des que le texte raccourcit. Les bordures s'ajoutent ensuite :
 * `scrollHeight` compte le contenu et ses marges interieures, jamais les
 * filets, et sous `box-sizing: border-box` la hauteur ecrite les englobe —
 * les omettre ampute le champ de deux pixels.
 */
function autoGrow(element: HTMLTextAreaElement | null) {
  if (element === null) return

  element.style.height = 'auto'

  const styles = getComputedStyle(element)
  const borders = parseFloat(styles.borderTopWidth) + parseFloat(styles.borderBottomWidth)

  element.style.height = `${element.scrollHeight + borders}px`
}

/**
 * Une sous-tache : sa case, son libelle, ses actions.
 *
 * Cocher part immediatement ; renommer attend la sortie du champ. Enregistrer
 * a chaque frappe ferait une requete par lettre.
 */
function SubtaskRow({
  subtask,
  taskId,
}: {
  subtask: TaskDetail['subtasks'][number]
  taskId: string
}) {
  const transition = useSlideTransition()
  const update = useUpdateSubtask(taskId)
  const remove = useDeleteSubtask(taskId)

  const [editing, setEditing] = useState<string | null>(null)

  function commitRename() {
    const label = (editing ?? '').trim()

    setEditing(null)

    // Un libelle vide est refuse plutot que d'etre enregistre : une sous-tache
    // sans nom ne se distingue plus des autres.
    if (label === '' || label === subtask.label) return

    update.mutate({ id: subtask.id, label }, { onError: reportError })
  }

  return (
    <motion.div
      layout
      initial={{ opacity: 0, height: 0 }}
      animate={{ opacity: 1, height: 'auto' }}
      exit={{ opacity: 0, height: 0 }}
      transition={transition}
      className="group flex w-full items-center gap-3 overflow-clip rounded-[8px] px-3 py-2 transition-colors hover:bg-[#f8f8f8]"
    >
      <Checkbox
        checked={subtask.done}
        onCheckedChange={(next) =>
          update.mutate({ id: subtask.id, done: next === true }, { onError: reportError })
        }
        aria-label={subtask.label}
      />

      {editing === null ? (
        /* Le trait est un element a part et non un `line-through` : la
           propriete CSS n'a pas de valeur intermediaire, elle ne peut
           qu'apparaitre d'un bloc. Une regle d'un pixel mise a l'echelle
           depuis son bord gauche se deplie, elle, du debut du mot jusqu'a sa
           fin. */
        <p
          className={cn(
            'relative min-w-px flex-1 text-[14px] leading-[20px] transition-colors',
            subtask.done ? 'text-[#999]' : 'text-[#111]',
          )}
        >
          <span className="relative inline">
            {subtask.label}
            <motion.span
              aria-hidden
              className="absolute top-[calc(50%-0.5px)] left-0 h-px w-full origin-left bg-current"
              initial={false}
              animate={{ scaleX: subtask.done ? 1 : 0 }}
              transition={transition}
            />
          </span>
        </p>
      ) : (
        <input
          autoFocus
          value={editing}
          onChange={(event) => setEditing(event.target.value)}
          onFocus={(event) => event.target.select()}
          onBlur={commitRename}
          onKeyDown={(event) => {
            if (event.key === 'Enter') commitRename()

            // Sans cette interception, Echap traverserait le champ et
            // fermerait le panneau entier au lieu d'abandonner le renommage.
            if (event.key === 'Escape') {
              event.stopPropagation()
              setEditing(null)
            }
          }}
          aria-label={`Renommer « ${subtask.label} »`}
          className="min-w-px flex-1 bg-transparent text-[14px] leading-[20px] text-[#111] outline-none"
        />
      )}

      {/* En retrait jusqu'au survol. L'opacite plutot que l'affichage :
          apparaitre en poussant la ligne la ferait sauter sous le curseur. */}
      <div className="flex shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
        <button
          type="button"
          onClick={() => setEditing(subtask.label)}
          aria-label={`Renommer « ${subtask.label} »`}
          className="flex cursor-pointer items-center justify-center rounded-[6px] p-1.5 text-[#777] transition-colors hover:bg-[#ebebeb]"
        >
          <HugeiconsIcon icon={Edit02Icon} size={16} strokeWidth={1.5} />
        </button>

        <button
          type="button"
          onClick={() => remove.mutate(subtask.id, { onError: reportError })}
          aria-label={`Supprimer « ${subtask.label} »`}
          className="flex cursor-pointer items-center justify-center rounded-[6px] p-1.5 text-[#777] transition-colors hover:bg-[#fdecec] hover:text-[#e5484d]"
        >
          <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.5} />
        </button>
      </div>
    </motion.div>
  )
}

/** Une ligne du journal, mise en phrase. */
function ActivityLine({ entry }: { entry: TaskActivity }) {
  const actor = entry.actor === null || entry.actor === undefined ? null : entry.actor
  const payload = entry.payload as Record<string, unknown>

  const sentence = (() => {
    switch (entry.kind) {
      case 'created':
        return 'a créé la tâche'
      case 'status_changed':
        return `a déplacé la tâche de « ${STATUS_LABEL[payload.from as TaskStatus] ?? payload.from} » à « ${STATUS_LABEL[payload.to as TaskStatus] ?? payload.to} »`
      case 'assigned':
        return 'a affecté quelqu’un'
      case 'unassigned':
        return `a retiré ${payload.name ?? 'quelqu’un'}`
      case 'subtask_done':
        return `a coché « ${payload.label} »`
      case 'subtask_undone':
        return `a décoché « ${payload.label} »`
      case 'commented':
        return 'a commenté'
      case 'due_changed':
        return payload.to === null
          ? 'a retiré l’échéance'
          : `a fixé l’échéance au ${DATE_FORMAT.format(parseApiDate(payload.to as string) ?? new Date())}`
      default:
        return entry.kind
    }
  })()

  return (
    <li className="flex items-start gap-3">
      {actor === null ? (
        <span className="relative size-8 shrink-0">
          <span className="absolute inset-0 rounded-full border border-[#ebebeb]" />
          <HugeiconsIcon
            icon={Tag01Icon}
            size={18}
            strokeWidth={1.5}
            className="absolute top-[7px] left-[7px] text-[#777]"
          />
        </span>
      ) : (
        <Avatar person={actor} size={32} />
      )}

      <div className="flex min-w-px flex-1 flex-col gap-0.5">
        <p className="text-[13px] leading-[18px] text-[#777]">
          <span className="font-medium text-[#111]">
            {actor === null ? 'Quelqu’un' : nameOf(actor)}
          </span>{' '}
          {sentence}
        </p>
        <p className="text-[12px] text-[#999]">{DATE_TIME.format(new Date(entry.created_at))}</p>
      </div>
    </li>
  )
}

type Tab = 'subtasks' | 'comments'

/**
 * Corps du panneau.
 *
 * Chaque champ enregistre pour lui-meme : il n'y a pas de bouton
 * « Enregistrer », donc pas de travail perdu en fermant le tiroir. Les champs
 * de texte attendent la sortie du champ, les choix partent au clic.
 */
function TaskDrawerBody({ task }: { task: TaskDetail }) {
  const transition = useSlideTransition()

  const [tab, setTab] = useState<Tab>('subtasks')
  const [statusOpen, setStatusOpen] = useState(false)
  const [openDate, setOpenDate] = useState<'starts_on' | 'due_on' | null>(null)
  const [editingTitle, setEditingTitle] = useState<string | null>(null)
  const [note, setNote] = useState(task.note)
  const [editingNote, setEditingNote] = useState(false)
  const [newSubtask, setNewSubtask] = useState<string | null>(null)
  const [comment, setComment] = useState('')

  const { data: people } = useQuery(peopleQuery)
  const { data: comments } = useQuery({ ...taskCommentsQuery(task.id), enabled: tab === 'comments' })

  const update = useUpdateTask(task.id, task.project_id)
  const move = useMoveTask(task.project_id)
  const setAssignees = useSetAssignees(task.id, task.project_id)
  const addSubtask = useAddSubtask(task.id)
  const addComment = useAddComment(task.id)

  const assigned = task.assignees ?? []
  const directory = people?.items ?? []
  const available = directory.filter((person) => !assigned.some((a) => a.id === person.id))

  const due = parseApiDate(task.due_on)
  const remaining = due === null ? null : daysUntil(due)

  const subtasks = task.subtasks ?? []
  const doneCount = subtasks.filter((item) => item.done).length
  const percent = subtasks.length === 0 ? 0 : Math.round((doneCount / subtasks.length) * 100)

  function commitTitle() {
    const title = (editingTitle ?? '').trim()

    setEditingTitle(null)

    if (title === '' || title === task.title) return

    update.mutate({ title }, { onError: reportError })
  }

  function commitNote() {
    setEditingNote(false)

    if (note === task.note) return

    update.mutate({ note }, { onError: reportError })
  }

  function commitSubtask() {
    const label = (newSubtask ?? '').trim()

    setNewSubtask(null)

    if (label === '') return

    addSubtask.mutate(label, { onError: reportError })
  }

  return (
    <>
      {/* Radix nomme la boite de dialogue par ces deux noeuds et avertit en
          console s'ils manquent. Le titre vit dans la carte, on les garde donc
          pour les lecteurs d'ecran seulement. */}
      <SheetTitle className="sr-only">{task.title}</SheetTitle>
      <SheetDescription className="sr-only">
        Détail de la tâche : projet, personnes assignées, dates, statut, sous-tâches et activité.
      </SheetDescription>

      <div className="flex w-full shrink-0 items-center justify-between px-3">
        <div className="flex shrink-0 items-center gap-2">
          <Link to="/pm/projets/$id/kanban" params={{ id: task.project_id }}>
            <ToolButton icon={SquareArrowUpLeftIcon}>Ouvrir le projet</ToolButton>
          </Link>
          <span className="h-6 w-px shrink-0 bg-[#ebebeb]" />
          <p className="overflow-hidden text-[14px] leading-[20px] font-medium text-ellipsis whitespace-nowrap text-[#777]">
            {task.project_name} · {task.client_name}
          </p>
        </div>

        {/* `SheetClose` plutot qu'un `onClick` : la fermeture passe par Radix,
            qui rend le focus a l'element qui avait ouvert le tiroir. */}
        <SheetClose aria-label="Fermer" className="shrink-0 cursor-pointer">
          <HugeiconsIcon icon={Cancel01Icon} size={20} strokeWidth={1.5} className="text-[#777]" />
        </SheetClose>
      </div>

      <div className="flex min-h-0 w-full flex-1 items-start justify-end overflow-clip rounded-[16px] border border-[#ebebeb] bg-white">
        {/* Colonne de gauche : la tache et son detail. */}
        <div className="flex h-full w-[612px] shrink-0 flex-col items-center gap-4 overflow-y-auto border-r border-[#ebebeb] bg-white p-6">
          <div className="flex w-full flex-col items-center gap-2">
            {editingTitle === null ? (
              <button
                type="button"
                onClick={() => setEditingTitle(task.title)}
                className="w-full cursor-text rounded-[6px] text-left text-[20px] leading-[24px] font-semibold text-[#111] transition-colors hover:bg-[#f8f8f8]"
              >
                {task.title}
              </button>
            ) : (
              <input
                autoFocus
                value={editingTitle}
                onChange={(event) => setEditingTitle(event.target.value)}
                onBlur={commitTitle}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') commitTitle()
                  if (event.key === 'Escape') {
                    event.stopPropagation()
                    setEditingTitle(null)
                  }
                }}
                aria-label="Titre de la tâche"
                className="w-full rounded-[6px] bg-transparent text-[20px] leading-[24px] font-semibold text-[#111] outline-none"
              />
            )}

            <p className="w-full text-[14px] leading-[20px] text-[#777]">
              {task.description === ''
                ? 'Aucune description.'
                : task.description}
            </p>
          </div>

          <div className="h-px w-full shrink-0 bg-[#ebebeb]" />

          <div className="flex w-full flex-col items-center gap-4">
            <Field label="Projet">
              <Link
                to="/pm/projets/$id"
                params={{ id: task.project_id }}
                className="min-w-px flex-1 truncate rounded-[6px] px-1 py-0.5 text-[14px] leading-[20px] font-medium text-[#111] transition-colors hover:bg-[#f8f8f8]"
              >
                {task.project_name}
              </Link>
            </Field>

            <Field label="Assigné à">
              <div className="flex min-w-px flex-1 flex-wrap items-center justify-between gap-1">
                <motion.div layout className="flex flex-wrap items-center gap-1">
                  <AnimatePresence mode="popLayout" initial={false}>
                    {assigned.map((person) => (
                      <motion.span
                        key={person.id}
                        layout
                        initial={{ opacity: 0, scale: 0.85 }}
                        animate={{ opacity: 1, scale: 1 }}
                        exit={{ opacity: 0, scale: 0.85 }}
                        transition={transition}
                        className="flex shrink-0 items-center gap-1 rounded-full bg-[#f8f8f8] py-0.5 pr-2 pl-[3px]"
                      >
                        <Avatar person={person} size={20} />
                        <span className="overflow-hidden text-[12px] leading-[20px] font-medium text-ellipsis whitespace-nowrap text-[#111]">
                          {nameOf(person)}
                        </span>
                        <button
                          type="button"
                          onClick={() =>
                            setAssignees.mutate(
                              assigned.filter((a) => a.id !== person.id).map((a) => a.id),
                              { onError: reportError },
                            )
                          }
                          aria-label={`Retirer ${nameOf(person)}`}
                          className="flex shrink-0 cursor-pointer items-center justify-center rounded-full text-[#777] transition-colors hover:text-[#e5484d]"
                        >
                          <HugeiconsIcon icon={Cancel01Icon} size={12} strokeWidth={1.5} />
                        </button>
                      </motion.span>
                    ))}
                  </AnimatePresence>
                </motion.div>

                {/* Le plus disparait quand l'annuaire est epuise : un menu qui
                    s'ouvrirait vide n'apprend rien. */}
                {available.length > 0 && (
                  <DropdownMenu>
                    <DropdownMenuTrigger
                      aria-label="Ajouter une personne"
                      className="flex shrink-0 cursor-pointer items-center justify-center rounded-full p-0.5 text-[#777] transition-colors hover:bg-[#f8f8f8] hover:text-[#111] aria-expanded:bg-[#f8f8f8]"
                    >
                      <HugeiconsIcon icon={AddCircleIcon} size={18} strokeWidth={1.5} />
                    </DropdownMenuTrigger>

                    <HoverMenuContent align="end" className="min-w-[220px]">
                      {available.map((person) => (
                        <HoverMenuItem
                          key={person.id}
                          onSelect={() =>
                            setAssignees.mutate([...assigned.map((a) => a.id), person.id], {
                              onError: reportError,
                            })
                          }
                        >
                          <Avatar person={person} size={20} />
                          <span className="truncate">{nameOf(person)}</span>
                        </HoverMenuItem>
                      ))}
                    </HoverMenuContent>
                  </DropdownMenu>
                )}
              </div>
            </Field>

            {(['starts_on', 'due_on'] as const).map((key) => {
              const value = parseApiDate(task[key])

              return (
                <Field key={key} label={key === 'starts_on' ? 'Date de début' : 'Échéance'}>
                  <Popover
                    open={openDate === key}
                    onOpenChange={(open) => setOpenDate(open ? key : null)}
                  >
                    <PopoverTrigger className="flex min-w-px flex-1 cursor-pointer items-center justify-between rounded-[6px] px-1 py-0.5 transition-colors hover:bg-[#f8f8f8] aria-expanded:bg-[#f8f8f8]">
                      <span className="overflow-hidden text-[14px] leading-[20px] font-medium text-ellipsis whitespace-nowrap text-[#111]">
                        {value === null ? '—' : DATE_FORMAT.format(value)}
                      </span>
                      <HugeiconsIcon
                        icon={Calendar01Icon}
                        size={18}
                        strokeWidth={1.5}
                        className="shrink-0 text-[#777]"
                      />
                    </PopoverTrigger>

                    <PopoverContent align="start" className="w-auto p-0">
                      <Calendar
                        mode="single"
                        autoFocus
                        // Le mois affiche est celui de la date en place, non le
                        // mois courant : ouvrir sur aujourd'hui obligerait a
                        // naviguer pour retrouver ce qu'on vient de lire.
                        defaultMonth={value ?? undefined}
                        selected={value ?? undefined}
                        onSelect={(day) => {
                          if (day === undefined) return

                          update.mutate({ [key]: toISO(day) }, { onError: reportError })
                          setOpenDate(null)
                        }}
                      />
                      {value !== null && (
                        <div className="border-t border-[#ebebeb] p-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            className="w-full text-[#777]"
                            onClick={() => {
                              update.mutate({ [key]: null }, { onError: reportError })
                              setOpenDate(null)
                            }}
                          >
                            Retirer la date
                          </Button>
                        </div>
                      )}
                    </PopoverContent>
                  </Popover>
                </Field>
              )
            })}

            <Field label="Statut">
              <DropdownMenu open={statusOpen} onOpenChange={setStatusOpen}>
                <DropdownMenuTrigger className="flex min-w-px flex-1 cursor-pointer items-center justify-between rounded-[6px] px-1 py-0.5 transition-colors hover:bg-[#f8f8f8] aria-expanded:bg-[#f8f8f8]">
                  <Badge tone={task.status}>{STATUS_LABEL[task.status]}</Badge>
                  <Chevron open={statusOpen} />
                </DropdownMenuTrigger>

                <HoverMenuContent align="start" className="min-w-[180px]">
                  {STATUS_ORDER.map((entry) => (
                    <HoverMenuItem
                      key={entry}
                      onSelect={() =>
                        move.mutate({ id: task.id, status: entry }, { onError: reportError })
                      }
                    >
                      <Badge tone={entry}>{STATUS_LABEL[entry]}</Badge>
                      {entry === task.status && <Tick />}
                    </HoverMenuItem>
                  ))}
                </HoverMenuContent>
              </DropdownMenu>
            </Field>

            <Field label="Étiquette">
              <TagField task={task} />
            </Field>

            {/* Le cadre reste le meme, edite ou non : c'est deja une boite de
                saisie a l'oeil, la faire apparaitre au clic ferait sauter la
                ligne. Seule la bordure se teinte pour dire ou l'on ecrit. */}
            <Field label="Note" align="start">
              <div className="flex min-w-px flex-1 items-center">
                {editingNote ? (
                  <textarea
                    autoFocus
                    value={note}
                    rows={1}
                    ref={autoGrow}
                    onChange={(event) => {
                      setNote(event.target.value)
                      autoGrow(event.target)
                    }}
                    onBlur={commitNote}
                    onKeyDown={(event) => {
                      if (event.key === 'Escape') {
                        event.stopPropagation()
                        setNote(task.note)
                        setEditingNote(false)
                      }
                    }}
                    aria-label="Note"
                    className="min-w-px flex-1 resize-none rounded-[9px] border border-[#111] px-3 pt-2 pb-3 text-[14px] leading-[20px] text-[#111] outline-none"
                  />
                ) : (
                  <button
                    type="button"
                    onClick={() => setEditingNote(true)}
                    className="flex min-w-px flex-1 cursor-text items-center rounded-[9px] border border-[#ebebeb] px-3 pt-2 pb-3 text-left transition-colors hover:border-[#c4c4c4]"
                  >
                    <span className="min-w-px flex-1 text-[14px] leading-[20px] text-[#777]">
                      {task.note === '' ? 'Ajouter une note…' : task.note}
                    </span>
                  </button>
                )}
              </div>
            </Field>
          </div>

          <div className="h-px w-full shrink-0 bg-[#ebebeb]" />

          <div className="relative flex w-full shrink-0 items-center gap-6 border-b border-[#ebebeb] py-3.5">
            {(
              [
                { key: 'subtasks', label: 'Sous-tâches', icon: TaskDaily02Icon, count: subtasks.length },
                { key: 'comments', label: 'Commentaires', icon: Comment01Icon, count: null },
              ] as const
            ).map((entry) => {
              const active = tab === entry.key

              return (
                <button
                  key={entry.key}
                  type="button"
                  role="tab"
                  aria-selected={active}
                  onClick={() => setTab(entry.key)}
                  className="relative flex shrink-0 cursor-pointer items-center justify-center gap-1.5"
                >
                  <HugeiconsIcon
                    icon={entry.icon}
                    size={18}
                    strokeWidth={1.5}
                    className={active ? 'text-[#111]' : 'text-[#999]'}
                  />
                  <p
                    className={cn(
                      'text-center text-[14px] leading-[20px] font-medium whitespace-nowrap',
                      active ? 'text-[#111]' : 'text-[#999]',
                    )}
                  >
                    {entry.label}
                  </p>
                  {entry.count !== null && entry.count > 0 && (
                    <span className="flex shrink-0 items-center justify-center rounded-full bg-[#c4c4c4] px-1.5 text-[11px] leading-[16px] font-medium text-white">
                      {entry.count}
                    </span>
                  )}

                  {/* Un seul noeud pour les onglets : il partage le `layoutId`,
                      donc Framer le glisse de sa position precedente vers la
                      nouvelle au lieu de le faire disparaitre ici et
                      reapparaitre la. */}
                  {active && (
                    <motion.span
                      aria-hidden
                      layoutId="task-drawer-tab"
                      transition={transition}
                      className="absolute right-0 -bottom-[14px] left-0 h-[2px] bg-[#ff782b]"
                    />
                  )}
                </button>
              )
            })}
          </div>

          {tab === 'subtasks' && (
            <div className="flex w-full flex-col items-center gap-2">
              <div className="flex w-full items-center gap-2 px-1">
                <span className="flex min-w-px flex-1 items-center gap-2 text-[14px] leading-[20px] font-medium">
                  <span className="shrink-0 text-[#111]">Sous-tâches</span>
                  <span className="shrink-0 text-[#999] tabular-nums">
                    {doneCount}/{subtasks.length}
                  </span>
                </span>

                <Button
                  variant="outline"
                  onClick={() => setNewSubtask('')}
                  className="border-[#ebebeb] text-[#111] shadow-[0_1px_2px_0_rgb(16_24_40/0.05)] hover:bg-[#f8f8f8]"
                >
                  <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={1.6} />
                  Ajouter
                </Button>
              </div>

              <div className="flex w-full flex-col">
                <AnimatePresence initial={false}>
                  {subtasks.map((subtask) => (
                    <SubtaskRow key={subtask.id} subtask={subtask} taskId={task.id} />
                  ))}
                </AnimatePresence>
              </div>

              {newSubtask !== null && (
                <input
                  autoFocus
                  value={newSubtask}
                  onChange={(event) => setNewSubtask(event.target.value)}
                  onBlur={commitSubtask}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') commitSubtask()
                    if (event.key === 'Escape') {
                      event.stopPropagation()
                      setNewSubtask(null)
                    }
                  }}
                  placeholder="Intitulé de la sous-tâche"
                  aria-label="Nouvelle sous-tâche"
                  className="w-full rounded-[8px] border border-[#111] px-3 py-2 text-[14px] leading-[20px] text-[#111] outline-none"
                />
              )}

              {subtasks.length === 0 && newSubtask === null && (
                <p className="w-full py-6 text-center text-[13px] text-[#999]">
                  Aucune sous-tâche pour le moment.
                </p>
              )}
            </div>
          )}

          {tab === 'comments' && (
            <div className="flex w-full flex-col gap-3">
              <div className="flex w-full flex-col gap-2">
                <textarea
                  value={comment}
                  rows={2}
                  onChange={(event) => setComment(event.target.value)}
                  placeholder="Écrire un commentaire…"
                  aria-label="Nouveau commentaire"
                  className="w-full resize-none rounded-[9px] border border-[#ebebeb] px-3 py-2 text-[14px] leading-[20px] text-[#111] outline-none focus:border-[#c4c4c4]"
                />
                <div className="flex justify-end">
                  <Button
                    variant="outline"
                    disabled={comment.trim() === '' || addComment.isPending}
                    onClick={() =>
                      addComment.mutate(comment.trim(), {
                        onError: reportError,
                        onSuccess: () => setComment(''),
                      })
                    }
                    className="border-[#ebebeb] text-[#111] shadow-[0_1px_2px_0_rgb(16_24_40/0.05)] hover:bg-[#f8f8f8]"
                  >
                    Publier
                  </Button>
                </div>
              </div>

              <ul className="flex w-full flex-col gap-3">
                {(comments?.items ?? []).map((entry) => (
                  <li key={entry.id} className="flex items-start gap-3">
                    {entry.author === null || entry.author === undefined ? (
                      <span className="size-8 shrink-0 rounded-full border border-[#ebebeb]" />
                    ) : (
                      <Avatar person={entry.author} size={32} />
                    )}
                    <div className="flex min-w-px flex-1 flex-col gap-0.5">
                      <p className="text-[13px] font-medium text-[#111]">
                        {entry.author === null || entry.author === undefined
                          ? 'Compte supprimé'
                          : nameOf(entry.author)}
                        <span className="pl-2 text-[12px] font-normal text-[#999]">
                          {DATE_TIME.format(new Date(entry.created_at))}
                        </span>
                      </p>
                      <p className="text-[14px] leading-[20px] whitespace-pre-wrap text-[#111]">
                        {entry.body}
                      </p>
                    </div>
                  </li>
                ))}

                {(comments?.items ?? []).length === 0 && (
                  <li className="py-4 text-center text-[13px] text-[#999]">
                    Aucun commentaire pour le moment.
                  </li>
                )}
              </ul>
            </div>
          )}
        </div>

        {/* Colonne de droite : les chiffres et le journal. */}
        <div className="flex h-full w-[464px] shrink-0 flex-col items-start gap-5 overflow-y-auto bg-white p-6">
          <DashboardCard
            icon={HourglassIcon}
            title="TEMPS RESTANT"
            className="w-full shrink-0"
            action={
              <p
                className={cn(
                  'ml-auto shrink-0 text-right text-[14px] leading-[1.5] font-semibold whitespace-nowrap tabular-nums',
                  remaining !== null && remaining < 0 ? 'text-[#e5484d]' : 'text-[#111]',
                )}
              >
                {remaining === null
                  ? '—'
                  : remaining < 0
                    ? `${-remaining} j de retard`
                    : `${remaining} j`}
              </p>
            }
          >
            <p className="text-[12px] leading-[16px] text-[#777]">
              {due === null
                ? 'Aucune échéance fixée.'
                : `Échéance au ${DATE_FORMAT.format(due)}${remaining === null ? '' : ` · ${RELATIVE.format(remaining, 'day')}`}`}
            </p>
          </DashboardCard>

          <DashboardCard icon={Loading03Icon} title="AVANCEMENT" className="w-full shrink-0">
            <div className="flex w-full flex-col gap-4">
              <div className="flex w-full items-center gap-4">
                <p className="min-w-px flex-1 text-[14px] leading-[20px] text-[#777]">
                  {subtasks.length === 0 ? (
                    'Aucune sous-tâche à suivre.'
                  ) : (
                    <>
                      L’avancement est à{' '}
                      <span className="font-semibold text-[#111] tabular-nums">
                        {percent}&nbsp;%
                      </span>
                    </>
                  )}
                </p>
                <span className="flex shrink-0 items-center rounded-full bg-[#fef3eb] p-1.5">
                  <span className="size-4 overflow-clip text-[16px] leading-none">
                    {percent === 100 ? '🎉' : percent >= 50 ? '😎' : percent > 0 ? '🙂' : '😴'}
                  </span>
                </span>
              </div>

              {/* Les crans sont arrondis a plein rayon : a cette largeur ils
                  deviennent des batonnets, et le bord droit ne se confond plus
                  avec le gap qui le suit. */}
              <div className="flex h-5 w-full shrink-0 items-center gap-1">
                {Array.from({ length: 24 }, (_, index) => (
                  <motion.span
                    key={index}
                    animate={{
                      backgroundColor:
                        index < Math.round((percent / 100) * 24) ? '#ff782b' : '#ebebeb',
                    }}
                    transition={{ ...transition, delay: Math.min(index, 24) * 0.012 }}
                    className="h-full min-w-px flex-1 rounded-full"
                  />
                ))}
              </div>
            </div>
          </DashboardCard>

          <div className="h-px w-full shrink-0 bg-[#ebebeb]" />

          <div className="flex w-full shrink-0 flex-col items-start gap-4">
            <p className="text-[14px] leading-[20px] font-medium text-[#111]">Activité</p>

            <ul className="flex w-full flex-col gap-3">
              {(task.activity ?? []).map((entry) => (
                <ActivityLine key={entry.id} entry={entry} />
              ))}
            </ul>
          </div>
        </div>
      </div>
    </>
  )
}

/** Etiquette de nature : un champ texte libre, enregistre en sortie de champ. */
function TagField({ task }: { task: TaskDetail }) {
  const update = useUpdateTask(task.id, task.project_id)
  const [value, setValue] = useState<string | null>(null)

  function commit() {
    const tag = (value ?? '').trim()

    setValue(null)

    if (tag === task.tag) return

    update.mutate({ tag }, { onError: reportError })
  }

  if (value === null) {
    return (
      <button
        type="button"
        onClick={() => setValue(task.tag)}
        className="min-w-px flex-1 cursor-text rounded-[6px] px-1 py-0.5 text-left text-[14px] leading-[20px] font-medium text-[#111] transition-colors hover:bg-[#f8f8f8]"
      >
        {task.tag === '' ? <span className="text-[#999]">Ajouter une étiquette…</span> : task.tag}
      </button>
    )
  }

  return (
    <input
      autoFocus
      value={value}
      onChange={(event) => setValue(event.target.value)}
      onBlur={commit}
      onKeyDown={(event) => {
        if (event.key === 'Enter') commit()
        if (event.key === 'Escape') {
          event.stopPropagation()
          setValue(null)
        }
      }}
      aria-label="Étiquette"
      className="min-w-px flex-1 rounded-[6px] bg-transparent px-1 py-0.5 text-[14px] leading-[20px] font-medium text-[#111] outline-none"
    />
  )
}

/**
 * Tiroir de detail d'une tache.
 *
 * Il charge lui-meme ce qu'il affiche a partir de l'identifiant porte par
 * l'URL : l'ecran qui l'ouvre n'a pas a connaitre la forme d'une tache, et un
 * lien colle dans la barre d'adresse rouvre le panneau tel quel.
 *
 * Le corps porte la cle de la tache, donc il se remonte quand on en ouvre une
 * autre : sans cela, une saisie en cours passerait d'une tache a la suivante.
 */
export function TaskDrawer({ taskId, onClose }: { taskId: string | null; onClose: () => void }) {
  const { data: task } = useQuery({
    ...taskDetailQuery(taskId ?? ''),
    enabled: taskId !== null,
  })

  return (
    <Sheet open={taskId !== null} onOpenChange={(open) => !open && onClose()}>
      {/* Le Sheet pose sa propre croix en haut a droite : elle doublerait celle
          de la barre d'outils.

          Chaque cote et chaque dimension porte le drapeau important : le Sheet
          du projet les pose en `data-[side=right]:*`, dont le selecteur
          d'attribut l'emporte sur une simple classe.

          Le mouvement : une decelaration longue plutot que `ease-in-out`, a
          duree egale — le tiroir n'est pas plus lent, il freine au lieu de
          buter. La fermeture prend la courbe inverse et 150ms.  */}
      <SheetContent
        side="right"
        showCloseButton={false}
        className="top-2! right-2! bottom-2! left-auto! flex h-auto! w-[1090px]! max-w-[calc(100vw-1rem)]! flex-col items-end gap-3 rounded-[20px] border border-[#ebebeb] bg-[#f8f8f8] px-1.5 pt-4 pb-1.5 ease-[cubic-bezier(0.32,0.72,0,1)]! will-change-[transform,opacity] data-closed:duration-150! data-closed:ease-[cubic-bezier(0.4,0,1,1)]! motion-reduce:animate-none!"
      >
        {task === undefined ? (
          <>
            <SheetTitle className="sr-only">Chargement de la tâche</SheetTitle>
            <SheetDescription className="sr-only">La tâche est en cours de chargement.</SheetDescription>
            <div className="flex min-h-0 w-full flex-1 items-center justify-center rounded-[16px] border border-[#ebebeb] bg-white">
              <p className="text-[13px] text-[#999]">Chargement…</p>
            </div>
          </>
        ) : (
          <TaskDrawerBody key={task.id} task={task} />
        )}
      </SheetContent>
    </Sheet>
  )
}
