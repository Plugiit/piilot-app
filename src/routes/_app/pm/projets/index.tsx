import {
  Alert02Icon,
  ArrowDown01Icon,
  ArrowRight01Icon,
  ArrowUp01Icon,
  ArrowUpDownIcon,
  Calendar03Icon,
  Flag02Icon,
  Exchange01Icon,
  FolderOpenIcon,
  MoreHorizontalIcon,
  Search01Icon,
  Settings02Icon,
  StarIcon,
  TaskAdd01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'

import { useId, useState } from 'react'
import { toast } from 'sonner'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { HoverBackdrop } from '@/components/hover-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import {
  clientListQuery,
  projectListQuery,
  useToggleFavorite,
  useUpdateProject,
  type ProjectListParams,
} from '@/features/projects/api'
import {
  DONE_COLOR,
  PROGRESS_COLOR,
  PROJECT_PRIORITY as PRIORITY,
  PROJECT_STATUS as STATUS,
  PROJECT_STATUS_ORDER as STATUS_ORDER,
  daysUntil as daysBetween,
  parseApiDate,
  tintOf,
} from '@/features/projects/format'
import { HttpError } from '@/lib/api'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'
import type { Person, Project, ProjectStatus } from '@/types/api'

import { NewProjectDialog } from './-new-project'

/** Neuf projets par page : trois rangees pleines de la grille a trois colonnes. */
const PAGE_SIZE = 9

const SORTS = ['name', 'progress', 'budget', 'due'] as const
type Sort = (typeof SORTS)[number]

const SORT_LABELS: Record<Sort, string> = {
  due: 'Échéance',
  name: 'Nom',
  progress: 'Avancement',
  budget: 'Budget',
}

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'short',
  year: 'numeric',
})

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
  client_id: z.string().optional(),
  sort: z.enum(SORTS).catch('due'),
  dir: z.enum(['asc', 'desc']).catch('asc'),
})

export const Route = createFileRoute('/_app/pm/projets/')({
  validateSearch: searchSchema,
  component: ProjectsPage,
})

/** Jours entiers d'ici a `date` — negatif si l'echeance est passee. */
function daysUntil(date: Date) {
  return daysBetween(date)
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
  const due = parseApiDate(project.due_on)

  return {
    overBudget: project.hours_spent > project.hours_sold,
    late: project.status !== 'livre' && due !== null && daysUntil(due) < 0,
  }
}


/**
 * Entree du menu d'actions d'une ligne.
 *
 * `onSelect` n'est pas optionnel : une entree de menu qui ne fait rien est un
 * bouton casse, et c'est ce que ce menu etait — la maquette posait sept
 * entrees dont une seule agissait. N'y figure desormais que ce qui s'execute.
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
  onSelect: () => void
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
 * Il ne porte que ce qui s'execute aujourd'hui. Il proposait « Saisir du
 * temps », « Voir le portail client », « Dupliquer comme modele » et
 * « Archiver » : quatre entrees sans code derriere, dont trois attendent un
 * module qui n'est pas au programme de ce lot — et l'archivage n'existe pas
 * cote API, ou seule la suppression logique est ecrite.
 *
 * Pas de « Supprimer » non plus, alors que l'endpoint existe : la suppression
 * demande de recopier le nom du projet, dans la zone de danger. Un menu
 * contextuel qui effacerait en deux clics contournerait ce garde-fou au lieu
 * de le servir. L'entree « Parametres » y mene.
 */
