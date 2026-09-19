import {
  DashboardSquare01Icon,
  FolderOpenIcon,
  ListViewIcon,
  Search01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { TabBar, type Tab } from '@/components/layout/tab-bar'
import { Input } from '@/components/ui/input'
import { projectListQuery } from '@/features/projects/api'
import { type TicketFilters } from '@/features/tickets/api'
import {
  TICKET_PRIORITY,
  TICKET_PRIORITY_ORDER,
  TICKET_STATUS,
  TICKET_STATUS_ORDER,
  TICKET_TRACKER,
  TICKET_TRACKER_ORDER,
} from '@/features/tickets/format'
import { NewTicketDialog } from '@/features/tickets/new-ticket-dialog'
import { useSearchField } from '@/lib/search-field'
import type { TicketPriority, TicketStatus, TicketTracker } from '@/types/api'

/**
 * Ecran « Tickets » du module.
 *
 * Trois vues des memes tickets : le tableau, qui les detaille et se pagine, et
 * deux kanbans qui les repartissent — par projet pour voir ou se concentre la
 * charge, par statut pour voir ou elle bloque.
 *
 * Les filtres vivent dans l'adresse et non dans un etat local : « mes anomalies
 * critiques du portail client » est une vue qu'on partage, qu'on met en favori
 * et ou le bouton Retour ramene. C'est aussi ce qui permet aux trois vues de
 * lire le meme filtre — elles le tiennent de l'URL, pas l'une de l'autre.
 */
const searchSchema = z.object({
  page: z.number().int().min(1).catch(1),
  search: z.string().optional(),
  status: z
    .enum(['backlog', 'todo', 'in_progress', 'in_review', 'ready_to_deploy', 'done', 'annule'])
    .optional()
    .catch(undefined),
  tracker: z.enum(['anomalie', 'evolution', 'assistance']).optional().catch(undefined),
  priority: z.enum(['low', 'normal', 'high', 'urgent', 'critical']).optional().catch(undefined),
  projet: z.string().optional(),
})

type TicketsSearch = z.infer<typeof searchSchema>

/** Ce que les trois vues passent a leur requete. */
export function filtersOf(search: TicketsSearch): TicketFilters {
  return {
    search: search.search?.trim() === '' ? undefined : search.search,
    status: search.status,
    tracker: search.tracker,
    priority: search.priority,
    projectId: search.projet,
  }
}

export const Route = createFileRoute('/_app/pm/tickets')({
  validateSearch: searchSchema,
  component: TicketsLayout,
})

const TABS: Tab[] = [
  { to: '/pm/tickets', label: 'Tableau', icon: ListViewIcon },
  { to: '/pm/tickets/projets', label: 'Par projet', icon: FolderOpenIcon },
  { to: '/pm/tickets/statuts', label: 'Par statut', icon: DashboardSquare01Icon },
]

/**
 * Chassis de l'ecran : la barre d'outils et les onglets, que les trois vues
 * partagent. Meme barre que la liste des projets et l'ecran des taches —
 * recherche, filtres, puis l'action qui cree.
 */
function TicketsLayout() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data: projects } = useQuery(
    projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }),
  )

  // Tout changement de filtre ramene page 1 : rester en page 3 d'une liste qui
  // vient de se reduire n'afficherait rien.
  //
  // `replace` : un filtre affine la vue courante, il ne fait pas une etape a
  // part. Sans lui, chaque case cochee laissait une entree a repasser au
  // retour arriere.
  function setFilter(patch: Partial<Omit<TicketsSearch, 'page'>>) {
    void navigate({ search: (prev) => ({ ...prev, ...patch, page: 1 }), replace: true })
  }

  // La frappe est immediate a l'ecran, l'adresse ne suit qu'apres une pause.
  const [draft, setDraft] = useSearchField(search.search ?? '', (value) =>
    setFilter({ search: value === '' ? undefined : value }),
  )

  const statusOptions: Option[] = TICKET_STATUS_ORDER.map((status) => ({
    value: status,
    label: TICKET_STATUS[status]!.label,
    color: TICKET_STATUS[status]!.pill.text,
  }))

  const trackerOptions: Option[] = TICKET_TRACKER_ORDER.map((tracker) => ({
    value: tracker,
    label: TICKET_TRACKER[tracker]!.label,
    color: TICKET_TRACKER[tracker]!.pill.text,
  }))

  const priorityOptions: Option[] = TICKET_PRIORITY_ORDER.map((priority) => ({
    value: priority,
    label: TICKET_PRIORITY[priority]!.label,
    color: TICKET_PRIORITY[priority]!.pill.text,
  }))

  const projectOptions: Option[] = (projects?.items ?? []).map((project) => ({
    value: project.id,
    label: project.name,
  }))

  return (
    <PageFrame title="Tickets">
      <div className="flex min-h-full flex-col">
        {/* Barre d'outils : recherche, puis les filtres, puis l'action.
            L'ordre suit la lecture — on reduit la liste de gauche a droite, et
            ce qui l'augmente est au bout. */}
        <div className="flex flex-col gap-3 p-4">
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
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                placeholder="Rechercher un sujet ou un numéro"
                aria-label="Rechercher un ticket"
                className="h-9 pl-8 text-[13px]"
              />
            </div>

            <FilterMenu
              name="Statut"
              all="Tous les statuts"
              value={search.status}
              options={statusOptions}
              onChange={(value) => setFilter({ status: value as TicketStatus | undefined })}
            />

            <FilterMenu
              name="Tracker"
              all="Tous les trackers"
              value={search.tracker}
              options={trackerOptions}
              onChange={(value) => setFilter({ tracker: value as TicketTracker | undefined })}
            />

            <FilterMenu
              name="Priorité"
              all="Toutes les priorités"
              value={search.priority}
              options={priorityOptions}
              onChange={(value) => setFilter({ priority: value as TicketPriority | undefined })}
            />

            <FilterMenu
              name="Projet"
              all="Tous les projets"
              value={search.projet}
              options={projectOptions}
              onChange={(value) => setFilter({ projet: value })}
            />

            <NewTicketDialog />
          </div>
        </div>

        <div className="flex shrink-0 items-center border-b border-[#e8e8e9] pl-4">
          <TabBar tabs={TABS} layoutId="tickets-tab" keepSearch />
        </div>

        <Outlet />
      </div>
    </PageFrame>
  )
}
