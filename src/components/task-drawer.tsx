import {
  AddCircleIcon,
  ArrowDown01Icon,
  ArrowExpand01Icon,
  ArrowRight02Icon,
  ArrowRight03Icon,
  ArrowUp01Icon,
  Award04Icon,
  Calendar01Icon,
  Cancel01Icon,
  Comment01Icon,
  Delete02Icon,
  DragDropVerticalIcon,
  Edit02Icon,
  HourglassIcon,
  Loading03Icon,
  MoreHorizontalIcon,
  PlusSignIcon,
  Share01Icon,
  SquareArrowUpLeftIcon,
  Tag01Icon,
  TaskDaily02Icon,
  Tick02Icon,
  ViewIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { AnimatePresence, motion, Reorder, useDragControls } from 'framer-motion'
import * as React from 'react'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import { DashboardCard } from '@/components/dashboard-card'
import { HoverMenuContent, HoverMenuItem } from '@/components/hover-menu'
import { TASK_STATUS, TASK_STATUS_ORDER, type TaskStatus } from '@/features/projects/fixtures'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Checkbox } from '@/components/ui/checkbox'
import { DropdownMenu, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from '@/components/ui/sheet'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'

/**
 * Ce qu'une tache ouvre.
 *
 * Les cinq premiers champs sont ceux de l'agenda, qui n'en sait pas plus. Les
 * suivants viennent du tableau d'un projet, qui les connait : le panneau
 * affiche ce qu'on lui donne et retombe sur ses valeurs de demonstration pour
 * le reste, plutot que de montrer des cases vides selon l'ecran d'ou l'on
 * vient.
 */
export interface TaskDetail {
  /** Identifiant porte par l'URL — c'est lui qui rouvre la tache au chargement. */
  id: string
  title: string
  category: string
  color: string
  when: string
  description?: string
  status?: TaskStatus
  /** Nom du projet portant la tache. */
  project?: string
  assignee?: { initials: string; name: string; tint: string }
  /** Echeance au format `AAAA-MM-JJ`. */
  due?: string
  hours?: number
}

/**
 * Annuaire complet : le champ Assignee y puise, et propose ce qui n'est pas
 * deja affecte. Une liste des seuls affectes ne permettrait pas d'en ajouter.
 */
const PEOPLE = [
  { name: 'Vous', initials: 'VO', tint: '#cfc3a7' },
  { name: 'Angelina J.', initials: 'AJ', tint: '#b6cdd8' },
  { name: 'Thomas F.', initials: 'TF', tint: '#c3c7df' },
  { name: 'Lena Müller', initials: 'LM', tint: '#b6cdd8' },
  { name: 'Jhon Doe', initials: 'JD', tint: '#cfc3a7' },
  { name: 'Marco B.', initials: 'MB', tint: '#d8c7b6' },
]

/** Affectations de depart — les trois du dessin. */
const ASSIGNED = ['Vous', 'Angelina J.', 'Thomas F.']

const PROJECTS = [
  { name: 'Refonte du parcours de garantie', initial: 'R', tint: '#394f6f' },
  { name: 'Fondations du design system', initial: 'F', tint: '#6f5039' },
  { name: 'Application mobile — Inscription', initial: 'A', tint: '#3f6f4f' },
]

/**
 * Statuts de la tache.
 *
 * Ils viennent du module et non du panneau : le tableau d'un projet range ses
 * colonnes avec les memes quatre etats et les memes intitules. Deux tables
 * auraient diverge des la premiere colonne renommee.
 */

/**
 * Sous-taches du dessin.
 *
 * Le dessin barre « Typography » sans le cocher et prive sa ligne de case a
 * cocher : deux details de maquette qui ne tiennent pas une fois la liste
 * vivante. Le texte barre dit « fait », donc il se deduit de la case, et une
 * ligne sur neuf qui ne se coche pas n'a aucune regle metier derriere elle.
 *
 * `checked` n'est ici que l'etat de depart — la suite vit dans le composant.
 *
 * L'identifiant est distinct du libelle parce que le libelle se renomme : une
 * case cochee retenue par son texte se decocherait au premier renommage.
 */
const SUBTASKS: {
  id: string
  label: string
  checked: boolean
  active?: boolean
}[] = [
  {
    id: 'color-palette',
    label: 'Palette de couleurs',
    checked: false,
    active: true,
  },
  { id: 'typography', label: 'Typographie', checked: false },
  { id: 'grid-system', label: 'Système de grille', checked: true },
  { id: 'shadow-effect', label: 'Ombres et effets', checked: false },
  { id: 'icons', label: 'Icônes', checked: false },
  { id: 'accordion', label: 'Accordéon', checked: true },
  { id: 'alerts', label: 'Alertes, notifications et toasts', checked: false },
  { id: 'button', label: 'Bouton', checked: false },
  { id: 'text-field', label: 'Champ de texte', checked: true },
]

const METER_TOTAL = 36

/** Bouton d'action : contour, fond blanc, halo de 2px. */
function ToolButton({
  icon,
  children,
  tone = 'sub',
}: {
  icon: IconSvgElement
  children: ReactNode
  tone?: 'sub' | 'soft'
}) {
  return (
    <button
      type="button"
      className="flex shrink-0 items-center justify-center gap-1 overflow-clip rounded-[6px] border border-[#ebebeb] bg-white px-2 py-1 shadow-[0px_0px_0px_2px_#f8f8f8]"
    >
      <HugeiconsIcon icon={icon} size={16} strokeWidth={1.5} className="text-[#777]" />
      <span
        className={cn(
          'overflow-hidden text-[14px] leading-[20px] font-medium tracking-[-0.14px] text-ellipsis whitespace-nowrap',
          tone === 'sub' ? 'text-[#777]' : 'text-[#999]',
        )}
      >
        {children}
      </span>
    </button>
  )
}

/**
 * Case a cocher du dessin.
 *
 * Un carre gris a 10% du bord, puis un carre blanc a 17,5% avec son ombre
 * portee. Cochee, le dessin la remplit et y pose une coche.
 */
/** Pastille d'avatar : le dessin y met une photo, on garde ses initiales. */
function Avatar({ initials, tint, size }: { initials: string; tint: string; size: 20 | 32 }) {
  return (
    <span
      className="flex shrink-0 items-center justify-center rounded-[80px] border-[0.5px] border-[#ebebeb] font-medium text-[#111]"
      style={{
        backgroundColor: tint,
        width: size,
        height: size,
        fontSize: size === 20 ? 8 : 11,
      }}
    >
      {initials}
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

/**
 * Cale la hauteur d'un champ de saisie sur son contenu.
 *
 * `height: auto` avant la mesure : sans cette remise a zero, `scrollHeight`
 * renvoie la hauteur deja posee des que le texte raccourcit, et le champ ne
 * sait plus que grandir.
 *
 * Les bordures s'ajoutent a la mesure : `scrollHeight` compte le contenu et
 * ses marges interieures, jamais les filets. Sous `box-sizing: border-box`,
 * que Tailwind pose partout, la hauteur ecrite les englobe pourtant — les
 * poser sans les compter ampute le champ de ses deux pixels de bordure, et
 * c'est tout ce qu'il faut pour que la ligne bouge encore au clic.
 */
function autoGrow(element: HTMLTextAreaElement | null) {
  if (element === null) return

  element.style.height = 'auto'

  const styles = getComputedStyle(element)
  const borders = parseFloat(styles.borderTopWidth) + parseFloat(styles.borderBottomWidth)

  element.style.height = `${element.scrollHeight + borders}px`
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
      <p className="w-[160px] shrink-0 overflow-hidden text-[14px] leading-[20px] font-medium tracking-[-0.14px] text-ellipsis whitespace-nowrap text-[#777]">
        {label}
      </p>
      {children}
    </div>
  )
}

const DATE_FIELDS = [
  { key: 'start', label: 'Date de début' },
  { key: 'due', label: 'Échéance' },
] as const

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', {
  month: 'long',
  day: 'numeric',
  year: 'numeric',
})

