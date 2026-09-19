import { DashboardSquare01Icon, ListViewIcon, PlusSignIcon, Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { TabBar, type Tab } from '@/components/layout/tab-bar'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { projectListQuery } from '@/features/projects/api'
import { TASK_STATUS, TASK_STATUS_ORDER } from '@/features/projects/format'
import { taskListQuery, type TaskListParams } from '@/features/tasks/api'
import { NewTaskDialog } from '@/features/tasks/new-task-dialog'
import { useSearchField } from '@/lib/search-field'
import type { TaskStatus } from '@/types/api'

/**
 * Ecran « Tâches » du module.
 *
 * Les filtres vivent dans l'adresse et non dans un etat local : « les tâches
 * en revue du portail client » est une vue qu'on partage, qu'on met en favori
 * et ou le bouton Retour ramene. C'est aussi ce qui permet aux deux vues de
 * lire le meme filtre — elles le tiennent de l'URL, pas l'une de l'autre.
 *
 * `tache` est declare ici et non dans les vues : le tiroir s'ouvre depuis la
 * liste comme depuis le kanban, et changer d'onglet ne doit pas le refermer.
 */
const searchSchema = z.object({
  search: z.string().optional(),
  status: z.enum(['todo', 'progress', 'review', 'done']).optional(),
  projet: z.string().optional(),
  tache: z.string().optional(),
})

/** Ce que la barre d'outils envoie au serveur, deduit de l'adresse. */
export function paramsOf(search: z.infer<typeof searchSchema>): TaskListParams {
  return {
    search: search.search?.trim() === '' ? undefined : search.search,
    status: search.status,
    projectId: search.projet,
  }
}

export const Route = createFileRoute('/_app/pm/taches')({
  validateSearch: searchSchema,
  // Seuls les filtres declenchent un rechargement : ouvrir une tache change
  // l'adresse sans rien changer a la liste.
  loaderDeps: ({ search }) => paramsOf(search),
  loader: ({ context, deps }) =>
    context.queryClient.query({ ...taskListQuery(deps), staleTime: 'static' }),
  component: TasksLayout,
})

const TABS: Tab[] = [
  { to: '/pm/taches', label: 'Liste', icon: ListViewIcon },
  { to: '/pm/taches/kanban', label: 'Kanban', icon: DashboardSquare01Icon },
]

/**
 * Chassis de l'ecran : la barre d'outils et les onglets, que les deux vues
 * partagent. Meme barre que la liste des projets — recherche, filtres, puis
 * l'action qui cree —, aux filtres pres : ici on reduit par statut et par
 * projet.
 */
function TasksLayout() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data: list } = useQuery(taskListQuery(paramsOf(search)))
  const { data: projects } = useQuery(
    projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }),
  )

  function setFilter(patch: { search?: string; status?: TaskStatus; projet?: string }) {
    // `replace` : un filtre affine la vue courante, il ne fait pas une
    // etape a part. Sans lui, chaque frappe et chaque case cochee laissait
    // une entree a repasser au retour arriere.
    void navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true })
  }

  // La frappe est immediate a l'ecran, l'adresse ne suit qu'apres une pause.
  const [draft, setDraft] = useSearchField(search.search ?? '', (value) =>
    setFilter({ search: value === '' ? undefined : value }),
  )

  const statusOptions: Option[] = TASK_STATUS_ORDER.map((status) => ({
    value: status,
    label: TASK_STATUS[status].label,
    color: TASK_STATUS[status].color,
  }))

  const projectOptions: Option[] = (projects?.items ?? []).map((project) => ({
    value: project.id,
    label: project.name,
  }))

  return (
    <PageFrame title="Tâches">
      <div className="flex min-h-full flex-col">
        <div className="flex flex-col gap-3 p-4">
          {/* Barre d'outils : recherche, puis les filtres, puis l'action.
              L'ordre suit la lecture — on reduit la liste de gauche a droite,
              et ce qui l'augmente est au bout. */}
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
                placeholder="Rechercher une tâche"
                aria-label="Rechercher une tâche"
                className="h-9 pl-8 text-[13px]"
              />
            </div>

            <FilterMenu
              name="Statut"
              all="Tous les statuts"
              value={search.status}
              options={statusOptions}
              onChange={(value) => setFilter({ status: value as TaskStatus | undefined })}
            />

            <FilterMenu
              name="Projet"
              all="Tous les projets"
              value={search.projet}
              options={projectOptions}
              onChange={(value) => setFilter({ projet: value })}
            />

            {/* Le projet n'est pas connu ici : le dialogue le demande. */}
            <NewTaskDialog
              trigger={
                <Button size="lg" className="gap-1.5">
                  <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
                  Nouvelle tâche
                </Button>
              }
            />
          </div>

          {list !== undefined && list.total > list.limit && (
            <p className="text-[12px] text-[#73757c]">
              {list.total} tâches correspondent, {list.limit} affichées. Affinez les filtres pour
              voir le reste.
            </p>
          )}
        </div>

        <div className="flex shrink-0 items-center border-b border-[#e8e8e9] pl-4">
          <TabBar tabs={TABS} layoutId="tasks-tab" keepSearch />
        </div>

        <Outlet />
      </div>
    </PageFrame>
  )
}
