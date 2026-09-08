import {
  Alert02Icon,
  Archive02Icon,
  ArrowDown01Icon,
  ArrowUp01Icon,
  CancelCircleIcon,
  Clock01Icon,
  Copy01Icon,
  Exchange01Icon,
  FolderOpenIcon,
  LinkSquare02Icon,
  MoreHorizontalIcon,
  PlusSignIcon,
  Search01Icon,
  TaskAdd01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'

import { useId, useState, type ReactNode } from 'react'
import { z } from 'zod'

import { HoverBackdrop } from '@/components/hover-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  PROJECTS,
  STATUS,
  STATUS_ORDER,
  type Project,
  type ProjectStatus,
} from '@/features/projects/fixtures'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'

/** Bleu de « facturable », rouge d'alerte : ceux du tableau de bord. */
const BUDGET_COLOR = '#4956f4'
const ALERT_COLOR = '#e5484d'

const PAGE_SIZE = 10

const SORTS = ['name', 'progress', 'budget', 'due'] as const
type Sort = (typeof SORTS)[number]

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })
const RELATIVE_FORMAT = new Intl.RelativeTimeFormat('fr-FR', { numeric: 'auto' })

/**
 * Les filtres vivent dans l'URL, pas dans un useState : la vue est
 * partageable, le bouton Retour fonctionne, et la cle de cache derivera
 * directement de ces parametres le jour ou l'ecran interrogera l'API.
 *
 * Le tri en fait partie pour la meme raison — et parce qu'il devra etre
 * applique par le serveur, sur l'ensemble des projets et non sur la page
 * affichee : trier dix lignes sur quatorze ne trie rien.
 */
const searchSchema = z.object({
  page: z.number().int().min(1).catch(1),
  search: z.string().optional(),
  status: z.enum(['cadrage', 'production', 'attente', 'livre']).optional().catch(undefined),
  client: z.string().optional(),
  member: z.string().optional(),
  sort: z.enum(SORTS).catch('due'),
  dir: z.enum(['asc', 'desc']).catch('asc'),
})

export const Route = createFileRoute('/_app/pm/projets/')({
  validateSearch: searchSchema,
  component: ProjectsPage,
})

/** Jours entiers d'ici a `date` — negatif si l'echeance est passee. */
function daysUntil(date: Date) {
  const today = new Date()
  today.setHours(0, 0, 0, 0)

  return Math.round((date.getTime() - today.getTime()) / 86_400_000)
}

/**
 * Un projet derive quand il consomme plus qu'il n'a vendu, ou quand son
 * echeance est passee sans qu'il soit livre.
 *
 * Ce n'est pas un cinquieme statut : les deux se superposent a n'importe
 * lequel des quatre, et un projet peut deriver sur les deux tableaux a la
 * fois.
 *
 * La derive ne se signale que dans la colonne concernee — le budget rougit le
 * budget, le retard rougit l'echeance. Pas de marqueur sur la ligne entiere :
 * il fallait en connaitre la regle pour le lire, et une couleur qui demande
 * une explication ne dit rien.
 */
function driftOf(project: Project) {
  return {
    overBudget: project.hoursSpent > project.hoursSold,
    late: project.status !== 'livre' && daysUntil(project.due) < 0,
  }
}

/** Une entree de menu de filtre. `color` pose une pastille, `total` un compte. */
interface Option {
  value: string
  label: string
  color?: string
  total: number
}

/**
 * Entree de menu dont le fond de survol glisse depuis l'entree precedente.
 *
 * Le fond natif de Radix est neutralise (`focus:bg-transparent`) et remplace
 * par un noeud unique partage entre toutes les entrees d'un meme menu : c'est
 * le `layoutId` qui le fait glisser plutot que disparaitre ici et reapparaitre
 * la — la meme mecanique que la pastille des onglets du graphique de charge.
 *
 * `onFocus` autant que `onPointerEnter` : Radix deplace le focus a la fleche
 * du clavier, et le fond doit suivre la navigation au clavier comme il suit le
 * curseur.
 *
 * L'indicateur de selection est remonte au-dessus du fond par son `data-slot` :
 * pose apres lui dans le DOM, le fond opaque le masquerait.
 */