/** Minuit local, et non l'UTC que `new Date('2024-06-12')` donnerait. */
function parseDate(iso: string) {
  const date = new Date(`${iso}T00:00:00`)

  return Number.isNaN(date.getTime()) ? undefined : date
}

/**
 * Date du jour au format `AAAA-MM-JJ`.
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

function formatDate(iso: string) {
  const date = parseDate(iso)

  return date === undefined ? '—' : DATE_FORMAT.format(date)
}

/** Jours entiers d'ici a l'echeance, negatif une fois qu'elle est passee. */
function daysUntil(iso: string) {
  const today = new Date()
  today.setHours(0, 0, 0, 0)

  return Math.round((new Date(`${iso}T00:00:00`).getTime() - today.getTime()) / 86_400_000)
}

/**
 * Pastilles de statut.
 *
 * « A faire » n'est pas un evenement : il reste gris. « En cours » prend
 * l'orange de marque, seul etat qui merite d'attirer l'oeil. « Fait » reprend
 * le vert du graphique de charge — le dessin n'en proposait aucun.
 *
 * L'encre porte le drapeau important : l'entree de menu de shadcn recolore
 * tous ses descendants au survol (`focus:**:text-accent-foreground`), et son
 * selecteur l'emporte sur une simple classe. Sans cela, survoler « Fait » dans
 * le menu de statut en effacerait le vert — la couleur dit ici l'etat, elle ne
 * decore pas.
 */
const BADGE_TONE: Record<TaskStatus, string> = {
  todo: 'border-[#ebebeb] bg-[#f8f8f8] text-[#777]!',
  progress: 'border-[#ffd9c2] bg-[#fef3eb] text-[#c95901]!',
  review: 'border-[#f5e3a3] bg-[#fdf8e7] text-[#a16207]!',
  done: 'border-[#e1f4df] bg-[#f1faf0] text-[#4fbb46]!',
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

type Tab = 'subtask' | 'achievement' | 'comment'

const TABS: {
  key: Tab
  label: string
  icon: IconSvgElement
  badge?: number
}[] = [
  { key: 'subtask', label: 'Sous-tâches', icon: TaskDaily02Icon },
  { key: 'achievement', label: 'Réussites', icon: Award04Icon },
  { key: 'comment', label: 'Commentaires', icon: Comment01Icon, badge: 2 },
]

/**
 * Une sous-tache : sa case, son libelle, ses actions.
 *
 * Extraite en composant parce que `useDragControls` est un hook — il lui faut
 * une instance par ligne, ce qu'une boucle dans le parent ne peut pas donner.
 */
function SubtaskRow({
  item,
  done,
  editing,
  setEditing,
  commitRename,
  toggle,
  remove,
  transition,
}: {
  item: (typeof SUBTASKS)[number]
  done: Set<string>
  editing: { id: string; value: string } | null
  setEditing: (value: { id: string; value: string } | null) => void
  commitRename: () => void
  toggle: (id: string) => void
  remove: (id: string) => void
  transition: ReturnType<typeof useSlideTransition>
}) {
  const controls = useDragControls()

  return (
    <Reorder.Item
      as="div"
      value={item}
      // Le glissement ne part que de la poignee : sans cela toute
      // la ligne deviendrait un rail, et le clic qui coche
      // entrerait en conflit avec le geste qui deplace.
      dragListener={false}
      dragControls={controls}
      transition={transition}
      // La ligne supprimee s'efface au lieu de disparaitre d'un
      // coup : sans cela, la liste sauterait a chaque poubelle.
      initial={{ opacity: 0, height: 0 }}
      animate={{ opacity: 1, height: 'auto' }}
      exit={{ opacity: 0, height: 0 }}
      // Une ligne en cours de renommage ne se coche pas : le
      // clic y sert a placer le curseur dans le texte.
      onClick={() => editing?.id !== item.id && toggle(item.id)}
      className={cn(
        // `bg-white` et non un fond transparent : la ligne soulevee
        // passe au-dessus de ses voisines, on doit cesser de les
        // voir au travers.
        'group relative flex w-full cursor-pointer items-start overflow-clip rounded-[8px] bg-white backdrop-blur-[21px] transition-colors hover:bg-[#f8f8f8]',
        item.active
          ? 'border-2 border-white bg-[#f8f8f8] shadow-[0px_56px_88px_-8px_rgba(88,92,95,0.08)]'
          : 'drop-shadow-[0px_56px_44px_rgba(88,92,95,0.08)]',
      )}
    >
      <div
        className={cn(
          'flex h-[40px] min-w-px flex-1 items-center gap-[12px] overflow-clip pr-[20px] pl-[12px]',
          item.active ? 'bg-[#f8f8f8] py-[6px]' : 'py-[4px]',
        )}
      >
        <Checkbox
          checked={done.has(item.id)}
          onCheckedChange={() => toggle(item.id)}
          onClick={(event: React.MouseEvent) => event.stopPropagation()}
          aria-label={item.label}
        />
        {editing?.id === item.id ? (
          // Un champ sans cadre ni fond : le renommage se fait
          // sur place, la ligne ne doit pas changer d'allure
          // sous le curseur. `w-full` et non la largeur du
          // texte — le champ doit pouvoir accueillir plus long
          // que ce qu'il remplace.
          <input
            autoFocus
            value={editing.value}
            onChange={(event) => setEditing({ id: item.id, value: event.target.value })}
            onFocus={(event) => event.target.select()}
            onClick={(event) => event.stopPropagation()}
            onBlur={commitRename}
            onKeyDown={(event) => {
              if (event.key === 'Enter') commitRename()

              // Sans cette interception, Echap traverserait le
              // champ et fermerait le panneau entier au lieu
              // d'abandonner le renommage.
              if (event.key === 'Escape') {
                event.stopPropagation()
                setEditing(null)
              }
            }}
            aria-label={`Renommer « ${item.label} »`}
            className="min-w-px flex-1 bg-transparent text-[14px] leading-[20px] text-[#111] outline-none"
          />
        ) : (
          /* Le trait est un element a part et non un
                         `line-through` : la propriete CSS n'a pas de valeur
                         intermediaire, elle ne peut qu'apparaitre d'un bloc.
                         Une regle d'un pixel mise a l'echelle depuis son
                         bord gauche se deplie, elle, du debut du mot
                         jusqu'a sa fin.

                         `bg-current` la fait suivre la couleur du texte, qui
                         passe au gris dans le meme mouvement. */
          <p
            className={cn(
              'relative shrink-0 overflow-hidden text-[14px] leading-[20px] text-ellipsis whitespace-nowrap transition-colors',
              done.has(item.id) ? 'text-[#999]' : 'text-[#111]',
            )}
          >
            {item.label}

            <motion.span
              aria-hidden
              // Pas de `-translate-y-1/2` pour centrer : la
              // translation et l'echelle partagent `transform`,
              // et Framer ecraserait l'une avec l'autre. A un
              // pixel de haut, `calc` place la regle aussi bien.
              className="absolute top-[calc(50%-0.5px)] left-0 h-px w-full origin-left bg-current"
              initial={false}
              animate={{ scaleX: done.has(item.id) ? 1 : 0 }}
              transition={transition}
            />
          </p>
        )}
      </div>

      <div
        className={cn(
          'flex h-[40px] w-[164px] shrink-0 items-center overflow-clip pr-[20px] pl-[12px]',
          item.active ? 'bg-[#f8f8f8] py-[6px]' : 'py-[4px]',
        )}
      />

      {/* Le dessin ne montre ces actions que sur une ligne, la
                      sienne. Rendues partout, en retrait jusqu'au survol :
                      un renommage qui n'existerait que sur la premiere
                      sous-tache ne serait pas une fonction. L'opacite
                      plutot que l'affichage — apparaitre en poussant la
                      ligne la ferait sauter sous le curseur. */}
      <div
        onClick={(event: React.MouseEvent) => event.stopPropagation()}
        className={cn(
          'flex shrink-0 items-center justify-center gap-[4px] overflow-clip px-[8px] py-[6px] transition-opacity group-hover:opacity-100 focus-within:opacity-100',
          item.active ? 'bg-[#f8f8f8] opacity-100' : 'opacity-0',
        )}
      >
        <button
          type="button"
          onClick={() => setEditing({ id: item.id, value: item.label })}
          aria-label={`Renommer « ${item.label} »`}
          className="flex shrink-0 cursor-pointer items-center justify-center overflow-clip rounded-[8px] p-[6px] transition-colors hover:bg-[#ebebeb]"
        >
          <HugeiconsIcon icon={Edit02Icon} size={16} strokeWidth={1.5} className="text-[#777]" />
        </button>

        <button
          type="button"
          onClick={() => remove(item.id)}
          aria-label={`Supprimer « ${item.label} »`}
          className="flex shrink-0 cursor-pointer items-center justify-center overflow-clip rounded-[8px] p-[6px] text-[#777] transition-colors hover:bg-[#fdecec] hover:text-[#e5484d]"
        >
          <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.5} />
        </button>

        {/* `onPointerDown` et non `onClick` : le glissement demarre
                        au moment ou le doigt se pose, pas au relachement —
                        attendre le clic reviendrait a ne jamais partir. */}
        <span
          role="button"
          aria-label={`Déplacer « ${item.label} »`}
          onPointerDown={(event) => controls.start(event)}
          className="flex shrink-0 cursor-grab items-center justify-center overflow-clip rounded-[8px] p-[6px] transition-colors select-none hover:bg-[#ebebeb] active:cursor-grabbing"
        >
          <HugeiconsIcon
            icon={DragDropVerticalIcon}
            size={16}
            strokeWidth={1.5}
            className="text-[#777]"
          />
        </span>
      </div>
    </Reorder.Item>
  )
}

