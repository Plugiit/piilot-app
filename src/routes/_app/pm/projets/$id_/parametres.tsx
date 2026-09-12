import {
  Alert02Icon,
  ArrowLeft02Icon,
  Calendar03Icon,
  Coins01Icon,
  Settings02Icon,
  UserGroupIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, notFound, Outlet, useNavigate } from '@tanstack/react-router'

import { PageFrame, type Crumb } from '@/components/layout/page-frame'
import { TabBar, type Tab } from '@/components/layout/tab-bar'
import { Button } from '@/components/ui/button'
import { projectDetailQuery } from '@/features/projects/api'
import { HttpError } from '@/lib/api'

/**
 * Onglets des reglages.
 *
 * La zone de danger est la derniere, et separee du reste par sa place autant
 * que par son nom : on n'y arrive pas en cherchant autre chose.
 */
/**
 * Retour a la liste, maillon commun aux ecrans d'un projet.
 *
 * Les parametres de recherche sont ceux de la liste au repos : sans eux, le
 * lien atterrirait sur une adresse incomplete que la route completerait par
 * ses valeurs de repli — le meme resultat, par un detour.
 */
const TOUS_LES_PROJETS: Crumb = {
  label: 'Tous les projets',
  to: '/pm/projets',
  search: { page: 1, sort: 'due', dir: 'asc' },
}

const TABS: Tab[] = [
  { to: '/pm/projets/$id/parametres', label: 'Général', icon: Settings02Icon },
  { to: '/pm/projets/$id/parametres/planning', label: 'Planning', icon: Calendar03Icon },
  { to: '/pm/projets/$id/parametres/budget', label: 'Budget', icon: Coins01Icon },
  { to: '/pm/projets/$id/parametres/equipe', label: 'Équipe', icon: UserGroupIcon },
  { to: '/pm/projets/$id/parametres/zone-de-danger', label: 'Zone de danger', icon: Alert02Icon },
]

export const Route = createFileRoute('/_app/pm/projets/$id_/parametres')({
  loader: async ({ context, params }) => {
    try {
      await context.queryClient.query({ ...projectDetailQuery(params.id), staleTime: 'static' })
    } catch (error) {
      if (error instanceof HttpError && error.status === 404) throw notFound()

      throw error
    }
  },
  component: SettingsLayout,
})

/**
 * Chassis des reglages : en-tete et onglets, communs aux cinq sections.
 */
function SettingsLayout() {
  const { id } = Route.useParams()
  const navigate = useNavigate()
  const { data: project } = useQuery(projectDetailQuery(id))

  if (project === undefined) return null

  return (
    <PageFrame
      title="Paramètres"
      trail={[
        TOUS_LES_PROJETS,
        { label: project.name, to: '/pm/projets/$id', params: { id } },
      ]}
    >
      <div className="flex flex-col">
        <div className="flex items-center gap-4 p-4">
          <Button
            type="button"
            variant="outline"
            size="icon-lg"
            aria-label="Revenir au projet"
            onClick={() => void navigate({ to: '/pm/projets/$id', params: { id } })}
          >
            <HugeiconsIcon icon={ArrowLeft02Icon} size={20} strokeWidth={1.8} />
          </Button>

          <div className="flex min-w-0 flex-col gap-0.5">
            <h1 className="truncate text-[20px] leading-[1.4] font-medium text-[#1b1b1b]">
              Paramètres
            </h1>
            <p className="truncate text-[16px] leading-[1.5] text-[#73757c]">
              Tout ce qui se règle sur « {project.name} ».
            </p>
          </div>
        </div>

        {/* Pas d'`overflow-x-auto` ici : quand un axe cesse d'etre `visible`,
            l'autre passe a `auto` par la meme occasion. Le filet de l'onglet
            actif, pose a `-bottom-px`, depassait alors d'un pixel et rendait la
            barre defilante verticalement — imperceptible a l'oeil, les barres
            etant masquees partout, mais sensible au doigt. Les cinq onglets
            tiennent avec 67px de marge a 1024px, la plus petite largeur ou le
            panneau est affiche. */}
        <div className="flex items-center border-b border-[#e8e8e9] pl-4">
          <TabBar tabs={TABS} params={{ id }} layoutId="project-settings-tab" />
        </div>

        <Outlet />
      </div>
    </PageFrame>
  )
}
