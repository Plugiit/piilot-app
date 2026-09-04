import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Livrables.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/pm/livrables/')({
  component: LivrablesPage,
})

function LivrablesPage() {
  return (
    <PageFrame title="Livrables" description="Les livrables et leur validation.">
      <EmptyModule name="Livrables" />
    </PageFrame>
  )
}
