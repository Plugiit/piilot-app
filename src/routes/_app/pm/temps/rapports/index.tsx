import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Rapports de temps.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/pm/temps/rapports/')({
  component: RapportsTempsPage,
})

function RapportsTempsPage() {
  return (
    <PageFrame
      title="Rapports de temps"
    >
      <EmptyModule name="Rapports de temps" />
    </PageFrame>
  )
}