/**
 * Tiroir de detail d'une tache.
 *
 * Le panneau garde la derniere tache affichee le temps de sa sortie : vider
 * son contenu des que `task` passe a null le viderait sous les yeux, juste
 * avant qu'il disparaisse.
 *
 * Le corps porte la cle de la tache, donc il se remonte quand on en ouvre une
 * autre. Sans cela, ce qu'on a touche dans le panneau — statut, echeance,
 * sous-taches cochees — survivrait d'une tache a la suivante, et la seconde
 * s'ouvrirait avec les modifications de la premiere.
 */
export function TaskDrawer({ task, onClose }: { task: TaskDetail | null; onClose: () => void }) {
  const [shown, setShown] = useState(task)

  // Ajustement d'etat pendant le rendu, et non dans un effet : React reprend
  // aussitot avec la nouvelle valeur, sans laisser passer une frame ou le
  // panneau montrerait encore la tache precedente.
  if (task !== null && task.id !== shown?.id) setShown(task)

  return (
    <Sheet open={task !== null} onOpenChange={(open) => !open && onClose()}>
      {/* Le Sheet pose sa propre croix en haut a droite : elle doublerait celle
          de la barre d'outils, et tomberait juste sur le partage.

          `max-w` porte le drapeau important — `data-[side=right]:sm:max-w-sm`
          du composant a un selecteur d'attribut, donc plus specifique que la
          classe posee ici. */}
      <SheetContent
        side="right"
        showCloseButton={false}
        // Chaque cote et chaque dimension porte le drapeau important : le Sheet
        // du projet les pose en `data-[side=right]:*`, dont le selecteur
        // d'attribut l'emporte sur une simple classe. Sans cela il garde ses
        // `w-3/4`, `h-full` et `right-0`, et le tiroir couvre l'ecran.
        //
        // 1090px = les deux colonnes du dessin (612 + 464) plus les 6px de
        // marge de chaque cote et le filet : la largeur suit le contenu.
        //
        // Le mouvement, ensuite. Le tiroir entre en `ease-in-out` sur 200ms :
        // une courbe symetrique demarre mou et s'arrete net, et sur un panneau
        // qui traverse une centaine de pixels, l'arret se voit. Elle cede la
        // place a une decelaration longue — vive au depart, posee a l'arrivee
        // — a duree egale : le tiroir n'est pas plus lent, il freine au lieu
        // de buter.
        //
        // La fermeture prend la courbe inverse et 150ms : on regarde une
        // ouverture, on ne regarde pas une fermeture.
        //
        // La distance ne bouge pas. `slide-in-from-right-10` vaut 10% de la
        // largeur, pas 40px : le trajet etait deja le bon, c'est la facon de
        // le parcourir qui ne l'etait pas. Rien non plus du cote de l'echelle
        // — une seconde transformation par-dessus la translation donne deux
        // mouvements a suivre au lieu d'un.
        //
        // `will-change` prend les devants sur un panneau de cette taille pose
        // devant un fond floute, et `motion-reduce` coupe l'animation comme le
        // fait `useSlideTransition` pour tout le reste de l'ecran — les
        // animations CSS, elles, n'ecoutent rien par defaut.
        className="top-2! right-2! bottom-2! left-auto! flex h-auto! w-[1090px]! max-w-[calc(100vw-1rem)]! flex-col items-end gap-[12px] rounded-[20px] border border-[#ebebeb] bg-[#f8f8f8] px-[6px] pt-[16px] pb-[6px] ease-[cubic-bezier(0.32,0.72,0,1)]! will-change-[transform,opacity] data-closed:duration-150! data-closed:ease-[cubic-bezier(0.4,0,1,1)]! motion-reduce:animate-none!"
      >
        {shown !== null && <TaskDrawerBody key={shown.id} task={shown} />}
      </SheetContent>
    </Sheet>
  )
}

