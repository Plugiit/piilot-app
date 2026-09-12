import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Tickets.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/pm/tickets/')({
  component: TicketsPage,
})

function TicketsPage() {
  return (
    <PageFrame title="Tickets">
      <EmptyModule name="Tickets" />
    </PageFrame>
  )
}
