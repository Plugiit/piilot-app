import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { Card } from '@/components/settings-ui'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ErasuresList } from '@/features/rgpd/erasures-list'
import { ExportsList } from '@/features/rgpd/exports-list'
import { ERASURE_STATUS_LABEL, ERASURE_STATUSES } from '@/features/rgpd/labels'
import { can } from '@/lib/auth'

/**
 * RGPD : exports de données et effacements.
 *
 * Les demandes se font depuis la fiche d'un client ou l'écran des comptes ;
 * cet écran en garde l'historique, permet de télécharger les archives prêtes
 * et d'annuler un effacement tant qu'il est planifié. L'onglet vit dans
 * l'adresse : on peut en partager le lien.
 */
const searchSchema = z.object({
  tab: z.enum(['exports', 'erasures']).catch('exports'),
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

function RgpdPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  return (
    <PageFrame title="RGPD">
      <div className="flex flex-col gap-3 p-4">
        <Card
          title={search.tab === 'exports' ? 'Exports de données' : 'Effacements'}
          description={
            search.tab === 'exports'
              ? 'Archives zip des données d’un client ou d’un compte. Elles restent téléchargeables quelques jours.'
              : 'Un effacement s’exécute à la fin du délai de rétractation et peut être annulé jusque-là.'
          }
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <Tabs value={search.tab} onValueChange={(tab) => void navigate({ search: { tab: tab as 'exports' | 'erasures', page: 1 } })}>
              <TabsList className="h-8">
                <TabsTrigger value="exports" className="px-2.5 text-[12px]">
                  Exports
                </TabsTrigger>
                <TabsTrigger value="erasures" className="px-2.5 text-[12px]">
                  Effacements
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
