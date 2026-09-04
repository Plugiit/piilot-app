import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Clients.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/crm/clients/')({
  component: ClientsPage,
})

function ClientsPage() {
  return (
    <PageFrame title="Clients" description="Les comptes clients de l’agence.">
      <EmptyModule name="Clients" />
    </PageFrame>
  )
}