function TaskDrawerBody({ task }: { task: TaskDetail }) {
  const [tab, setTab] = useState<Tab>('subtask')

  // Les libelles se renomment, donc la liste devient un etat. Les cases, elles,
  // ne retiennent que des identifiants : un ensemble suffit, seule
  // l'appartenance nous interesse.
  const [subtasks, setSubtasks] = useState(SUBTASKS)
  const [done, setDone] = useState(
    () => new Set(SUBTASKS.filter((item) => item.checked).map((item) => item.id)),
  )

  /** Sous-tache en cours de renommage, avec sa saisie. */
  const [editing, setEditing] = useState<{ id: string; value: string } | null>(null)

  // Le projet de la tache ouverte remplace la liste de demonstration : sans
  // lui, le champ afficherait le premier projet du dessin a cote d'une tache
  // qui appartient a un autre.
  const projects =
    task.project === undefined
      ? PROJECTS
      : [{ name: task.project, initial: task.project.slice(0, 1).toUpperCase(), tint: '#394f6f' }]

  // Meme principe pour l'annuaire : la personne affectee doit y figurer avant
  // que le champ ne cherche a l'afficher.
  const directory = task.assignee === undefined ? PEOPLE : [task.assignee, ...PEOPLE]

  const [project, setProject] = useState(projects[0]!)
  const [assigned, setAssigned] = useState<string[]>(
    task.assignee === undefined ? ASSIGNED : [task.assignee.name],
  )
  const [status, setStatus] = useState<TaskStatus>(task.status ?? 'todo')

  // L'ouverture des menus est suivie pour que leur chevron pivote : Radix la
  // porte sur le declencheur en `aria-expanded`, mais Framer a besoin d'une
  // valeur a interpoler.
  const [projectOpen, setProjectOpen] = useState(false)
  const [statusOpen, setStatusOpen] = useState(false)

  const [dates, setDates] = useState({
    start: '2024-06-12',
    due: task.due ?? '2024-06-12',
  })
  const [openDate, setOpenDate] = useState<'start' | 'due' | null>(null)

  const [note, setNote] = useState(
    'Search for inspiration to provide a rich content of text inputs for the design system.',
  )
  const [editingNote, setEditingNote] = useState(false)

  const [remind, setRemind] = useState(true)
  const [openSubtasks, setOpenSubtasks] = useState(true)

  // La progression se deduit des cases : deux chiffres qui se contredisent —
  // « 68 % » fige d'un cote, des cases vivantes de l'autre — sont pires que
  // pas de chiffre du tout.
  const ratio = subtasks.length === 0 ? 0 : done.size / subtasks.length
  const percent = Math.round(ratio * 100)
  const filled = Math.round(ratio * METER_TOTAL)
  const remaining = daysUntil(dates.due)

  const assignees = directory.filter((person) => assigned.includes(person.name))
  const available = directory.filter((person) => !assigned.includes(person.name))

  function toggle(id: string) {
    setDone((previous) => {
      const next = new Set(previous)

      if (!next.delete(id)) next.add(id)

      return next
    })
  }

  /**
   * Ajoute une sous-tache et ouvre aussitot son renommage.
   *
   * Elle nait sans nom : demander un titre dans une fenetre avant de creer la
   * ligne ferait deux gestes la ou le champ, deja sur place, en demande un.
   */
  function add() {
    const id = `subtask-${Date.now()}`

    setSubtasks((previous) => [...previous, { id, label: 'Nouvelle sous-tâche', checked: false }])
    setOpenSubtasks(true)
    setEditing({ id, value: '' })
  }

  /**
   * Retire une sous-tache, avec de quoi revenir en arriere.
   *
   * Pas de fenetre de confirmation : elle arreterait le geste a chaque fois, y
   * compris les neuf fois sur dix ou il est volontaire. Le rappel avec son
   * « Annuler » ne coute rien quand la suppression etait voulue, et repose la
   * ligne a sa place quand elle ne l'etait pas — d'ou l'index garde, sans quoi
   * elle reviendrait en fin de liste.
   */
  function remove(id: string) {
    const index = subtasks.findIndex((item) => item.id === id)
    const removed = subtasks[index]

    if (removed === undefined) return

    const wasDone = done.has(id)

    setSubtasks((previous) => previous.filter((item) => item.id !== id))
    setDone((previous) => {
      const next = new Set(previous)
      next.delete(id)

      return next
    })

    // Renommer une ligne puis la supprimer laisserait un champ ouvert sur une
    // sous-tache qui n'existe plus.
    if (editing?.id === id) setEditing(null)

    toast(`« ${removed.label} » supprimée`, {
      action: {
        label: 'Annuler',
        onClick: () => {
          setSubtasks((previous) => {
            const next = [...previous]
            next.splice(index, 0, removed)

            return next
          })

          if (wasDone) setDone((previous) => new Set(previous).add(id))
        },
      },
    })
  }

  /**
   * Valide le renommage.
   *
   * Un libelle vide est refuse plutot que d'etre enregistre : une sous-tache
   * sans nom ne se distingue plus des autres, et rien dans l'ecran ne
   * permettrait de la retrouver pour la corriger.
   */
  function commitRename() {
    if (editing === null) return

    const label = editing.value.trim()

    if (label !== '') {
      setSubtasks((previous) =>
        previous.map((item) => (item.id === editing.id ? { ...item, label } : item)),
      )
    }

    setEditing(null)
  }

  // Le meme ressort que les onglets du graphique de charge et les menus de la
  // liste des projets : deux mouvements voisins d'allures differentes se
  // remarquent bien plus que deux couleurs voisines.
  const transition = useSlideTransition()

  return (
    <>
      {/* Radix nomme la boite de dialogue par ces deux noeuds et avertit en
          console s'ils manquent. Le dessin porte son titre dans la carte, on
          les garde donc pour les lecteurs d'ecran seulement. */}
      <SheetTitle className="sr-only">{task.title}</SheetTitle>
      <SheetDescription className="sr-only">
        Détail de la tâche : projet, personnes assignées, dates, statut, sous-tâches et activité.
      </SheetDescription>

      <div className="flex w-full shrink-0 items-center justify-between px-[12px]">
        <div className="flex shrink-0 items-center gap-[8px]">
          <HugeiconsIcon
            icon={ArrowRight03Icon}
            size={24}
            strokeWidth={1.5}
            className="shrink-0 text-[#777]"
          />
          <ToolButton icon={ArrowExpand01Icon}>Ouvrir en grand</ToolButton>
          <ToolButton icon={SquareArrowUpLeftIcon}>Ouvrir le projet</ToolButton>
          <span className="h-[24px] w-px shrink-0 bg-[#ebebeb]" />
          <p className="overflow-hidden text-[14px] leading-[20px] font-medium tracking-[-0.14px] text-ellipsis whitespace-nowrap text-[#777]">
            Détail de la tâche
          </p>
        </div>

        {/* Le dessin fige ce groupe a 504,5px : c'est lui qui pousse les
            quatre actions contre le bord droit. */}
        <div className="flex w-[504.5px] max-w-full shrink-0 items-center justify-end gap-[8px]">
          <HugeiconsIcon
            icon={Share01Icon}
            size={24}
            strokeWidth={1.5}
            className="shrink-0 text-[#777]"
          />
          <ToolButton icon={ViewIcon} tone="soft">
            10
          </ToolButton>
          <HugeiconsIcon
            icon={MoreHorizontalIcon}
            size={20}
            strokeWidth={1.5}
            className="shrink-0 text-[#777]"
          />
          {/* `SheetClose` plutot qu'un `onClick` : la fermeture passe par
              Radix, qui rend le focus a l'element qui avait ouvert le
              tiroir. */}
          <SheetClose aria-label="Fermer" className="shrink-0">
            <HugeiconsIcon
              icon={Cancel01Icon}
              size={20}
              strokeWidth={1.5}
              className="text-[#777]"
            />
          </SheetClose>
        </div>
      </div>

      {/* Le dessin fige la carte a 950px ; ici elle prend la hauteur laissee
          par la barre d'outils, sans quoi le tiroir montrerait du fond gris
          en dessous sur un ecran haut. */}
      <div className="flex min-h-0 w-full flex-1 items-start justify-end overflow-clip rounded-[16px] border border-[#ebebeb] bg-white">
        {/* Colonne de gauche : 612px au dessin. */}
        <div className="flex h-full w-[612px] shrink-0 flex-col items-center gap-[16px] overflow-y-auto border-r border-[#ebebeb] bg-white p-[24px]">
          <div className="flex w-full flex-col items-center gap-[8px]">
            {/* Plus de hauteur figee a 24px : un titre de deux lignes s'y
                serait fait couper. */}
            <p className="w-full text-[20px] leading-[24px] font-semibold text-[#111]">
              {task.title}
            </p>
            <p className="w-full text-[14px] leading-[20px] tracking-[-0.14px] text-[#777]">
              {task.description ??
                'Aucune description pour cette tâche. Le modèle de données du module dira ce qu’elle porte ; en attendant, le panneau montre ce que l’écran d’origine sait d’elle.'}
            </p>
          </div>

          <div className="h-px w-full shrink-0 bg-[#ebebeb]" />

          <div className="flex w-full flex-col items-center gap-[16px]">
            {/* Le chevron du dessin annoncait un menu qui n'existait pas.
                La ligne entiere l'ouvre : viser une fleche de 20px quand tout
                ce qui la precede designe la meme chose est une contrainte
                sans raison. */}
            <Field label="Projet">
              <DropdownMenu open={projectOpen} onOpenChange={setProjectOpen}>
                <DropdownMenuTrigger className="flex min-w-px flex-1 cursor-pointer items-center gap-[6px] rounded-[6px] px-[4px] py-[2px] text-left transition-colors hover:bg-[#f8f8f8] aria-expanded:bg-[#f8f8f8]">
                  <span
                    className="relative size-[22px] shrink-0 rounded-[4.4px]"
                    style={{ backgroundColor: project.tint }}
                  >
                    <span className="absolute inset-0 flex items-center justify-center text-[11px] font-bold text-white">
                      {project.initial}
                    </span>
                  </span>
                  <p className="min-w-px flex-1 overflow-hidden text-[14px] leading-[20px] font-medium tracking-[-0.14px] text-ellipsis whitespace-nowrap text-[#111]">
                    {project.name}
                  </p>
                  <Chevron open={projectOpen} />
                </DropdownMenuTrigger>

                <HoverMenuContent align="start" className="min-w-[280px]">
                  {projects.map((entry) => (
                    <HoverMenuItem key={entry.name} onSelect={() => setProject(entry)}>
                      <span
                        className="relative size-[18px] shrink-0 rounded-[4px]"
                        style={{ backgroundColor: entry.tint }}
                      >
                        <span className="absolute inset-0 flex items-center justify-center text-[9px] font-bold text-white">
                          {entry.initial}
                        </span>
                      </span>
                      <span className="truncate">{entry.name}</span>
                      {entry.name === project.name && <Tick />}
                    </HoverMenuItem>
                  ))}
                </HoverMenuContent>
              </DropdownMenu>
            </Field>

            <Field label="Assigné à">
              <div className="flex min-w-px flex-1 flex-wrap items-center justify-between gap-[4px]">
                {/* `layout` sur chaque puce : retirer celle du milieu doit
                    faire glisser les suivantes a sa place, pas les faire
                    sauter. `popLayout` laisse la sortante quitter le flux
                    pendant que les autres se replacent. */}
                <motion.div layout className="flex flex-wrap items-center gap-[4px]">
                  <AnimatePresence mode="popLayout" initial={false}>
                    {assignees.map((person) => (
                      <motion.span
                        key={person.name}
                        layout
                        initial={{ opacity: 0, scale: 0.85 }}
                        animate={{ opacity: 1, scale: 1 }}
                        exit={{ opacity: 0, scale: 0.85 }}
                        transition={transition}
                        className="group/chip flex shrink-0 items-center gap-[4px] rounded-[24px] bg-[#f8f8f8] py-[2px] pr-[8px] pl-[3px]"
                      >
                        <Avatar initials={person.initials} tint={person.tint} size={20} />
                        <span className="overflow-hidden text-[12px] leading-[20px] font-medium tracking-[-0.12px] text-ellipsis whitespace-nowrap text-[#111]">
                          {person.name}
                        </span>
                        <button
                          type="button"
                          onClick={() =>
                            setAssigned((previous) =>
                              previous.filter((name) => name !== person.name),
                            )
                          }
                          aria-label={`Retirer ${person.name}`}
                          className="flex shrink-0 cursor-pointer items-center justify-center rounded-full text-[#777] transition-colors hover:text-[#e5484d]"
                        >
                          <HugeiconsIcon icon={Cancel01Icon} size={12} strokeWidth={1.5} />
                        </button>
                      </motion.span>
                    ))}
                  </AnimatePresence>
                </motion.div>

                {/* Le plus disparait quand l'annuaire est epuise : un menu
                    qui s'ouvrirait vide n'apprend rien. */}
                {available.length > 0 && (
                  <DropdownMenu>
                    <DropdownMenuTrigger
                      aria-label="Ajouter une personne"
                      className="flex shrink-0 cursor-pointer items-center justify-center rounded-full p-[2px] text-[#777] transition-colors hover:bg-[#f8f8f8] hover:text-[#111] aria-expanded:bg-[#f8f8f8]"
                    >
                      <HugeiconsIcon icon={AddCircleIcon} size={18} strokeWidth={1.5} />
                    </DropdownMenuTrigger>

                    <HoverMenuContent align="end" className="min-w-[200px]">
                      {available.map((person) => (
                        <HoverMenuItem
                          key={person.name}
                          onSelect={() => setAssigned((previous) => [...previous, person.name])}
                        >
                          <Avatar initials={person.initials} tint={person.tint} size={20} />
                          <span className="truncate">{person.name}</span>
                        </HoverMenuItem>
                      ))}
                    </HoverMenuContent>
                  </DropdownMenu>
                )}
              </div>
            </Field>

            {/* Le calendrier de shadcn dans un `Popover`, ouvert par l'icone
                que le dessin posait sans lui donner de role.

                L'etat d'ouverture est suivi pour que le champ se distingue
                pendant que sa popup est la, et pour la refermer des qu'un
                jour est choisi : un calendrier qui reste ouvert apres le clic
                oblige a un second geste pour rien. */}
            {DATE_FIELDS.map((entry) => (
              <Field key={entry.key} label={entry.label}>
                <Popover
                  open={openDate === entry.key}
                  onOpenChange={(open) => setOpenDate(open ? entry.key : null)}
                >
                  <PopoverTrigger className="flex min-w-px flex-1 cursor-pointer items-center justify-between rounded-[6px] px-[4px] py-[2px] transition-colors hover:bg-[#f8f8f8] aria-expanded:bg-[#f8f8f8]">
                    <span className="overflow-hidden text-[14px] leading-[20px] font-medium tracking-[-0.14px] text-ellipsis whitespace-nowrap text-[#111]">
                      {formatDate(dates[entry.key])}
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
                      defaultMonth={parseDate(dates[entry.key])}
                      selected={parseDate(dates[entry.key])}
                      onSelect={(day) => {
                        if (day === undefined) return

                        setDates((previous) => ({
                          ...previous,
                          [entry.key]: toISO(day),
                        }))
                        setOpenDate(null)
                      }}
                    />
                  </PopoverContent>
                </Popover>
              </Field>
            ))}

            <Field label="Statut">
              <DropdownMenu open={statusOpen} onOpenChange={setStatusOpen}>
                <DropdownMenuTrigger className="flex min-w-px flex-1 cursor-pointer items-center justify-between rounded-[6px] px-[4px] py-[2px] transition-colors hover:bg-[#f8f8f8] aria-expanded:bg-[#f8f8f8]">
                  <Badge tone={status}>{TASK_STATUS[status].label}</Badge>
                  <Chevron open={statusOpen} />
                </DropdownMenuTrigger>

                <HoverMenuContent align="start" className="min-w-[180px]">
                  {TASK_STATUS_ORDER.map((entry) => (
                    <HoverMenuItem key={entry} onSelect={() => setStatus(entry)}>
                      <Badge tone={entry}>{TASK_STATUS[entry].label}</Badge>
                      {entry === status && <Tick />}
                    </HoverMenuItem>
                  ))}
                </HoverMenuContent>
              </DropdownMenu>
            </Field>

            {/* Le cadre reste le meme, edite ou non : c'est deja une boite
                de saisie a l'oeil, la faire apparaitre au clic ferait sauter
                la ligne. Seule la bordure se teinte pour dire ou l'on
                ecrit.

                Encore faut-il que la hauteur suive : `rows={3}` ouvrait le
                champ sur trois lignes quand la note en occupait deux, et
                poussait de 20px tout ce qui se trouvait dessous. Le champ se
                cale donc sur son contenu — paddings, graisse et interligne
                identiques des deux cotes, la boite ne bouge plus d'un pixel
                au passage en edition. Elle ne grandit qu'a la ligne
                suivante, quand on la tape. */}
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
                    onBlur={() => setEditingNote(false)}
                    onKeyDown={(event) => {
                      if (event.key === 'Escape') {
                        event.stopPropagation()
                        setEditingNote(false)
                      }
                    }}
                    aria-label="Note"
                    className="min-w-px flex-1 resize-none rounded-[9px] border border-[#111] px-[12px] pt-[8px] pb-[12px] text-[14px] leading-[20px] tracking-[-0.084px] text-[#111] outline-none"
                  />
                ) : (
                  <button
                    type="button"
                    onClick={() => setEditingNote(true)}
                    className="flex min-w-px flex-1 cursor-text items-center rounded-[9px] border border-[#ebebeb] px-[12px] pt-[8px] pb-[12px] text-left transition-colors hover:border-[#c4c4c4]"
                  >
                    <span className="min-w-px flex-1 text-[14px] leading-[20px] tracking-[-0.084px] text-[#777]">
                      {note === '' ? 'Ajouter une note…' : note}
                    </span>
                  </button>
                )}
              </div>
            </Field>
          </div>

          <div className="h-px w-full shrink-0 bg-[#ebebeb]" />

          <div className="relative flex w-[564px] shrink-0 items-center gap-[24px] border-b border-[#ebebeb] bg-white py-[14px]">
            {TABS.map((entry) => {
              const active = tab === entry.key

              return (
                <button
                  key={entry.key}
                  type="button"
                  role="tab"
                  aria-selected={active}
                  onClick={() => setTab(entry.key)}
                  className="relative flex shrink-0 cursor-pointer items-center justify-center gap-[6px]"
                >
                  <HugeiconsIcon
                    icon={entry.icon}
                    size={18}
                    strokeWidth={1.5}
                    className={active ? 'text-[#111]' : 'text-[#999]'}
                  />
                  <div className="flex shrink-0 items-center justify-center gap-[4px]">
                    <p
                      className={cn(
                        'text-center text-[14px] leading-[20px] font-medium tracking-[-0.084px] whitespace-nowrap',
                        active ? 'text-[#111]' : 'text-[#999]',
                      )}
                    >
                      {entry.label}
                    </p>
                    {entry.badge && (
                      <span className="flex shrink-0 items-center justify-center overflow-clip rounded-[999px] bg-[#c4c4c4] p-[2px]">
                        <span className="w-[12px] text-center text-[11px] leading-[12px] font-medium tracking-[0.22px] text-white uppercase">
                          {entry.badge}
                        </span>
                      </span>
                    )}
                  </div>
                  {/* Un seul noeud pour les trois onglets : il partage le
                    `layoutId`, donc Framer Motion le glisse de sa position
                    precedente vers la nouvelle au lieu de le faire
                    disparaitre ici et reapparaitre la. Sa couleur tient a une
                    classe CSS et non a `animate` — appliquee au montage
                    seulement, elle laisserait passer un premier frame sans
                    filet. */}
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

            <span className="absolute top-[calc(50%+0.5px)] right-[12px] flex -translate-y-1/2 items-center justify-center overflow-clip rounded-[999px] border border-[#ebebeb] bg-white p-[2px] shadow-[0_1px_2px_0_rgb(16_24_40/0.05)]">
              <HugeiconsIcon
                icon={ArrowUp01Icon}
                size={20}
                strokeWidth={1.5}
                className="text-[#777]"
              />
            </span>
          </div>

          {/* Un seul panneau est ecrit : les deux autres attendent leur
              modele de donnees, mais leur onglet doit repondre au clic —
              un onglet qui ne change rien n'est pas un onglet. */}
          {tab === 'subtask' && (
            <div className="flex w-full flex-col items-center gap-[8px]">
              <div className="flex w-full flex-col items-start">
                <div className="flex w-full items-center gap-[6px] rounded-[8px] py-[4px]">
                  {/* Le chevron du dessin pointait vers le bas sans rien
                      ouvrir ni fermer. Il replie la liste, et le compteur
                      suit les cases plutot que d'annoncer un 12/25 que rien
                      ne produit. */}
                  <button
                    type="button"
                    onClick={() => setOpenSubtasks((previous) => !previous)}
                    aria-expanded={openSubtasks}
                    aria-label={
                      openSubtasks ? 'Replier les sous-tâches' : 'Déplier les sous-tâches'
                    }
                    className="flex min-w-px flex-1 cursor-pointer items-center gap-[6px] rounded-[6px] px-[2px] py-[2px] transition-colors hover:bg-[#f8f8f8]"
                  >
                    <motion.span
                      aria-hidden
                      animate={{ rotate: openSubtasks ? 0 : -90 }}
                      transition={transition}
                      className="flex shrink-0 items-center text-[#111]"
                    >
                      <HugeiconsIcon icon={ArrowDown01Icon} size={20} strokeWidth={1.5} />
                    </motion.span>

                    <span className="flex min-w-px flex-1 items-center gap-[8px] text-left text-[14px] leading-[20px] font-medium tracking-[-0.084px] whitespace-nowrap">
                      <span className="shrink-0 text-[#111]">Cette semaine</span>
                      <span className="shrink-0 text-[#999] tabular-nums">
                        {done.size}/{subtasks.length}
                      </span>
                    </span>
                  </button>
                  {/* Le bouton de la barre d'en-tete, repris tel quel : meme
                      filet, meme ombre d'un pixel, meme encre. Le dessin le
                      posait en gris pale avec un halo de 2px — une action
                      principale qui s'efface derriere les libelles qu'elle
                      ajoute. */}
                  <Button
                    variant="outline"
                    onClick={add}
                    className="border-[#ebebeb] text-[#111] shadow-[0_1px_2px_0_rgb(16_24_40/0.05)] hover:bg-[#f8f8f8]"
                  >
                    <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={1.6} />
                    Ajouter une tâche
                  </Button>
                </div>
              </div>

              {/* Le repli se joue sur la hauteur, pas sur un affichage :
                  `height: auto` laisse Framer mesurer la liste, donc elle
                  s'enroule a la vitesse de ce qu'elle contient. `overflow`
                  masque evite que les lignes debordent pendant le trajet. */}
              <AnimatePresence initial={false}>
                {openSubtasks && (
                  <motion.div
                    key="subtasks"
                    initial={{ height: 0, opacity: 0 }}
                    animate={{ height: 'auto', opacity: 1 }}
                    exit={{ height: 0, opacity: 0 }}
                    transition={transition}
                    className="w-full overflow-hidden"
                  >
                    {/* `axis="y"` borne le glissement a la verticale : une
                        liste ne se reordonne que dans un sens, et laisser
                        partir la ligne de cote donnerait l'impression qu'on
                        peut la sortir. */}
                    <Reorder.Group
                      as="div"
                      axis="y"
                      values={subtasks}
                      onReorder={setSubtasks}
                      className="flex w-full flex-col items-center gap-[4px]"
                    >
                      <AnimatePresence mode="popLayout" initial={false}>
                        {subtasks.map((item) => (
                          <SubtaskRow
                            key={item.id}
                            item={item}
                            done={done}
                            editing={editing}
                            setEditing={setEditing}
                            commitRename={commitRename}
                            toggle={toggle}
                            remove={remove}
                            transition={transition}
                          />
                        ))}
                      </AnimatePresence>
                    </Reorder.Group>
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          )}

          {tab !== 'subtask' && (
            <div className="flex w-full shrink-0 flex-col items-center gap-[8px] rounded-[12px] border border-dashed border-[#ebebeb] px-[16px] py-[32px]">
              <HugeiconsIcon
                icon={tab === 'achievement' ? Award04Icon : Comment01Icon}
                size={24}
                strokeWidth={1.5}
                className="text-[#c4c4c4]"
              />
              <p className="text-center text-[14px] leading-[20px] text-[#999]">
                {tab === 'achievement'
                  ? 'Aucune réussite pour le moment.'
                  : 'Aucun commentaire pour le moment.'}
              </p>
            </div>
          )}
        </div>

        {/* Colonne de droite : 464px au dessin. */}
        <div className="flex h-full w-[464px] shrink-0 flex-col items-start gap-[20px] overflow-y-auto bg-white p-[24px]">
          <p className="shrink-0 text-[18px] leading-[22px] font-medium whitespace-nowrap text-[#111]">
            Statistiques du projet
          </p>

          {/* Les deux blocs prennent le modele de carte du tableau de
              bord : coque grise filetee, intitule en capitales, carte
              blanche posee dedans. Ils portaient jusqu'ici une carte blanche
              sur fond blanc, sans le creux qui la detache. */}
          <DashboardCard
            icon={HourglassIcon}
            title="TEMPS RESTANT"
            className="w-full shrink-0"
            // « 4d » etait ecrit en dur a cote d'une echeance modifiable :
            // les deux se contredisaient des le premier changement de date.
            // Le compte se deduit, et vire au rouge une fois l'echeance
            // passee.
            action={
              <p
                className={cn(
                  'ml-auto shrink-0 text-right text-[14px] leading-[1.5] font-semibold whitespace-nowrap tabular-nums transition-colors',
                  remaining < 0 ? 'text-[#e5484d]' : 'text-[#111]',
                )}
              >
                {remaining < 0 ? `${-remaining}\u00a0j de retard` : `${remaining}\u00a0j`}
              </p>
            }
          >
            <div className="flex w-full items-center gap-[20px]">
              <p className="min-w-px flex-1 text-[12px] leading-[16px] text-[#777]">
                Activer le rappel
              </p>
              {/* L'interrupteur du dessin est fige en position allumee. La
                  pastille glisse d'un bout a l'autre plutot que de sauter :
                  c'est le trajet qui dit lequel des deux etats on vient de
                  quitter. */}
              <button
                type="button"
                role="switch"
                aria-checked={remind}
                aria-label="Activer le rappel"
                onClick={() => setRemind((previous) => !previous)}
                className={cn(
                  'relative h-[20px] w-[36px] shrink-0 cursor-pointer rounded-full transition-colors',
                  remind ? 'bg-[#ff782b]' : 'bg-[#c4c4c4]',
                )}
              >
                <motion.span
                  aria-hidden
                  animate={{ x: remind ? 18 : 2 }}
                  transition={transition}
                  className="absolute top-[2px] left-0 size-[16px] rounded-full bg-white shadow-[0px_2px_2px_0px_rgba(27,28,29,0.12)]"
                />
              </button>
            </div>
          </DashboardCard>

          <DashboardCard icon={Loading03Icon} title="AVANCEMENT" className="w-full shrink-0">
            <div className="flex w-full flex-col gap-[16px]">
              <div className="flex w-full items-center gap-[16px]">
                <p className="min-w-px flex-1 text-[14px] leading-[20px] text-[#777]">
                  L'avancement est à{' '}
                  <span className="font-semibold text-[#111] tabular-nums">{percent}&nbsp;%</span>
                </p>
                {/* La frimousse suit le taux : figee, elle affichait la meme
                    satisfaction a 0 % qu'a 100 %. */}
                <span className="flex shrink-0 items-center rounded-full bg-[#fef3eb] p-[6px]">
                  <span className="size-[16px] overflow-clip text-[16px] leading-none">
                    {percent === 100 ? '🎉' : percent >= 50 ? '😎' : percent > 0 ? '🙂' : '😴'}
                  </span>
                </span>
              </div>

              {/* Le dessin allumait 25 crans sur 36, sans rapport avec ses
                  propres cases. Ils suivent maintenant les sous-taches
                  cochees, et chacun s'allume avec un retard proportionnel a
                  son rang : la jauge se remplit de gauche a droite au lieu
                  de changer d'un bloc.

                  Les crans sont arrondis a plein rayon : a cette largeur ils
                  deviennent des batonnets, et le bord droit ne se confond
                  plus avec le gap qui le suit. */}
              <div className="flex h-[20px] w-full shrink-0 items-center gap-[4px]">
                {Array.from({ length: METER_TOTAL }, (_, index) => (
                  <motion.span
                    key={index}
                    animate={{
                      backgroundColor: index < filled ? '#ff782b' : '#ebebeb',
                    }}
                    transition={{
                      ...transition,
                      delay: Math.min(index, filled) * 0.012,
                    }}
                    className="h-full min-w-px flex-1 rounded-full"
                  />
                ))}
              </div>
            </div>
          </DashboardCard>

          <div className="h-px w-full shrink-0 bg-[#ebebeb]" />

          <div className="flex w-full shrink-0 flex-col items-start gap-[16px]">
            <div className="flex w-full items-center">
              <p className="text-[14px] leading-[20px] font-medium tracking-[-0.14px] whitespace-nowrap text-[#111]">
                Activité
              </p>
            </div>

            <div className="flex w-full flex-col items-start gap-[12px]">
              <div className="flex w-full items-start gap-[12px]">
                <Avatar initials="TF" tint="#c3c7df" size={32} />
                <div className="flex min-w-px flex-1 flex-col items-start justify-center gap-[3px]">
                  <div className="flex w-full flex-col items-start gap-[2px] text-[14px] leading-[20px] font-medium whitespace-nowrap">
                    <div className="flex w-[230px] items-center gap-[8px]">
                      <p className="shrink-0 overflow-hidden text-[14px] tracking-[-0.14px] text-ellipsis text-[#111]">
                        Thomas Fletcher
                      </p>
                      <p className="shrink-0 text-[12px] tracking-[-0.12px] text-[#999]">
                        il y a 30 min
                      </p>
                    </div>
                    <p className="shrink-0 text-[12px] tracking-[-0.12px] text-[#999]">
                      5 juin 2024 à 10:30
                    </p>
                  </div>
                  <div className="flex h-[40px] shrink-0 items-center gap-[12px] overflow-clip py-[4px] pr-[20px]">
                    <Checkbox checked disabled aria-label="Système de grille" />
                    <p className="shrink-0 overflow-hidden text-[14px] leading-[20px] tracking-[-0.084px] text-ellipsis whitespace-nowrap text-[#111] line-through">
                      Système de grille
                    </p>
                  </div>
                </div>
              </div>

              <div className="relative flex w-full items-start gap-[12px]">
                <span className="relative size-[32px] shrink-0">
                  <span className="absolute inset-0 rounded-full border border-[#ebebeb]" />
                  <HugeiconsIcon
                    icon={Tag01Icon}
                    size={18}
                    strokeWidth={1.5}
                    className="absolute top-[7px] left-[7px] text-[#777]"
                  />
                </span>
                <div className="flex min-w-px flex-1 flex-col items-start justify-center gap-[3px]">
                  <div className="flex w-full flex-col items-start gap-[2px] text-[14px] leading-[20px] font-medium whitespace-nowrap">
                    <div className="flex w-[230px] items-center gap-[8px]">
                      <p className="shrink-0 overflow-hidden text-[14px] tracking-[-0.14px] text-ellipsis text-[#111]">
                        Lena Müller
                      </p>
                      <p className="shrink-0 text-[12px] tracking-[-0.12px] text-[#999]">
                        a déplacé la carte
                      </p>
                      <p className="shrink-0 overflow-hidden text-[14px] tracking-[-0.14px] text-ellipsis text-[#111]">
                        Design system
                      </p>
                    </div>
                    <p className="shrink-0 text-[12px] tracking-[-0.12px] text-[#999]">
                      5 juin 2024 à 10:30
                    </p>
                  </div>
                  <div className="flex h-[40px] shrink-0 items-center overflow-clip py-[4px] pr-[20px]">
                    <div className="flex w-[148px] shrink-0 items-center gap-[8px]">
                      <Badge tone="todo">{TASK_STATUS.todo.label}</Badge>
                      <HugeiconsIcon
                        icon={ArrowRight02Icon}
                        size={20}
                        strokeWidth={1.5}
                        className="shrink-0 text-[#999]"
                      />
                      <Badge tone="progress">{TASK_STATUS.progress.label}</Badge>
                    </div>
                  </div>
                </div>
                {/* Le trait qui relie cette entree a la suivante. */}
                <span className="absolute top-[38px] left-[16px] h-[53px] w-px bg-[#ebebeb]" />
              </div>

              <div className="flex w-full items-start gap-[12px]">
                <Avatar initials="LM" tint="#b6cdd8" size={32} />
                <div className="flex min-w-px flex-1 flex-col items-start justify-center">
                  <div className="flex w-full flex-col items-start gap-[2px] text-[14px] leading-[20px] font-medium whitespace-nowrap">
                    <div className="flex w-[230px] items-center gap-[8px]">
                      <p className="shrink-0 overflow-hidden text-[14px] tracking-[-0.14px] text-ellipsis text-[#111]">
                        Lena Müller
                      </p>
                      <p className="shrink-0 text-[12px] tracking-[-0.12px] text-[#999]">
                        a créé la tâche
                      </p>
                      <p className="shrink-0 overflow-hidden text-[14px] tracking-[-0.14px] text-ellipsis text-[#111]">
                        Design system
                      </p>
                    </div>
                    <p className="shrink-0 text-[12px] tracking-[-0.12px] text-[#999]">
                      8 janvier 2024 à 14:00
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className="h-px w-full shrink-0 bg-[#ebebeb]" />

          <div className="flex w-full shrink-0 flex-col items-start gap-[12px]">
            <p className="text-[14px] leading-[20px] tracking-[-0.14px] whitespace-nowrap text-[#777]">
              Créée par
            </p>
            <div className="flex w-full items-center gap-[8px]">
              <Avatar initials="JD" tint="#cfc3a7" size={32} />
              <div className="flex min-w-px flex-1 flex-col items-start justify-center">
                <div className="flex w-[230px] items-center">
                  <p className="overflow-hidden text-[14px] leading-[20px] font-medium tracking-[-0.14px] text-ellipsis whitespace-nowrap text-[#111]">
                    Jhon Doe
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </>
  )
}