function HoverItem({
  value,
  layoutId,
  hovered,
  onHover,
  transition,
  children,
}: {
  value: string
  layoutId: string
  hovered: string | null
  onHover: (value: string) => void
  transition: ReturnType<typeof useSlideTransition>
  children: ReactNode
}) {
  return (
    <DropdownMenuRadioItem
      value={value}
      onPointerEnter={() => onHover(value)}
      onFocus={() => onHover(value)}
      className="relative text-[13px] focus:bg-transparent [&>[data-slot=dropdown-menu-radio-item-indicator]]:z-10"
    >
      {hovered === value && <HoverBackdrop layoutId={layoutId} transition={transition} />}

      {/* Au-dessus du fond : sans quoi le libelle disparaitrait derriere lui
          pendant le glissement. */}
      <span className="relative z-10 flex w-full items-center gap-1.5">{children}</span>
    </DropdownMenuRadioItem>
  )
}

/**
 * Entree du menu d'actions d'une ligne.
 *
 * Aucune n'est cablee : la maquette les pose, les ecrans qu'elles ouvrent
 * n'existent pas encore — comme les deux boutons de l'en-tete de frame.
 */
function ActionItem({
  id,
  icon,
  label,
  danger,
  onSelect,
  layoutId,
  hovered,
  onHover,
  transition,
}: {
  id: string
  icon: IconSvgElement
  label: string
  danger?: boolean
  onSelect?: () => void
  layoutId: string
  hovered: string | null
  onHover: (id: string) => void
  transition: ReturnType<typeof useSlideTransition>
}) {
  return (
    <DropdownMenuItem
      onSelect={onSelect}
      onPointerEnter={() => onHover(id)}
      onFocus={() => onHover(id)}
      className={cn(
        'relative text-[13px] focus:bg-transparent',
        danger ? 'text-[#e5484d] focus:text-[#e5484d]' : 'text-[#111]',
      )}
    >
      {hovered === id && (
        <HoverBackdrop
          layoutId={layoutId}
          transition={transition}
          color={danger ? '#fdecec' : '#f4f4f4'}
        />
      )}

      <span className="relative z-10 flex w-full items-center gap-2">
        <HugeiconsIcon
          icon={icon}
          size={16}
          strokeWidth={1.6}
          className={danger ? 'text-[#e5484d]' : 'text-[#64748b]'}
        />
        {label}
      </span>
    </DropdownMenuItem>
  )
}

/**
 * Menu d'actions d'une ligne de projet.
 *
 * Trois groupes : ce qu'on fait sans quitter la liste, ce qui change l'etat du
 * projet, puis ce qui le sort de la liste. L'ordre n'est pas decoratif — il
 * met la distance entre « saisir du temps », qu'on declenche vingt fois par
 * jour, et « archiver », qu'on ne veut pas atteindre par megarde.
 *
 * Pas de « Supprimer » : un projet porte du temps saisi et des livrables
 * valides. L'archivage est reversible, la suppression ne l'est pas.
 */
