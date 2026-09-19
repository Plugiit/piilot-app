import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Interactions.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/crm/interactions/')({
  component: InteractionsPage,
})

function InteractionsPage() {
  return (
    <PageFrame title="Interactions">
      <EmptyModule name="Interactions" />
    </PageFrame>
  )
}
