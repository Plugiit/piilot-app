import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Services.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/pm/services/')({
  component: ServicesPage,
})

function ServicesPage() {
  return (
    <PageFrame title="Services">
      <EmptyModule name="Services" />
    </PageFrame>
  )
}
