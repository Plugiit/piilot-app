import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { Card } from '@/components/settings-ui'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { DeletedClientsList } from '@/features/rgpd/deleted-clients-list'
import { ErasuresList } from '@/features/rgpd/erasures-list'
import { ExportsList } from '@/features/rgpd/exports-list'
import { ERASURE_STATUS_LABEL, ERASURE_STATUSES } from '@/features/rgpd/labels'
import { can } from '@/lib/auth'

/**
 * RGPD : exports de données et effacements.
 *
 * Les demandes se font depuis la fiche d'un client ou l'écran des comptes ;
 * cet écran en garde l'historique, permet de télécharger les archives prêtes
 * et d'annuler un effacement tant qu'il est planifié. Les clients supprimés du
 * CRM (suppression logique, données conservées) se retrouvent dans leur propre
 * onglet pour être exportés ou effacés. L'onglet vit dans
 * l'adresse : on peut en partager le lien.
 */
const searchSchema = z.object({
  tab: z.enum(['exports', 'erasures', 'deleted']).catch('exports'),
  page: z.number().int().min(1).catch(1),
  status: z.enum(['scheduled', 'cancelled', 'running', 'done', 'failed']).optional().catch(undefined),
})

export const Route = createFileRoute('/_app/parametres/rgpd/')({
  validateSearch: searchSchema,
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'rgpd.manage')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  component: RgpdPage,
})

const STATUS_OPTIONS: Option[] = ERASURE_STATUSES.map((value) => ({ value, label: ERASURE_STATUS_LABEL[value] }))

const TITLES = { exports: 'Exports de données', erasures: 'Effacements', deleted: 'Clients supprimés du CRM' } as const

const DESCRIPTIONS = {
  exports: 'Archives zip des données d’un client ou d’un compte. Elles restent téléchargeables quelques jours.',
  erasures: 'Un effacement s’exécute à la fin du délai de rétractation et peut être annulé jusque-là.',
  deleted:
    'Un client supprimé du CRM n’est pas effacé : ses données sont conservées. Exportez-les ou demandez leur effacement ici.',
} as const

function RgpdPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  return (
    <PageFrame title="RGPD">
      <div className="flex flex-col gap-3 p-4">
        <Card
          title={TITLES[search.tab]}
          description={DESCRIPTIONS[search.tab]}
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <Tabs value={search.tab} onValueChange={(tab) => void navigate({ search: { tab: tab as 'exports' | 'erasures' | 'deleted', page: 1 } })}>
              <TabsList className="h-8">
                <TabsTrigger value="exports" className="px-2.5 text-[12px]">
                  Exports
                </TabsTrigger>
                <TabsTrigger value="erasures" className="px-2.5 text-[12px]">
                  Effacements
                </TabsTrigger>
                <TabsTrigger value="deleted" className="px-2.5 text-[12px]">
                  Clients supprimés
                </TabsTrigger>
              </TabsList>
            </Tabs>

            {search.tab === 'erasures' && (
              <FilterMenu
                name="Statut"
                all="Tous les statuts"
                value={search.status}
                options={STATUS_OPTIONS}
                onChange={(value) => void navigate({ search: { tab: 'erasures', page: 1, status: value as typeof search.status }, replace: true })}
              />
            )}
          </div>

          {search.tab === 'exports' ? (
            <ExportsList page={search.page} onPage={(page) => void navigate({ search: { tab: 'exports', page } })} />
          ) : search.tab === 'deleted' ? (
            <DeletedClientsList page={search.page} onPage={(page) => void navigate({ search: { tab: 'deleted', page } })} />
          ) : (
            <ErasuresList
              status={search.status}
              page={search.page}
              onPage={(page) => void navigate({ search: { tab: 'erasures', page, status: search.status } })}
            />
          )}
        </Card>
      </div>
    </PageFrame>
  )
}
