import {
  Calendar03Icon,
  Cancel01Icon,
  Clock01Icon,
  Delete02Icon,
  Download04Icon,
  Note01Icon,
  PlusSignIcon,
  Tag01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'framer-motion'
import { useCallback, useRef, useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { FileIcon } from '@/components/file-icon'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from '@/components/ui/sheet'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { fileUrl, projectDetailQuery } from '@/features/projects/api'
import {
  PRIORITY_TONE,
  TASK_STATUS,
  TASK_STATUS_ORDER,
  parseApiDate,
} from '@/features/projects/format'
import { Avatars, PriorityTag, StatusPill } from '@/features/projects/ui'
import {
  taskCommentsQuery,
  taskDetailQuery,
  useAddComment,
  useAddSubtask,
  useDeleteSubtask,
  useDeleteTaskFile,
  useMoveTask,
  useSetAssignees,
  useUpdateSubtask,
  useUpdateTask,
  useUploadTaskFile,
} from '@/features/tasks/api'
import { HttpError } from '@/lib/api'
import { useSlideTransition } from '@/lib/motion'
import { cn, extensionOf } from '@/lib/utils'
import type {
  Attachment,
  Person,
  Subtask,
  TaskDetail,
  TaskPriority,
} from '@/types/api'

/**
 * Panneau de detail d'une tache.
 *
 * Repris du fichier de design : un panneau etroit d'une seule colonne — le
 * titre et son resume, les proprietes en lignes label/valeur, les pieces
 * jointes, puis deux onglets qui se partagent le bas.
 *
 * Ce que le design montre en lecture seule est ici cliquable. Une vignette de
 * statut, une priorite, une echeance gardent exactement leur apparence au
 * repos et ouvrent leur menu au clic : reproduire la maquette ne doit pas
 * couter la possibilite de modifier la tache depuis l'endroit ou on la lit.
 */

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

const DATE_TIME = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'short',
  hour: '2-digit',
  minute: '2-digit',
})

const SIZE_FORMAT = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 2 })

