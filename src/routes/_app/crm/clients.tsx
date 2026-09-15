import { DashboardSquare01Icon, ListViewIcon, Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { TabBar, type Tab } from '@/components/layout/tab-bar'
import { Input } from '@/components/ui/input'
import { type CrmClientParams } from '@/features/clients/api'
import { CLIENT_STATUS, CLIENT_STATUS_ORDER } from '@/features/clients/format'
import { NewClientDialog } from '@/features/clients/new-client-dialog'
import { peopleQuery } from '@/features/projects/api'
import { useSearchField } from '@/lib/search-field'
import type { ClientStatus, Person } from '@/types/api'

/**
 * Ecran « Clients » du CRM.
 *
 * Deux vues sur le meme jeu de donnees : un tableau et un kanban commercial.
 * Les filtres vivent dans l'adresse et non dans un etat local — « les clients
 * en veille que je suis » est une vue qu'on partage, qu'on met en favori et ou
 * le bouton Retour ramene. C'est aussi ce qui permet aux deux vues de lire le
 * meme filtre : elles le tiennent de l'URL, pas l'une de l'autre.
 */
const searchSchema = z.object({
  search: z.string().optional(),
  statut: z.enum(['lead', 'devis', 'actif', 'veille', 'perdu']).optional().catch(undefined),
  charge: z.string().optional(),
  portail: z.enum(['avec', 'sans']).optional().catch(undefined),
  // `default` et non le seul `catch` : /crm redirige ici sans parametres.
  page: z.number().int().min(1).default(1).catch(1),
})

export type ClientsSearch = z.infer<typeof searchSchema>

/** Ce que la barre d'outils envoie au serveur, deduit de l'adresse. */
export function paramsOf(search: ClientsSearch): CrmClientParams {
  return {
    search: search.search?.trim() === '' ? undefined : search.search,
    status: search.statut,
    managerId: search.charge,
    // Absent, le filtre ne doit pas exister : envoyer `false` demanderait les
    // clients sans compte, ce qui n'est pas la meme chose que ne pas filtrer.
    hasPortal: search.portail === undefined ? undefined : search.portail === 'avec',
    page: search.page,
  }
}

export const Route = createFileRoute('/_app/crm/clients')({
  validateSearch: searchSchema,
  component: ClientsLayout,
})

const TABS: Tab[] = [
  { to: '/crm/clients', label: 'Liste', icon: ListViewIcon },
  { to: '/crm/clients/kanban', label: 'Pipeline', icon: DashboardSquare01Icon },
]

function ClientsLayout() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data: people } = useQuery(peopleQuery)

  // Tout changement de filtre ramene page 1 : rester en page 3 d'une liste qui
  // vient de se reduire n'afficherait rien.
  function setFilter(patch: Partial<Omit<ClientsSearch, 'page'>>) {
    // `replace` : un filtre affine la vue courante, il ne fait pas une
    // etape a part. Sans lui, chaque frappe et chaque case cochee laissait
    // une entree a repasser au retour arriere.
    void navigate({ search: (prev) => ({ ...prev, ...patch, page: 1 }), replace: true })
  }

  // La frappe est immediate a l'ecran, l'adresse ne suit qu'apres une pause.
  const [draft, setDraft] = useSearchField(search.search ?? '', (value) =>
    setFilter({ search: value === '' ? undefined : value }),
  )

  const statusOptions: Option[] = CLIENT_STATUS_ORDER.map((status) => ({
    value: status,
    label: CLIENT_STATUS[status].label,
    color: CLIENT_STATUS[status].color,
  }))

  const managerOptions: Option[] = (people?.items ?? []).map((person: Person) => ({
    value: person.id,
    label: `${person.firstname} ${person.lastname}`.trim() || 'Sans nom',
  }))

  return (
    <PageFrame title="Clients">
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
                placeholder="Rechercher un client ou un contact"
                aria-label="Rechercher un client ou un contact"
                className="h-9 pl-8 text-[13px]"
              />
            </div>

            <FilterMenu
              name="Statut"
              all="Tous les statuts"
              value={search.statut}
              options={statusOptions}
              onChange={(value) => setFilter({ statut: value as ClientStatus | undefined })}
            />

            <FilterMenu
              name="Chargé de compte"
              all="Tous"
              value={search.charge}
              options={managerOptions}
              onChange={(value) => setFilter({ charge: value })}
            />

            <FilterMenu
              name="Portail"
              all="Tous les clients"
              value={search.portail}
              options={[
                { value: 'avec', label: 'Avec accès' },
                { value: 'sans', label: 'Sans accès' },
              ]}
              onChange={(value) => setFilter({ portail: value as ClientsSearch['portail'] })}
            />

            <NewClientDialog />
          </div>
        </div>

        <div className="flex shrink-0 items-center border-b border-[#e8e8e9] pl-4">
          <TabBar tabs={TABS} layoutId="clients-tab" keepSearch />
        </div>

        <Outlet />
      </div>
    </PageFrame>
  )
}