function RowActions({ project }: { project: Project }) {
  const navigate = useNavigate()
  const layoutId = useId()
  const subLayoutId = useId()
  const [hovered, setHovered] = useState<string | null>(null)
  const [subHovered, setSubHovered] = useState<string | null>(null)
  const transition = useSlideTransition()

  const favorite = useToggleFavorite(project.id)
  const update = useUpdateProject(project.id)

  function changeStatus(status: ProjectStatus) {
    update.mutate(
      { status },
      {
        onSuccess: () => toast.success(`« ${project.name} » passe en ${STATUS[status].label}`),
        onError: (error) =>
          toast.error(error instanceof HttpError ? error.message : 'Changement impossible'),
      },
    )
  }

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

        {/* L'etoile de la fiche, a portee de la liste : c'est elle qui decide
            de l'ordre de cette page et des raccourcis de la barre laterale,
            et devoir ouvrir un projet pour l'y ranger est un detour. */}
        <ActionItem
          id="favorite"
          icon={StarIcon}
          label={project.is_favorite ? 'Retirer des favoris' : 'Mettre en favori'}
          onSelect={() => favorite.mutate(!project.is_favorite)}
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
                disabled={status === project.status || update.isPending}
                onSelect={() => changeStatus(status)}
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

        <DropdownMenuSeparator />

        <ActionItem
          id="settings"
          icon={Settings02Icon}
          label="Paramètres du projet"
          onSelect={() =>
            void navigate({ to: '/pm/projets/$id/parametres', params: { id: project.id } })
          }
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
 * Pile des membres affectes, en haut a droite de la carte.
 *
 * La photo du compte quand il en a une, ses initiales sinon — sur la pastille
 * de couleur deduite de son identifiant, qui sert deja partout ailleurs. Les
 * deux font le meme disque de 20 px : la pile ne se deforme pas selon que les
 * comptes ont ou non depose une photo.
 */
function TeamStack({ team }: { team: Person[] }) {
  const shown = team.slice(0, 3)
  const rest = team.length - shown.length

  return (
    <div className="flex items-center">
      {shown.map((member) => {
        const name = `${member.firstname} ${member.lastname}`.trim() || 'Sans nom'

        return (
          <div
            key={member.id}
            title={name}
            className="relative size-5 shrink-0 overflow-hidden rounded-full border border-white not-first:-ml-1.5"
            style={{ backgroundColor: tintOf(member.id) }}
          >
            {member.avatar_url == null || member.avatar_url === '' ? (
              <span className="flex size-full items-center justify-center text-[9px] font-medium text-[#1b1b1b]">
                {member.initials}
              </span>
            ) : (
              <img
                src={member.avatar_url}
                alt=""
                loading="lazy"
                className="size-full object-cover"
              />
            )}
          </div>
        )
      })}

      {rest > 0 && (
        <div
          title={`${rest} autre${rest > 1 ? 's' : ''}`}
          className="-ml-1.5 flex size-5 shrink-0 items-center justify-center rounded-full border border-white bg-[#e8e8e9] text-[11px] font-medium text-[#1b1b1b]"
        >
          {rest}+
        </div>
      )}
    </div>
  )
}

/** Vignette d'etat : fond clair, bordure a peine plus soutenue, point plein. */
function StatusPill({ status }: { status: ProjectStatus }) {
  const { label, color, pill } = STATUS[status]

  return (
    <span
      className="inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[12px] whitespace-nowrap"
      style={{ backgroundColor: pill.bg, borderColor: pill.border, color: pill.text }}
    >
      <span aria-hidden className="size-1.5 rounded-full" style={{ backgroundColor: color }} />
      {label}
    </span>
  )
}

/**
 * Anneau d'avancement.
 *
 * Trace a partir de la circonference plutot qu'avec une animation : la valeur
 * est connue au rendu, et un anneau qui se remplit a chaque changement de page
 * ferait clignoter neuf cartes pour ne rien apprendre.
 */
function ProgressRing({ value }: { value: number }) {
  const ratio = Math.max(0, Math.min(100, value)) / 100
  const circumference = 2 * Math.PI * 8

  return (
    <svg viewBox="0 0 20 20" className="size-5 shrink-0 -rotate-90" aria-hidden>
      <circle cx="10" cy="10" r="8" fill="none" stroke="#e8e8e9" strokeWidth="2.5" />
      {ratio > 0 && (
        <circle
          cx="10"
          cy="10"
          r="8"
          fill="none"
          stroke={value >= 100 ? DONE_COLOR : PROGRESS_COLOR}
          strokeWidth="2.5"
          strokeLinecap="round"
          strokeDasharray={`${ratio * circumference} ${circumference}`}
        />
      )}
    </svg>
  )
}

/**
 * Carte d'un projet.
 *
 * Trois etages, comme la maquette : l'etat et l'equipe en haut, l'identite au
 * milieu, ce qu'on peut en faire en bas. Le titre porte le lien plutot que la
 * carte entiere — une carte cliquable avalerait le menu d'actions et
 * n'offrirait pas de cible au clic-milieu.
 */
function ProjectCard({ project }: { project: Project }) {
  const drift = driftOf(project)
  const due = parseApiDate(project.due_on)

  return (
    <article className="flex flex-col gap-3 rounded-[12px] border border-[#e8e8e9] bg-white p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-1.5">
          {/* Dit pourquoi la carte est en tete : l'ordre vient du serveur, qui
              remonte les projets etoiles avant tous les autres. Sans marque,
              le classement passerait pour un caprice du tri. */}
          {project.is_favorite && (
            <HugeiconsIcon
              icon={StarIcon}
              size={16}
              strokeWidth={1.6}
              aria-label="En favori"
              className="fill-brand text-brand shrink-0"
            />
          )}
          <StatusPill status={project.status} />
        </div>
        <div className="flex items-center gap-1">
          <TeamStack team={project.team} />
          <RowActions project={project} />
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <div className="flex min-w-0 flex-col gap-0.5">
          <Link
            to="/pm/projets/$id"
            params={{ id: project.id }}
            className="truncate text-[16px] leading-[1.5] font-medium text-[#1b1b1b] hover:underline"
          >
            {project.name}
          </Link>
          {/* A defaut de resume, le client : une carte sans deuxieme ligne se
              tasse et la grille perd son alignement. */}
          <p className="truncate text-[14px] leading-[1.5] text-[#73757c]">
            {project.description === '' ? project.client_name : project.description}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span
            className={cn(
              'flex items-center gap-1 text-[12px] whitespace-nowrap',
              drift.late ? 'font-medium text-[#e5484d]' : 'text-[#73757c]',
            )}
          >
            <HugeiconsIcon
              icon={drift.late ? Alert02Icon : Calendar03Icon}
              size={16}
              strokeWidth={1.6}
            />
            {due === null ? 'Sans échéance' : DATE_FORMAT.format(due)}
          </span>

          <span className="flex items-center gap-1 text-[12px] whitespace-nowrap text-[#73757c]">
            <HugeiconsIcon icon={Flag02Icon} size={16} strokeWidth={1.6} />
            {PRIORITY[project.priority].label}
          </span>
        </div>
      </div>

      <div className="mt-auto flex items-center justify-between gap-2">
        <Button
          asChild
          variant="outline"
          className="h-8 gap-2 rounded-[8px] px-2.5 text-[12px]"
        >
          <Link to="/pm/projets/$id" params={{ id: project.id }}>
            Voir le détail
            <HugeiconsIcon icon={ArrowRight01Icon} size={16} strokeWidth={1.6} />
          </Link>
        </Button>

        <span className="flex shrink-0 items-center gap-1 rounded-[8px] bg-[#f3f4f4] p-1.5 text-[12px] font-medium text-[#1b1b1b]">
          <ProgressRing value={project.progress} />
          <span className="tabular-nums">{project.progress} %</span>
        </span>
      </div>
    </article>
  )
}

/** Derniere case de la grille : ouvre le meme dialogue que la barre d'outils. */
function AddProjectCard() {
  return (
    <NewProjectDialog
      trigger={
        <button
          type="button"
          className="flex min-h-[173px] cursor-pointer flex-col items-center justify-center gap-3 rounded-[12px] border border-dashed border-[#d0d1d3] bg-white p-3 text-center transition-colors hover:border-[#b5b6b9] hover:bg-[#fcfcfc]"
        >
          <span className="rounded-full border border-[#e8e8e9] bg-white p-2">
            <HugeiconsIcon icon={TaskAdd01Icon} size={20} strokeWidth={1.6} />
          </span>
          <span className="flex flex-col gap-0.5">
            <span className="text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">
              Nouveau projet
            </span>
            <span className="text-[14px] leading-[1.5] text-[#73757c]">
              Nom et client suffisent pour démarrer.
            </span>
          </span>
        </button>
      }
    />
  )
}

/** Le tri quitte les en-tetes de colonne avec le tableau : il passe en menu. */
function SortMenu({
  sort,
  dir,
  onSort,
}: {
  sort: Sort
  dir: 'asc' | 'desc'
  onSort: (value: Sort) => void
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="lg" className="gap-1.5 text-[13px]">
          <HugeiconsIcon icon={ArrowUpDownIcon} size={16} strokeWidth={1.6} />
          <span className="text-[#111]">{SORT_LABELS[sort]}</span>
          <HugeiconsIcon
            icon={dir === 'asc' ? ArrowUp01Icon : ArrowDown01Icon}
            size={14}
            strokeWidth={1.8}
            className="text-[#8d8d8d]"
          />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        {SORTS.map((value) => (
          <DropdownMenuItem key={value} onSelect={() => onSort(value)}>
            {SORT_LABELS[value]}
            {sort === value && (
              <HugeiconsIcon
                icon={dir === 'asc' ? ArrowUp01Icon : ArrowDown01Icon}
                size={14}
                strokeWidth={1.8}
                className="ml-auto text-[#8d8d8d]"
              />
            )}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function ProjectsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  // Filtre, tri et pagination partent au serveur. La cle de cache derive des
  // parametres d'URL, donc revenir en arriere reaffiche la page precedente
  // sans la recharger.
  const params: ProjectListParams = {
    page: search.page,
    pageSize: PAGE_SIZE,
    search: search.search?.trim() === '' ? undefined : search.search,
    status: search.status,
    sort: search.sort,
    dir: search.dir,
  }

  const { data, isPending, isError, error } = useQuery(projectListQuery(params))
  const { data: clients } = useQuery(clientListQuery())

  const rows = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const page = search.page

  const statusOptions: Option[] = STATUS_ORDER.map((status) => ({
    value: status,
    label: STATUS[status].label,
    color: STATUS[status].color,
  }))

  const clientOptions: Option[] = (clients?.items ?? []).map((client) => ({
    value: client.id,
    label: client.name,
  }))

  const active = [search.status, search.client_id].filter(Boolean).length

  /** Tout changement de filtre ramene en page 1 : la page 2 d'une autre liste n'a pas de sens. */
  function setFilter(patch: { search?: string; status?: ProjectStatus; client_id?: string }) {
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
    >
      <div className="flex flex-col gap-3 p-4">
        {/* Barre d'outils : recherche, puis les filtres, puis l'action.
            L'ordre suit la lecture — on reduit la liste de gauche a droite,
            et ce qui la quitte (creer un projet) est au bout. */}
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
              placeholder="Rechercher un projet"
              aria-label="Rechercher un projet"
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
            value={search.client_id}
            options={clientOptions}
            onChange={(value) => setFilter({ client_id: value })}
          />

          {/* Pas de bouton « effacer » ici : chaque menu porte son entree
              « Tous les… », qui leve son filtre la ou on l'a pose. Un bouton
              de plus dans la barre disait la meme chose une troisieme fois. */}
          <SortMenu sort={search.sort} dir={search.dir} onSort={setSort} />

          <NewProjectDialog />
        </div>

        {isError && (
          <p className="rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
            {error instanceof HttpError ? error.message : 'Chargement impossible'}
          </p>
        )}

        {!isError && isPending && (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {Array.from({ length: 6 }, (_, index) => (
              <div
                key={index}
                className="min-h-[173px] animate-pulse rounded-[12px] border border-[#e8e8e9] bg-[#fafafa]"
              />
            ))}
          </div>
        )}

        {!isError && !isPending && (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {rows.map((project) => (
              <ProjectCard key={project.id} project={project} />
            ))}

            {/* La carte d'ajout ferme la liste : sur une page intermediaire
                elle se glisserait au milieu des projets. */}
            {page >= totalPages && <AddProjectCard />}
          </div>
        )}

        {!isError && !isPending && rows.length === 0 && (active > 0 || (search.search ?? '') !== '') && (
          <p className="text-center text-[13px] text-[#777]">
            Aucun projet ne correspond à ce filtre.
          </p>
        )}

        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-[12px] text-[#777]">
            {total} projet{total > 1 ? 's' : ''} · page {page} sur {totalPages}
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