/**
 * Taille lisible.
 *
 * Ko au-dela de mille octets, Mo au-dela d'un million : le design montre
 * « 3,64 MB », donc deux decimales et la virgule francaise.
 */
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`
  if (bytes < 1024 * 1024) return `${SIZE_FORMAT.format(bytes / 1024)} Ko`

  return `${SIZE_FORMAT.format(bytes / (1024 * 1024))} Mo`
}

function toISO(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

function nameOf(person: Person): string {
  return `${person.firstname} ${person.lastname}`.trim() || 'Sans nom'
}

/** Signale l'echec d'une ecriture. Le succes, lui, se voit a l'ecran. */
function reportError(error: unknown) {
  toast.error(error instanceof HttpError ? error.message : 'Une erreur est survenue')
}

/**
 * Bouton encadre du design : fond blanc, bordure claire, et l'ombre interne
 * basse qui lui donne son relief.
 */
function FramedButton({
  radius = 8,
  className,
  children,
  ...props
}: React.ComponentProps<'button'> & { radius?: 6 | 8 }) {
  return (
    <button
      type="button"
      {...props}
      className={cn(
        'relative flex shrink-0 cursor-pointer items-center justify-center border border-[#e8e8e9] bg-white transition-colors hover:bg-[#f8f8f8] disabled:cursor-not-allowed disabled:opacity-50',
        'shadow-[inset_0px_-2px_0px_0px_rgba(0,0,0,0.05)]',
        radius === 6 && 'rounded-[6px]',
        radius === 8 && 'rounded-[8px]',
        className,
      )}
    >
      {children}
    </button>
  )
}

/** Bouton « Ajouter … » du design : icone plus, libelle 12px. */
function AddButton({ label, ...props }: React.ComponentProps<'button'> & { label: string }) {
  return (
    <FramedButton {...props} className="gap-2 self-start px-2.5 py-1.5">
      <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} className="text-[#1b1b1b]" />
      <span className="text-[12px] font-medium text-[#1b1b1b]">{label}</span>
    </FramedButton>
  )
}

/**
 * Ligne « label : valeur » du bloc des proprietes.
 *
 * La colonne des libelles a une largeur fixe : dans le design les valeurs sont
 * alignees entre elles, ce qu'un simple `gap` ne donne pas puisque les
 * libelles n'ont pas la meme longueur.
 */
function PropertyRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex w-full items-center gap-2">
      <span className="w-[96px] shrink-0 text-[14px] text-[#73757c]">{label}</span>
      {children}
    </div>
  )
}

/** Case a cocher de 20px : aplat de marque quand elle est cochee. */
function Checkbox({
  checked,
  onToggle,
  label,
  disabled,
}: {
  checked: boolean
  onToggle: () => void
  label: string
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={onToggle}
      className={cn(
        'flex size-5 shrink-0 cursor-pointer items-center justify-center rounded-[6px] border transition-colors disabled:cursor-not-allowed',
        checked ? 'border-brand bg-brand text-white' : 'border-[#e8e8e9] bg-white hover:border-[#c9cacc]',
      )}
    >
      {checked && (
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

/**
 * Gabarit commun au texte affiche et au champ qui le remplace.
 *
 * Les deux etats partagent leur typographie et leurs marges intérieures, et
 * l'edition se signale par un anneau — un `ring` est une ombre, il ne prend
 * pas de place dans le flux la ou une bordure aurait pousse le contenu d'un
 * pixel a chaque clic. Le retrait horizontal annule les marges intérieures :
 * le texte reste aligne sur le reste du panneau, au repos comme en saisie.
 */
const TITLE_FIELD =
  '-mx-2 min-w-0 flex-1 rounded-[8px] px-2 py-1 text-[20px] leading-[1.4] font-medium text-[#1b1b1b] outline-none'

const DESCRIPTION_FIELD =
  '-mx-2 w-[calc(100%+1rem)] rounded-[8px] px-2 py-1 text-[14px] leading-[1.5] text-[#73757c] outline-none'

/**
 * Titre et resume, modifiables d'un clic sur le texte lui-meme.
 *
 * Le titre tient sur une ligne et se coupe : une tache au libelle a rallonge
 * ne doit pas repousser tout le panneau vers le bas. Le resume, lui, s'etend
 * sur ce qu'il faut de lignes, et le champ qui le remplace prend exactement la
 * meme hauteur — d'ou la mesure au montage et a chaque frappe.
 *
 * Rien n'est valide par un bouton : sortir du champ enregistre, comme partout
 * ailleurs dans ce panneau. Echap remet la valeur d'origine avant de sortir,
 * ce qui revient a ne rien ecrire puisque l'enregistrement compare d'abord.
 */
function TaskHeading({ task }: { task: TaskDetail }) {
  const [editing, setEditing] = useState<'title' | 'description' | null>(null)
  const update = useUpdateTask(task.id, task.project_id)

  // Identite stable : un `ref` inline serait rappele a chaque rendu, donc a
  // chaque frappe, et le champ perdrait sa hauteur en cours de saisie.
  const sizeToContent = useCallback((element: HTMLTextAreaElement | null) => {
    if (element === null) return

    element.style.height = 'auto'
    element.style.height = `${element.scrollHeight}px`
  }, [])

  function saveTitle(next: string) {
    const title = next.trim()

    setEditing(null)

    if (title !== '' && title !== task.title) {
      update.mutate({ title }, { onError: reportError })
    }
  }

  function saveDescription(next: string) {
    const description = next.trim()

    setEditing(null)

    if (description !== task.description) {
      update.mutate({ description }, { onError: reportError })
    }
  }

  return (
    <div className="flex w-full flex-col gap-1.5">
      {editing === 'title' ? (
        <input
          autoFocus
          defaultValue={task.title}
          onBlur={(event) => saveTitle(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault()
              event.currentTarget.blur()
            }
            if (event.key === 'Escape') {
              event.stopPropagation()
              event.currentTarget.value = task.title
              event.currentTarget.blur()
            }
          }}
          aria-label="Libellé de la tâche"
          className={cn(TITLE_FIELD, 'ring-1 ring-brand')}
        />
      ) : (
        <button
          type="button"
          onClick={() => setEditing('title')}
          title={task.title}
          className={cn(TITLE_FIELD, 'truncate text-left transition-colors hover:bg-[#f3f4f4]')}
        >
          {task.title}
        </button>
      )}

      {editing === 'description' ? (
        <textarea
          autoFocus
          ref={sizeToContent}
          rows={1}
          defaultValue={task.description}
          onInput={(event) => sizeToContent(event.currentTarget)}
          onBlur={(event) => saveDescription(event.target.value)}
          onKeyDown={(event) => {
            // Entree va a la ligne : c'est un resume, pas un libelle. La
            // validation au clavier passe donc par la touche de commande.
            if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
              event.preventDefault()
              event.currentTarget.blur()
            }
            if (event.key === 'Escape') {
              event.stopPropagation()
              event.currentTarget.value = task.description
              event.currentTarget.blur()
            }
          }}
          aria-label="Description de la tâche"
          className={cn(DESCRIPTION_FIELD, 'resize-none ring-1 ring-brand')}
        />
      ) : (
        <button
          type="button"
          onClick={() => setEditing('description')}
          className={cn(
            DESCRIPTION_FIELD,
            'text-left break-words whitespace-pre-wrap transition-colors hover:bg-[#f3f4f4]',
          )}
        >
          {task.description === '' ? (
            <span className="text-[#a2a3a7]">Ajouter une description</span>
          ) : (
            task.description
          )}
        </button>
      )}
    </div>
  )
}

/** Vignette de statut, cliquable : le menu porte les quatre colonnes. */
function StatusProperty({ task }: { task: TaskDetail }) {
  const move = useMoveTask(task.project_id)
  const status = TASK_STATUS[task.status]

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`Statut : ${status.label}. Changer`}
          className="cursor-pointer rounded-full transition-opacity hover:opacity-80"
        >
          <StatusPill label={status.label} color={status.color} pill={status.pill} />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-[160px]">
        {TASK_STATUS_ORDER.map((value) => (
          <DropdownMenuItem
            key={value}
            onSelect={() => {
              if (value !== task.status) {
                move.mutate({ id: task.id, status: value }, { onError: reportError })
              }
            }}
            className="gap-2"
          >
            <span
              aria-hidden
              className="size-1.5 rounded-full"
              style={{ backgroundColor: TASK_STATUS[value].color }}
            />
            {TASK_STATUS[value].label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** Vignette de priorite, cliquable. */
function PriorityProperty({ task }: { task: TaskDetail }) {
  const update = useUpdateTask(task.id, task.project_id)

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`Priorité : ${PRIORITY_TONE[task.priority].label}. Changer`}
          className="cursor-pointer rounded-full transition-opacity hover:opacity-80"
        >
          <PriorityTag priority={task.priority} />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-[160px]">
        {(['high', 'medium', 'low'] as TaskPriority[]).map((value) => (
          <DropdownMenuItem
            key={value}
            onSelect={() => {
              if (value !== task.priority) {
                update.mutate({ priority: value }, { onError: reportError })
              }
            }}
            className="gap-2"
          >
            <span
              aria-hidden
              className="size-2 rounded-full"
              style={{ backgroundColor: PRIORITY_TONE[value].bg }}
            />
            {PRIORITY_TONE[value].label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/**
 * Personnes affectees : visage et nom, comme dans le design.
 *
 * Le menu liste l'equipe du projet — on n'affecte pas une tache a quelqu'un
 * qui n'a pas acces au projet qui la porte.
 */
function AssigneesProperty({ task }: { task: TaskDetail }) {
  const { data: project } = useQuery(projectDetailQuery(task.project_id))
  const setAssignees = useSetAssignees(task.id, task.project_id)
  const team = project?.team ?? []
  const assigned = new Set(task.assignees.map((person) => person.id))

  function toggle(id: string) {
    const next = new Set(assigned)

    if (next.has(id)) next.delete(id)
    else next.add(id)

    setAssignees.mutate([...next], { onError: reportError })
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label="Modifier les personnes affectées"
          className="flex min-w-0 cursor-pointer items-center gap-3 rounded-[6px] px-1 py-0.5 transition-colors hover:bg-[#f3f4f4]"
        >
          {task.assignees.length === 0 ? (
            <span className="text-[14px] text-[#a2a3a7]">Personne</span>
          ) : (
            task.assignees.map((person) => (
              <span key={person.id} className="flex shrink-0 items-center gap-2">
                <Avatars people={[person]} max={1} size={20} />
                <span className="text-[14px] whitespace-nowrap text-[#1b1b1b]">
                  {nameOf(person)}
                </span>
              </span>
            ))
          )}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-[220px]">
        {team.length === 0 ? (
          <p className="px-2 py-1.5 text-[13px] text-[#73757c]">
            Personne dans l’équipe du projet.
          </p>
        ) : (
          team.map((person) => (
            <DropdownMenuItem
              key={person.id}
              onSelect={(event) => {
                event.preventDefault()
                toggle(person.id)
              }}
              className="gap-2"
            >
              <Avatars people={[person]} max={1} size={20} />
              <span className="flex-1 truncate">{nameOf(person)}</span>
              {assigned.has(person.id) && <span className="text-brand">✓</span>}
            </DropdownMenuItem>
          ))
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** Une date, en lecture puis en champ au clic. */
function DateProperty({
  value,
  label,
  onChange,
}: {
  value: string | null | undefined
  label: string
  onChange: (next: string | null) => void
}) {
  const [editing, setEditing] = useState(false)
  const date = parseApiDate(value)

  if (editing) {
    return (
      <input
        type="date"
        autoFocus
        defaultValue={date === null ? '' : toISO(date)}
        onBlur={(event) => {
          setEditing(false)
          const next = event.target.value
          if (next !== (date === null ? '' : toISO(date))) onChange(next === '' ? null : next)
        }}
        onKeyDown={(event) => {
          if (event.key === 'Enter') event.currentTarget.blur()
          if (event.key === 'Escape') {
            event.stopPropagation()
            setEditing(false)
          }
        }}
        aria-label={label}
        className="rounded-[6px] ring-1 ring-brand px-1.5 py-0.5 text-[14px] text-[#1b1b1b] outline-none"
      />
    )
  }

  return (
    <button
      type="button"
      onClick={() => setEditing(true)}
      aria-label={`${label}. Modifier`}
      className="cursor-pointer rounded-[6px] px-1 py-0.5 text-[14px] text-[#1b1b1b] transition-colors hover:bg-[#f3f4f4]"
    >
      {date === null ? <span className="text-[#a2a3a7]">Aucune</span> : DATE_FORMAT.format(date)}
    </button>
  )
}

/** Un texte court, en lecture puis en champ au clic. */
function TextProperty({
  value,
  label,
  placeholder,
  onChange,
}: {
  value: string
  label: string
  placeholder: string
  onChange: (next: string) => void
}) {
  const [editing, setEditing] = useState(false)

  if (editing) {
    return (
      <input
        autoFocus
        defaultValue={value}
        onBlur={(event) => {
          setEditing(false)
          if (event.target.value !== value) onChange(event.target.value.trim())
        }}
        onKeyDown={(event) => {
          if (event.key === 'Enter') event.currentTarget.blur()
          if (event.key === 'Escape') {
            event.stopPropagation()
            setEditing(false)
          }
        }}
        aria-label={label}
        className="min-w-0 flex-1 rounded-[6px] ring-1 ring-brand px-1.5 py-0.5 text-[14px] text-[#1b1b1b] outline-none"
      />
    )
  }

  return (
    <button
      type="button"
      onClick={() => setEditing(true)}
      aria-label={`${label}. Modifier`}
      className="min-w-0 flex-1 cursor-pointer truncate rounded-[6px] px-1 py-0.5 text-left text-[14px] text-[#1b1b1b] transition-colors hover:bg-[#f3f4f4]"
    >
      {value === '' ? <span className="text-[#a2a3a7]">{placeholder}</span> : value}
    </button>
  )
}

/** Les proprietes que le design ne montre pas, et que le bouton revele. */
const EXTRA_PROPERTIES = [
  { key: 'tag', label: 'Étiquette', icon: Tag01Icon },
  { key: 'hours', label: 'Charge', icon: Clock01Icon },
  { key: 'starts_on', label: 'Début', icon: Calendar03Icon },
  { key: 'note', label: 'Note', icon: Note01Icon },
] as const

type ExtraKey = (typeof EXTRA_PROPERTIES)[number]['key']

/**
 * Bloc des proprietes.
 *
 * Les quatre du design sont toujours la. Les autres — etiquette, charge, date
 * de debut, note — n'apparaissent que si elles portent une valeur, ou si on
 * les a demandees : c'est ce que fait « Ajouter une propriété », plutot que
 * d'etaler dix lignes vides sous chaque tache.
 */
function TaskProperties({ task }: { task: TaskDetail }) {
  const update = useUpdateTask(task.id, task.project_id)
  const [revealed, setRevealed] = useState<ExtraKey[]>([])

  function filled(key: ExtraKey): boolean {
    if (key === 'tag') return task.tag !== ''
    if (key === 'note') return task.note !== ''
    if (key === 'hours') return task.hours !== null && task.hours !== undefined
    return task.starts_on !== null && task.starts_on !== undefined
  }

  const shown = EXTRA_PROPERTIES.filter((item) => filled(item.key) || revealed.includes(item.key))
  const hidden = EXTRA_PROPERTIES.filter((item) => !shown.includes(item))

  return (
    <div className="flex w-full flex-col gap-3">
      <PropertyRow label="Priorité :">
        <PriorityProperty task={task} />
      </PropertyRow>

      <PropertyRow label="Statut :">
        <StatusProperty task={task} />
      </PropertyRow>

      <PropertyRow label="Assigné à :">
        <AssigneesProperty task={task} />
      </PropertyRow>

      <PropertyRow label="Échéance :">
        <DateProperty
          value={task.due_on}
          label="Échéance"
          onChange={(next) => update.mutate({ due_on: next }, { onError: reportError })}
        />
      </PropertyRow>

      {shown.map((item) => (
        <PropertyRow key={item.key} label={`${item.label} :`}>
          {item.key === 'starts_on' && (
            <DateProperty
              value={task.starts_on}
              label="Date de début"
              onChange={(next) => update.mutate({ starts_on: next }, { onError: reportError })}
            />
          )}
          {item.key === 'tag' && (
            <TextProperty
              value={task.tag}
              label="Étiquette"
              placeholder="Aucune"
              onChange={(next) => update.mutate({ tag: next }, { onError: reportError })}
            />
          )}
          {item.key === 'note' && (
            <TextProperty
              value={task.note}
              label="Note"
              placeholder="Aucune"
              onChange={(next) => update.mutate({ note: next }, { onError: reportError })}
            />
          )}
          {item.key === 'hours' && (
            <TextProperty
              value={task.hours === null || task.hours === undefined ? '' : `${task.hours}`}
              label="Charge en heures"
              placeholder="Non estimée"
              onChange={(next) => {
                const parsed = Number(next.replace(',', '.'))
                update.mutate(
                  { hours: next === '' || Number.isNaN(parsed) ? null : parsed },
                  { onError: reportError },
                )
              }}
            />
          )}
        </PropertyRow>
      ))}

      {hidden.length > 0 && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <AddButton label="Ajouter une propriété" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="min-w-[180px]">
            {hidden.map((item) => (
              <DropdownMenuItem
                key={item.key}
                onSelect={() => setRevealed((current) => [...current, item.key])}
                className="gap-2"
              >
                <HugeiconsIcon icon={item.icon} size={16} strokeWidth={1.8} />
                {item.label}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  )
}

/** Une piece jointe : vignette, nom, type et poids, bouton de telechargement. */
function AttachmentCard({ file, onDelete }: { file: Attachment; onDelete: () => void }) {
  const extension = extensionOf(file.filename)

  return (
    <div className="group flex w-full items-center gap-2 rounded-[12px] border border-[#e8e8e9] bg-white py-2 pr-3 pl-2">
      <FileIcon filename={file.filename} size={64} className="shrink-0" />

      <div className="flex min-w-0 flex-1 flex-col justify-center gap-0.5">
        <p className="truncate text-[14px] leading-[1.5] font-medium text-[#1b1b1b]">
          {file.filename}
        </p>
        <div className="flex items-center gap-2">
          <span className="text-[14px] text-[#73757c] uppercase">{extension || 'Fichier'}</span>
          <span aria-hidden className="size-1.5 rounded-full bg-[#d0d1d3]" />
          <span className="text-[14px] text-[#73757c]">{formatSize(file.size_bytes)}</span>
        </div>
      </div>

      <button
        type="button"
        onClick={onDelete}
        aria-label={`Supprimer ${file.filename}`}
        className="shrink-0 cursor-pointer text-[#a2a3a7] opacity-0 transition-opacity group-hover:opacity-100 hover:text-[#e5484d] focus-visible:opacity-100"
      >
        <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.8} />
      </button>

      <a
        href={fileUrl(file.id)}
        download={file.filename}
        aria-label={`Télécharger ${file.filename}`}
        className="relative flex shrink-0 items-center justify-center rounded-[6px] border border-[#e8e8e9] bg-white p-1.5 shadow-[inset_0px_-2px_0px_0px_rgba(0,0,0,0.05)] transition-colors hover:bg-[#f8f8f8]"
      >
        <HugeiconsIcon
          icon={Download04Icon}
          size={18}
          strokeWidth={1.8}
          className="text-[#1b1b1b]"
        />
      </a>
    </div>
  )
}

/**
 * Pieces jointes.
 *
 * Le fichier de design montre la liste pleine, sans dire comment on depose.
 * Deux chemins, donc : le glisser-deposer, et un bouton dans l'en-tete — le
 * premier ne s'annonce pas tout seul, et reserver le second a la section vide
 * revenait a n'offrir aucune prise des qu'une piece jointe existait.
 *
 * La zone de depot s'ouvre entre le titre et la liste quand un fichier survole
 * le panneau : elle pousse le contenu vers le bas au lieu de le recouvrir, ce
 * qui laisse voir ou le fichier va atterrir.
 */
function Attachments({ task }: { task: TaskDetail }) {
  const upload = useUploadTaskFile(task.id)
  const remove = useDeleteTaskFile(task.id)
  const input = useRef<HTMLInputElement>(null)
  const [over, setOver] = useState(false)
  const transition = useSlideTransition()
  const files = task.files ?? []

  function send(list: FileList | null) {
    if (list === null) return

    for (const file of list) upload.mutate(file, { onError: reportError })
  }

  return (
    <div
      className="flex w-full flex-col gap-2"
      onDragOver={(event) => {
        event.preventDefault()
        setOver(true)
      }}
      onDragLeave={(event) => {
        // Passer d'un enfant a l'autre declenche un `dragleave` sur le parent :
        // sans ce test la zone clignoterait a chaque carte survolee.
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setOver(false)
        }
      }}
      onDrop={(event) => {
        event.preventDefault()
        setOver(false)
        send(event.dataTransfer.files)
      }}
    >
      <div className="flex w-full items-center justify-between gap-3">
        <h3 className="text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">Pièces jointes</h3>

        <div className="flex shrink-0 items-center gap-3">
          {files.length > 0 && (
            <button
              type="button"
              onClick={() => {
                // Un clic par fichier : le navigateur ne sait pas empaqueter
                // une archive, et la faire cote serveur pour deux fichiers
                // couterait plus que le geste qu'elle economise.
                for (const file of files) {
                  const link = document.createElement('a')
                  link.href = fileUrl(file.id)
                  link.download = file.filename
                  link.click()
                }
              }}
              className="flex cursor-pointer items-center gap-1 text-[14px] text-brand transition-opacity hover:opacity-80"
            >
              Tout télécharger
              <HugeiconsIcon icon={Download04Icon} size={16} strokeWidth={1.8} />
            </button>
          )}

          <button
            type="button"
            onClick={() => input.current?.click()}
            disabled={upload.isPending}
            className="flex cursor-pointer items-center gap-1 text-[14px] text-brand transition-opacity hover:opacity-80 disabled:opacity-50"
          >
            <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
            {upload.isPending ? 'Dépôt…' : 'Ajouter'}
          </button>
        </div>
      </div>

      {/* `height: auto` anime : Framer mesure la hauteur naturelle puis
          l'interpole, ce qu'une transition CSS ne sait pas faire. Le
          debordement est masque pendant le trajet, faute de quoi le contenu
          depasserait du cadre en cours d'ouverture. */}
      <AnimatePresence initial={false}>
        {over && (
          <motion.div
            key="dropzone"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={transition}
            className="overflow-hidden"
          >
            <div className="flex items-center justify-center rounded-[12px] border-2 border-dashed border-brand bg-[#fff6f1] px-4 py-6 text-[14px] font-medium text-brand">
              Déposer le fichier ici
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Une piece jointe qui arrive se deplie, celle qui part se replie.
          `marginTop` accompagne la hauteur parce que l'espacement des cartes
          vient d'un `gap` : Framer ne sait pas l'animer, et sans cette
          compensation la place d'une carte disparue resterait ouverte de huit
          pixels pendant tout le trajet.

          `initial={false}` : a l'ouverture du panneau les pieces jointes sont
          deja la, elles n'ont pas a se deplier une par une. */}
      <AnimatePresence initial={false}>
        {files.map((file) => (
          <motion.div
            key={file.id}
            initial={{ height: 0, opacity: 0, marginTop: -8 }}
            animate={{ height: 'auto', opacity: 1, marginTop: 0 }}
            exit={{ height: 0, opacity: 0, marginTop: -8 }}
            transition={transition}
            className="w-full overflow-hidden"
          >
            <AttachmentCard
              file={file}
              onDelete={() => remove.mutate(file.id, { onError: reportError })}
            />
          </motion.div>
        ))}
      </AnimatePresence>

      {files.length === 0 && !over && (
        <p className="text-[14px] text-[#73757c]">
          Aucune pièce jointe. Glissez un fichier ici, ou utilisez « Ajouter ».
        </p>
      )}

      <input
        ref={input}
        type="file"
        multiple
        hidden
        onChange={(event) => {
          send(event.target.files)
          event.target.value = ''
        }}
      />
    </div>
  )
}

/**
 * Compteur pose a droite du libelle d'un onglet.
 *
 * Un carre gris de 20px, comme dans le fichier de design : le chiffre doit se
 * lire sans concurrencer le libelle.
 */
function TabCount({ value }: { value: number }) {
  return (
    <span className="flex size-5 items-center justify-center rounded-[4px] bg-[#e8e8e9] p-0.5 text-[12px] text-[#1b1b1b] tabular-nums">
      {value}
    </span>
  )
}

/** Une ligne a cocher, renommable au clic sur son libelle. */
function SubtaskRow({ task, subtask }: { task: TaskDetail; subtask: Subtask }) {
  const update = useUpdateSubtask(task.id)
  const remove = useDeleteSubtask(task.id)
  const [editing, setEditing] = useState(false)

  return (
    <div className="group flex w-full items-center gap-3 rounded-[12px] border border-[#e8e8e9] bg-white p-3">
      <Checkbox
        checked={subtask.done}
        label={subtask.done ? `Rouvrir ${subtask.label}` : `Terminer ${subtask.label}`}
        onToggle={() =>
          update.mutate({ id: subtask.id, done: !subtask.done }, { onError: reportError })
        }
      />

      {editing ? (
        <input
          autoFocus
          defaultValue={subtask.label}
          onBlur={(event) => {
            setEditing(false)
            const next = event.target.value.trim()
            if (next !== '' && next !== subtask.label) {
              update.mutate({ id: subtask.id, label: next }, { onError: reportError })
            }
          }}
          onKeyDown={(event) => {
            if (event.key === 'Enter') event.currentTarget.blur()
            if (event.key === 'Escape') {
              event.stopPropagation()
              setEditing(false)
            }
          }}
          aria-label="Libellé de la sous-tâche"
          className="min-w-0 flex-1 rounded-[6px] ring-1 ring-brand px-1.5 py-0.5 text-[14px] font-medium text-[#1b1b1b] outline-none"
        />
      ) : (
        <button
          type="button"
          onClick={() => setEditing(true)}
          className={cn(
            'min-w-0 flex-1 cursor-pointer text-left text-[14px] leading-[1.5] font-medium break-words',
            subtask.done ? 'text-[#73757c] line-through' : 'text-[#1b1b1b]',
          )}
        >
          {subtask.label}
        </button>
      )}

      <button
        type="button"
        onClick={() => remove.mutate(subtask.id, { onError: reportError })}
        aria-label={`Supprimer ${subtask.label}`}
        className="shrink-0 cursor-pointer text-[#a2a3a7] opacity-0 transition-opacity group-hover:opacity-100 hover:text-[#e5484d] focus-visible:opacity-100"
      >
        <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.8} />
      </button>
    </div>
  )
}

/** Onglet des sous-taches : la liste, son avancement, et la ligne de saisie. */
function SubtaskPanel({ task }: { task: TaskDetail }) {
  const add = useAddSubtask(task.id)
  const [drafting, setDrafting] = useState(false)
  const [draft, setDraft] = useState('')
  const done = task.subtasks.filter((subtask) => subtask.done).length

  function submit() {
    const label = draft.trim()

    setDraft('')
    setDrafting(false)

    if (label !== '') add.mutate(label, { onError: reportError })
  }

  return (
    <div className="flex w-full flex-col gap-2">
      <div className="flex w-full items-center justify-between">
        <h3 className="text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">Sous-tâches</h3>
        <span className="text-[14px] text-[#73757c] tabular-nums">
          {done}/{task.subtasks.length}
        </span>
      </div>

      {task.subtasks.length === 0 && !drafting && (
        <p className="py-2 text-[14px] text-[#73757c]">Aucune sous-tâche.</p>
      )}

      {task.subtasks.map((subtask) => (
        <SubtaskRow key={subtask.id} task={task} subtask={subtask} />
      ))}

      {/* La ligne en cours de saisie porte le double liseré du design : un
          cadre clair autour d'un cadre plus soutenu. */}
      {drafting && (
        <div className="w-full rounded-[12px] border-2 border-[#ffd3b9]">
          <div className="flex w-full items-center gap-3 rounded-[12px] border border-[#ffa674] bg-white p-3">
            <Checkbox checked={false} label="Nouvelle sous-tâche" onToggle={() => {}} disabled />
            <input
              autoFocus
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              onBlur={submit}
              onKeyDown={(event) => {
                if (event.key === 'Enter') submit()
                if (event.key === 'Escape') {
                  event.stopPropagation()
                  setDraft('')
                  setDrafting(false)
                }
              }}
              aria-label="Nouvelle sous-tâche"
              className="min-w-0 flex-1 text-[14px] font-medium text-[#1b1b1b] outline-none"
            />
          </div>
        </div>
      )}

      <AddButton label="Ajouter une sous-tâche" onClick={() => setDrafting(true)} />
    </div>
  )
}

/** Onglet des commentaires : le fil, puis le champ d'envoi. */
function CommentPanel({ task }: { task: TaskDetail }) {
  const { data: comments } = useQuery(taskCommentsQuery(task.id))
  const add = useAddComment(task.id)
  const [body, setBody] = useState('')
  const items = comments?.items ?? []

  function submit() {
    const trimmed = body.trim()

    if (trimmed === '') return

    setBody('')
    add.mutate(trimmed, { onError: reportError })
  }

  return (
    <div className="flex w-full flex-col gap-3">
      {items.length === 0 && <p className="text-[14px] text-[#73757c]">Aucun commentaire.</p>}

      {items.map((comment) => (
        <article key={comment.id} className="flex w-full gap-2">
          {comment.author != null ? (
            <Avatars people={[comment.author]} max={1} size={28} />
          ) : (
            <span aria-hidden className="size-7 shrink-0 rounded-full bg-[#e8e8e9]" />
          )}

          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <div className="flex items-baseline gap-2">
              <span className="text-[14px] font-medium text-[#1b1b1b]">
                {comment.author == null ? 'Compte supprimé' : nameOf(comment.author)}
              </span>
              <time className="text-[12px] text-[#a2a3a7]" dateTime={comment.created_at}>
                {DATE_TIME.format(new Date(comment.created_at))}
              </time>
            </div>
            <p className="text-[14px] leading-[1.5] break-words whitespace-pre-wrap text-[#1b1b1b]">
              {comment.body}
            </p>
          </div>
        </article>
      ))}

      <div className="flex w-full flex-col gap-2">
        <textarea
          rows={2}
          value={body}
          onChange={(event) => setBody(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) submit()
          }}
          placeholder="Écrire un commentaire…"
          aria-label="Nouveau commentaire"
          className="w-full resize-none rounded-[12px] border border-[#e8e8e9] p-3 text-[14px] leading-[1.5] text-[#1b1b1b] outline-none focus:border-brand"
        />
        <FramedButton
          onClick={submit}
          disabled={body.trim() === '' || add.isPending}
          className="self-start px-3 py-1.5 text-[12px] font-medium text-[#1b1b1b]"
        >
          Publier
        </FramedButton>
      </div>
    </div>
  )
}

/**
 * Onglet a la mesure du design : libelle de 16px, filet de marque, et la
 * largeur du contenu plutot que la moitie de la barre — le composant etire ses
 * declencheurs par defaut.
 */
const TAB_CLASS =
  'h-auto flex-none gap-2 rounded-none px-3.5 py-2 text-[16px] font-normal text-[#73757c] data-active:font-medium data-active:text-brand'

/** Le contenu du panneau, une fois la tache chargee. */
function TaskDrawerBody({ task, onClose }: { task: TaskDetail; onClose: () => void }) {
  // Le compteur de l'onglet « Commentaire » se lit meme quand cet onglet est
  // ferme : Radix demonte le panneau inactif, la requete doit donc vivre ici.
  const { data: comments } = useQuery(taskCommentsQuery(task.id))

  return (
    <div className="flex h-full w-full flex-col overflow-clip rounded-[12px] border border-[#e8e8e9] bg-white">
      <header className="flex w-full shrink-0 items-center gap-2 border-b border-[#e8e8e9] py-3 pr-3 pl-4">
        <SheetTitle className="min-w-0 flex-1 text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">
          Détail de la tâche
        </SheetTitle>
        <SheetDescription className="sr-only">
          {task.title} — projet {task.project_name}.
        </SheetDescription>

        {/* `hover:shadow-none` : le variant fantome du projet pose une ombre
            interne au survol, qui redonnerait du relief a un bouton qui n'en
            veut pas. */}
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onClose}
          aria-label="Fermer"
          className="shrink-0 rounded-full text-[#1b1b1b] hover:shadow-none"
        >
          <HugeiconsIcon icon={Cancel01Icon} size={16} strokeWidth={1.8} />
        </Button>
      </header>

      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        <div className="flex w-full flex-col gap-4 p-4">
          <TaskHeading task={task} />
          <TaskProperties task={task} />
          <Attachments task={task} />
        </div>

        {/* La barre d'onglets sort de la gouttiere de 16px : son filet gris
            traverse le panneau d'un bord a l'autre, comme sur les parametres
            d'un projet. Les onglets, eux, restent alignes sur le reste du
            contenu — d'ou le decalage porte par la liste et non par le cadre.

            La variante « line » porte le filet de l'onglet actif ; le composant
            le fait glisser d'un onglet a l'autre, du meme ressort que le rail
            de modules et les onglets d'un projet. Sa couleur vient du libelle
            actif — le filet est en `bg-current`. */}
        <Tabs defaultValue="subtasks" className="w-full gap-0">
          <TabsList
            variant="line"
            indicatorClassName="bg-brand"
            className="w-full justify-start gap-0 rounded-none border-b border-[#e8e8e9] p-0 pl-4 group-data-horizontal/tabs:h-auto"
          >
            <TabsTrigger value="subtasks" className={TAB_CLASS}>
              Sous-tâche
              <TabCount value={task.subtasks.length} />
            </TabsTrigger>
            <TabsTrigger value="comments" className={TAB_CLASS}>
              Commentaire
              <TabCount value={comments?.items.length ?? 0} />
            </TabsTrigger>
          </TabsList>

          <TabsContent value="subtasks" className="p-4">
            <SubtaskPanel task={task} />
          </TabsContent>
          <TabsContent value="comments" className="p-4">
            <CommentPanel task={task} />
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}

export function TaskDrawer({ taskId, onClose }: { taskId: string | null; onClose: () => void }) {
  // La tache lue reste celle de la derniere ouverture, meme apres la fermeture.
  //
  // Sans cela le panneau se vide avant d'avoir fini de sortir : `taskId`
  // retombe a null des le clic, la requete se coupe, et le corps laisse la
  // place a l'ecran de chargement pendant que le glissement se joue encore.
  // Garder l'identifiant laisse le contenu a l'ecran jusqu'au bout.
  const [lastId, setLastId] = useState<string | null>(null)

  if (taskId !== null && taskId !== lastId) setLastId(taskId)

  const { data: task } = useQuery({
    ...taskDetailQuery(lastId ?? ''),
    enabled: lastId !== null,
  })

  return (
    <Sheet open={taskId !== null} onOpenChange={(open) => !open && onClose()}>
      {/* Le Sheet pose sa propre croix : elle doublerait celle de l'en-tete.

          Chaque cote et chaque dimension porte le drapeau important : le Sheet
          du projet les pose en `data-[side=right]:*`, dont le selecteur
          d'attribut l'emporte sur une simple classe.

          Le mouvement : un glissement franc, sans fondu, et la fermeture est
          exactement l'ouverture a l'envers — meme duree, courbe symetrique
          (l'inverse de cubic-bezier(a,b,c,d) est cubic-bezier(1-c,1-d,1-a,1-b)).

          La course est reprise ici parce que le composant la compte en
          pourcentage de la largeur, ce qui laisse depasser les 8px de marge :
          `calc(100%+0.5rem)` sort le panneau pour de bon. Le drapeau important
          est necessaire, les classes du composant portent un attribut de plus
          et gagneraient autrement. */}
      <SheetContent
        side="right"
        showCloseButton={false}
        className="top-2! right-2! bottom-2! left-auto! flex h-auto! w-[460px]! max-w-[calc(100vw-1rem)]! flex-col gap-0 rounded-[12px] border-0 bg-transparent p-0 duration-[280ms]! ease-[cubic-bezier(0.32,0.72,0,1)]! will-change-transform data-open:[--tw-enter-translate-x:calc(100%+0.5rem)]! data-closed:ease-[cubic-bezier(1,0,0.68,0.28)]! data-closed:[--tw-exit-translate-x:calc(100%+0.5rem)]! motion-reduce:animate-none!"
      >
        {task === undefined ? (
          <div className="flex h-full w-full items-center justify-center rounded-[12px] border border-[#e8e8e9] bg-white">
            <SheetTitle className="sr-only">Chargement de la tâche</SheetTitle>
            <SheetDescription className="sr-only">
              La tâche est en cours de chargement.
            </SheetDescription>
            <p className="text-[13px] text-[#73757c]">Chargement…</p>
          </div>
        ) : (
          <TaskDrawerBody key={task.id} task={task} onClose={onClose} />
        )}
      </SheetContent>
    </Sheet>
  )
}