function RowActions({ project }: { project: Project }) {
  const navigate = useNavigate()
  const layoutId = useId()
  const subLayoutId = useId()
  const [hovered, setHovered] = useState<string | null>(null)
  const [subHovered, setSubHovered] = useState<string | null>(null)
  const transition = useSlideTransition()

  return (
    <DropdownMenu onOpenChange={(open) => !open && setHovered(null)}>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={`Actions sur ${project.name}`}>
          <HugeiconsIcon icon={MoreHorizontalIcon} size={16} strokeWidth={1.6} />
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent
        align="end"
        className="min-w-[210px]"
        onPointerLeave={() => setHovered(null)}
      >
        {/* Agir sans quitter la liste : c'est la raison d'etre du menu. La
            saisie de temps est l'action la plus frequente d'une agence, et la
            seule qui justifie a elle seule de ne pas ouvrir la fiche. */}
        <ActionItem
          id="open"
          icon={FolderOpenIcon}
          label="Ouvrir le projet"
          onSelect={() => void navigate({ to: '/pm/projets/$id', params: { id: project.id } })}
          layoutId={layoutId}
          hovered={hovered}
          onHover={setHovered}
          transition={transition}
        />
        <ActionItem
          id="time"
          icon={Clock01Icon}
          label="Saisir du temps"
          layoutId={layoutId}
          hovered={hovered}
          onHover={setHovered}
          transition={transition}
        />
        <ActionItem
          id="task"
          icon={TaskAdd01Icon}
          label="Créer une tâche"
          layoutId={layoutId}
          hovered={hovered}
          onHover={setHovered}
          transition={transition}
        />

        <DropdownMenuSeparator />

        {/* Changer l'etat. Le statut en sous-menu plutot qu'en boite de
            dialogue : quatre valeurs exclusives ne meritent pas un ecran. */}
        <DropdownMenuSub>
          <DropdownMenuSubTrigger
            onPointerEnter={() => setHovered('status')}
            onFocus={() => setHovered('status')}
            className="relative text-[13px] text-[#111] focus:bg-transparent data-open:bg-transparent"
          >
            {hovered === 'status' && (
              <HoverBackdrop layoutId={layoutId} transition={transition} />
            )}

            <span className="relative z-10 flex w-full items-center gap-2">
              <HugeiconsIcon
                icon={Exchange01Icon}
                size={16}
                strokeWidth={1.6}
                className="text-[#64748b]"
              />
              Changer le statut
            </span>
          </DropdownMenuSubTrigger>

          <DropdownMenuSubContent
            className="min-w-[170px]"
            onPointerLeave={() => setSubHovered(null)}
          >
            {STATUS_ORDER.map((status) => (
              <DropdownMenuItem
                key={status}
                disabled={status === project.status}
                onPointerEnter={() => setSubHovered(status)}
                onFocus={() => setSubHovered(status)}
                className="relative text-[13px] text-[#111] focus:bg-transparent"
              >
                {subHovered === status && (
                  <HoverBackdrop layoutId={subLayoutId} transition={transition} />
                )}

                <span className="relative z-10 flex w-full items-center gap-2">
                  <span
                    aria-hidden
                    className="size-1.5 shrink-0 rounded-full"
                    style={{ backgroundColor: STATUS[status].color }}
                  />
                  {STATUS[status].label}
                </span>
              </DropdownMenuItem>
            ))}
          </DropdownMenuSubContent>
        </DropdownMenuSub>

        {/* Le portail est le produit livre au client : pouvoir verifier ce
            qu'il voit de ce projet, depuis la liste, n'existe nulle part
            ailleurs dans l'application. */}
        <ActionItem
          id="portal"
          icon={LinkSquare02Icon}
          label="Voir le portail client"
          layoutId={layoutId}
          hovered={hovered}
          onHover={setHovered}
          transition={transition}
        />

        <DropdownMenuSeparator />

        <ActionItem
          id="template"
          icon={Copy01Icon}
          label="Dupliquer comme modèle"
          layoutId={layoutId}
          hovered={hovered}
          onHover={setHovered}
          transition={transition}
        />
        <ActionItem
          id="archive"
          icon={Archive02Icon}
          label="Archiver le projet"
          danger
          layoutId={layoutId}
          hovered={hovered}
          onHover={setHovered}
          transition={transition}
        />
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/**
 * Filtre en menu deroulant.
 *
 * Le declencheur porte la valeur choisie et non le nom du champ : « Client »
 * seul obligerait a ouvrir le menu pour savoir sur quoi la liste est filtree.
 * Sans selection il porte le nom, ce qui dit ce qu'il ouvrira.
 *
 * Un groupe radio et non des cases : un projet n'a qu'un statut, et le multi-
 * selection demanderait de rendre « Client : 3 sélectionnés » lisible dans un
 * bouton de 140px. Il viendra si le besoin se presente.
 */
function FilterMenu({
  name,
  all,
  value,
  options,
  onChange,
}: {
  name: string
  /** Libelle de l'entree qui leve le filtre. */
  all: string
  value: string | undefined
  options: Option[]
  onChange: (value: string | undefined) => void
}) {
  const selected = options.find((option) => option.value === value)

  // Un identifiant par instance : trois menus partageant un `layoutId` verraient
  // leur fond tenter de glisser de l'un a l'autre.
  const layoutId = useId()
  const [hovered, setHovered] = useState<string | null>(null)
  const transition = useSlideTransition()

  return (
    <DropdownMenu onOpenChange={(open) => !open && setHovered(null)}>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="lg"
          className={cn('gap-1.5 text-[13px]', selected && 'border-[#d4d4d4]')}
        >
          {selected?.color && (
            <span
              aria-hidden
              className="size-1.5 shrink-0 rounded-full"
              style={{ backgroundColor: selected.color }}
            />
          )}

          <span className={selected ? 'text-[#111]' : 'text-[#64748b]'}>
            {selected?.label ?? name}
          </span>

          <HugeiconsIcon
            icon={ArrowDown01Icon}
            size={14}
            strokeWidth={2}
            className="text-[#8d8d8d]"
          />
        </Button>
      </DropdownMenuTrigger>

      {/* `align="start"` : le menu tombe sous le bord gauche de son bouton, donc
          la colonne de libelles reste alignee avec ce qu'on vient de lire. */}
      {/* Le fond de survol s'efface des que le curseur quitte le menu : un
          survol qui resterait affiche apres coup designerait une entree que
          plus personne ne pointe. */}
      <DropdownMenuContent
        align="start"
        className="min-w-[200px]"
        onPointerLeave={() => setHovered(null)}
      >
        <DropdownMenuRadioGroup
          value={value ?? ''}
          onValueChange={(next) => onChange(next === '' ? undefined : next)}
        >
          <HoverItem
            value=""
            layoutId={layoutId}
            hovered={hovered}
            onHover={setHovered}
            transition={transition}
          >
            {all}
          </HoverItem>

          <DropdownMenuSeparator />

          {options.map((option) => (
            <HoverItem
              key={option.value}
              value={option.value}
              layoutId={layoutId}
              hovered={hovered}
              onHover={setHovered}
              transition={transition}
            >
              {option.color && (
                <span
                  aria-hidden
                  className="size-1.5 shrink-0 rounded-full"
                  style={{ backgroundColor: option.color }}
                />
              )}
              <span className="truncate">{option.label}</span>
              {/* Le compte porte sur l'ensemble et non sur la liste affichee :
                  un filtre annonce ce qu'il va montrer, pas ce qu'on a deja
                  sous les yeux.

                  `!` sur la couleur : Radix repeint tous les descendants de
                  l'entree focalisee, ce qui ecraserait ce gris. */}
              <span className="ml-auto pl-2 text-[12px] text-[#8d8d8d]! tabular-nums">
                {option.total}
              </span>
            </HoverItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** Jauge de la carte « Équipe aujourd'hui », reprise telle quelle. */
function Meter({ ratio, color, className }: { ratio: number; color: string; className?: string }) {
  return (
    <div
      aria-hidden
      className={cn('h-1 w-full max-w-[72px] overflow-hidden rounded-full bg-[#ebebeb]', className)}
    >
      <div
        className="h-full rounded-full"
        style={{ width: `${Math.min(100, Math.max(0, ratio))}%`, backgroundColor: color }}
      />
    </div>
  )
}

/**
 * En-tete de colonne triable.
 *
 * La fleche n'apparait que sur la colonne active : trois fleches grises en
 * permanence font un bandeau de bruit, et on ne sait plus laquelle compte.
 */
function SortHead({
  sort,
  label,
  current,
  dir,
  onSort,
  className,
}: {
  sort: Sort
  label: string
  current: Sort
  dir: 'asc' | 'desc'
  onSort: (sort: Sort) => void
  className?: string
}) {
  const active = current === sort

  return (
    <TableHead className={cn('h-9 px-3', className)}>
      <button
        type="button"
        onClick={() => onSort(sort)}
        aria-sort={active ? (dir === 'asc' ? 'ascending' : 'descending') : 'none'}
        className="flex items-center gap-1 text-[12px] font-medium text-[#64748b] transition-colors hover:text-[#0f172a]"
      >
        {label}
        {active && (
          <HugeiconsIcon
            icon={dir === 'asc' ? ArrowUp01Icon : ArrowDown01Icon}
            size={14}
            strokeWidth={2}
          />
        )}
      </button>
    </TableHead>
  )
}

/**
 * Equipe affectee, en pile.
 *
 * Trois visages au plus puis un compteur : la colonne garde une largeur fixe,
 * qu'un projet mobilise une personne ou cinq.
 */
function TeamStack({ team }: { team: Project['team'] }) {
  const shown = team.slice(0, 3)
  const rest = team.length - shown.length

  return (
    <div className="flex items-center">
      {shown.map((member, index) => (
        <div
          key={`${member.initials}-${index}`}
          className="flex size-6 items-center justify-center rounded-[6px] text-[10px] font-semibold text-[#0f172a] ring-2 ring-white not-first:-ml-1.5"
          style={{ backgroundColor: member.tint }}
        >
          {member.initials}
        </div>
      ))}

      {rest > 0 && (
        <div className="-ml-1.5 flex size-6 items-center justify-center rounded-[6px] bg-[#f0f0f0] text-[10px] font-semibold text-[#64748b] ring-2 ring-white">
          +{rest}
        </div>
      )}
    </div>
  )
}

function ProjectsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  // Filtre, tri et decoupe en pages sont faits ici parce que la liste est
  // figee. Rien de tout cela ne doit survivre au branchement de l'API : le
  // projet impose que toute liste soit paginee par le serveur, et un tri de
  // client ne trie jamais que ce qu'il a deja recu.
  const needle = search.search?.trim().toLowerCase() ?? ''

  const filtered = PROJECTS.filter(
    (project) =>
      (search.status === undefined || project.status === search.status) &&
      (search.client === undefined || project.client === search.client) &&
      (search.member === undefined ||
        project.team.some((member) => member.initials === search.member)) &&
      (needle === '' ||
        project.name.toLowerCase().includes(needle) ||
        project.client.toLowerCase().includes(needle)),
  )

  const sorted = [...filtered].sort((a, b) => {
    const way = search.dir === 'asc' ? 1 : -1

    switch (search.sort) {
      case 'name':
        return way * a.name.localeCompare(b.name, 'fr')
      case 'progress':
        return way * (a.progress - b.progress)
      case 'budget':
        return way * (a.hoursSpent / a.hoursSold - b.hoursSpent / b.hoursSold)
      case 'due':
        return way * (a.due.getTime() - b.due.getTime())
    }
  })

  const totalPages = Math.max(1, Math.ceil(sorted.length / PAGE_SIZE))
  const page = Math.min(search.page, totalPages)
  const rows = sorted.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  const drifting = PROJECTS.filter((project) => {
    const drift = driftOf(project)
    return drift.overBudget || drift.late
  }).length

  // Les compteurs portent sur l'ensemble, jamais sur la page : un filtre doit
  // annoncer ce qu'il va montrer, pas ce qui se trouve sous les yeux.
  const statusOptions: Option[] = STATUS_ORDER.map((status) => ({
    value: status,
    label: STATUS[status].label,
    color: STATUS[status].color,
    total: PROJECTS.filter((project) => project.status === status).length,
  }))

  const clientOptions: Option[] = [...new Set(PROJECTS.map((project) => project.client))]
    .sort((a, b) => a.localeCompare(b, 'fr'))
    .map((client) => ({
      value: client,
      label: client,
      total: PROJECTS.filter((project) => project.client === client).length,
    }))

  // Les personnes sont dedupliquees par initiales : le fixture les partage par
  // reference, mais l'API renverra un membre par projet.
  const memberOptions: Option[] = [
    ...new Map(
      PROJECTS.flatMap((project) => project.team).map((member) => [member.initials, member]),
    ).values(),
  ]
    .sort((a, b) => a.name.localeCompare(b.name, 'fr'))
    .map((member) => ({
      value: member.initials,
      label: member.name,
      color: member.tint,
      total: PROJECTS.filter((project) =>
        project.team.some((entry) => entry.initials === member.initials),
      ).length,
    }))

  const active = [search.status, search.client, search.member].filter(Boolean).length

  /** Tout changement de filtre ramene en page 1 : la page 2 d'une autre liste n'a pas de sens. */
  function setFilter(patch: {
    search?: string
    status?: ProjectStatus
    client?: string
    member?: string
  }) {
    void navigate({ search: (prev) => ({ ...prev, ...patch, page: 1 }) })
  }

  /** Recliquer sur la colonne active inverse le sens plutot que de ne rien faire. */
  function setSort(sort: Sort) {
    void navigate({
      search: (prev) => ({
        ...prev,
        sort,
        dir: prev.sort === sort && prev.dir === 'asc' ? 'desc' : 'asc',
        page: 1,
      }),
    })
  }

  return (
    <PageFrame
      title="Projets"
      description={`${PROJECTS.length} projets · ${drifting} en dérive`}
    >
      <div className="flex flex-col gap-3 p-4">
        {/* Barre d'outils : recherche, puis les filtres, puis l'action.
            L'ordre suit la lecture — on reduit la liste de gauche a droite,
            et ce qui la quitte (creer un projet) est au bout.

            La recherche prend la largeur disponible et les menus passent a la
            ligne : sur un ecran etroit, un champ de recherche de 120px ne sert
            a personne. */}
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative min-w-[200px] flex-1">
            <HugeiconsIcon
              icon={Search01Icon}
              size={16}
              strokeWidth={1.6}
              className="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-[#8d8d8d]"
            />
            <Input
              type="search"
              value={search.search ?? ''}
              onChange={(event) => setFilter({ search: event.target.value || undefined })}
              placeholder="Rechercher un projet ou un client"
              aria-label="Rechercher un projet ou un client"
              className="h-9 pl-8 text-[13px]"
            />
          </div>

          <FilterMenu
            name="Statut"
            all="Tous les statuts"
            value={search.status}
            options={statusOptions}
            onChange={(value) => setFilter({ status: value as ProjectStatus | undefined })}
          />

          <FilterMenu
            name="Client"
            all="Tous les clients"
            value={search.client}
            options={clientOptions}
            onChange={(value) => setFilter({ client: value })}
          />

          <FilterMenu
            name="Équipe"
            all="Toute l’équipe"
            value={search.member}
            options={memberOptions}
            onChange={(value) => setFilter({ member: value })}
          />

          {/* N'apparait qu'une fois quelque chose a effacer : un bouton
              toujours la, toujours inerte, occupe la place sans rien dire. */}
          {active > 0 && (
            <Button
              variant="ghost"
              size="lg"
              className="gap-1.5 text-[#64748b]"
              onClick={() => setFilter({ status: undefined, client: undefined, member: undefined })}
            >
              <HugeiconsIcon icon={CancelCircleIcon} size={16} strokeWidth={1.6} />
              Effacer ({active})
            </Button>
          )}

          <Button size="lg" className="ml-auto gap-1.5">
            <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
            Nouveau projet
          </Button>
        </div>

        <div className="overflow-hidden rounded-[12px] border border-[#ebebeb]">
          <Table>
            <TableHeader className="[&_tr]:border-[#ebebeb]">
              <TableRow className="bg-[#f8f8f8] hover:bg-[#f8f8f8]">
                <SortHead
                  sort="name"
                  label="Projet"
                  current={search.sort}
                  dir={search.dir}
                  onSort={setSort}
                />
                <TableHead className="h-9 px-3 text-[12px] font-medium text-[#64748b]">
                  Statut
                </TableHead>
                <SortHead
                  sort="progress"
                  label="Avancement"
                  current={search.sort}
                  dir={search.dir}
                  onSort={setSort}
                  className="hidden md:table-cell"
                />
                <SortHead
                  sort="budget"
                  label="Budget"
                  current={search.sort}
                  dir={search.dir}
                  onSort={setSort}
                  className="hidden lg:table-cell"
                />
                <SortHead
                  sort="due"
                  label="Échéance"
                  current={search.sort}
                  dir={search.dir}
                  onSort={setSort}
                  className="hidden sm:table-cell"
                />
                <TableHead className="hidden h-9 px-3 text-[12px] font-medium text-[#64748b] xl:table-cell">
                  Équipe
                </TableHead>
                <TableHead className="h-9 w-10 px-3" />
              </TableRow>
            </TableHeader>

            <TableBody>
              {rows.length === 0 && (
                <TableRow className="border-[#ebebeb] hover:bg-transparent">
                  <TableCell colSpan={7} className="h-24 text-center text-[13px] text-[#64748b]">
                    Aucun projet ne correspond à ce filtre.
                  </TableCell>
                </TableRow>
              )}

              {rows.map((project) => {
                const drift = driftOf(project)
                const days = daysUntil(project.due)
                const budgetRatio = (project.hoursSpent / project.hoursSold) * 100

                return (
                  <TableRow
                    key={project.id}
                    className="border-[#ebebeb] hover:bg-[#f8f8f8] [&>td]:h-14 [&>td]:px-3"
                  >
                    <TableCell>
                      <div className="flex min-w-0 flex-col">
                        {/* Le nom porte le lien plutot que la ligne entiere :
                            une ligne cliquable avalerait le menu d'actions et
                            la selection du texte, et n'offrirait pas de cible
                            au clic-milieu ni au « ouvrir dans un onglet ». */}
                        <Link
                          to="/pm/projets/$id"
                          params={{ id: project.id }}
                          className="truncate text-[13px] font-semibold text-[#0f172a] hover:underline"
                        >
                          {project.name}
                        </Link>
                        <p className="truncate text-[12px] text-[#64748b]">{project.client}</p>
                      </div>
                    </TableCell>

                    <TableCell>
                      <div className="flex items-center gap-1.5">
                        <span
                          aria-hidden
                          className="size-1.5 shrink-0 rounded-full"
                          style={{ backgroundColor: STATUS[project.status].color }}
                        />
                        <span className="text-[12px] whitespace-nowrap text-[#0f172a]">
                          {STATUS[project.status].label}
                        </span>
                      </div>
                    </TableCell>

                    <TableCell className="hidden md:table-cell">
                      <div className="flex items-center gap-2">
                        <Meter ratio={project.progress} color={STATUS[project.status].color} />
                        <span className="w-9 text-right text-[12px] font-medium text-[#0f172a] tabular-nums">
                          {project.progress} %
                        </span>
                      </div>
                    </TableCell>

                    {/* Heures consommees sur heures vendues. Le chiffre seul
                        obligerait a faire la division de tete a chaque ligne ;
                        la jauge la fait, et le rouge dit le depassement sans
                        qu'on ait a comparer les deux nombres. */}
                    <TableCell className="hidden lg:table-cell">
                      <div className="flex items-center gap-2">
                        <Meter
                          ratio={budgetRatio}
                          color={drift.overBudget ? ALERT_COLOR : BUDGET_COLOR}
                        />
                        <span
                          className={cn(
                            'text-[12px] font-medium whitespace-nowrap tabular-nums',
                            drift.overBudget ? 'text-[#e5484d]' : 'text-[#64748b]',
                          )}
                        >
                          {project.hoursSpent} / {project.hoursSold} h
                        </span>
                      </div>
                    </TableCell>

                    <TableCell className="hidden sm:table-cell">
                      <div className="flex flex-col">
                        <span className="text-[13px] whitespace-nowrap text-[#0f172a] tabular-nums">
                          {DATE_FORMAT.format(project.due)}
                        </span>
                        <span
                          className={cn(
                            'flex items-center gap-1 text-[12px] whitespace-nowrap',
                            drift.late ? 'font-medium text-[#e5484d]' : 'text-[#64748b]',
                          )}
                        >
                          {drift.late && (
                            <HugeiconsIcon icon={Alert02Icon} size={13} strokeWidth={2} />
                          )}
                          {RELATIVE_FORMAT.format(days, 'day')}
                        </span>
                      </div>
                    </TableCell>

                    <TableCell className="hidden xl:table-cell">
                      <TeamStack team={project.team} />
                    </TableCell>

                    <TableCell>
                      <RowActions project={project} />
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-[12px] text-[#64748b]">
            {sorted.length} projet{sorted.length > 1 ? 's' : ''} · page {page} sur {totalPages}
          </p>

          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => void navigate({ search: (prev) => ({ ...prev, page: page - 1 }) })}
            >
              Précédent
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= totalPages}
              onClick={() => void navigate({ search: (prev) => ({ ...prev, page: page + 1 }) })}
            >
              Suivant
            </Button>
          </div>
        </div>
      </div>
    </PageFrame>
  )
}

