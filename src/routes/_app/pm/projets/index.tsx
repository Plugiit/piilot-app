import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Projets.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/pm/projets/')({
  component: ProjetsPage,
})

function ProjetsPage() {
  return (
    <PageFrame title="Projets" description="Tous les projets de l’agence.">
      <EmptyModule name="Projets" />
    </PageFrame>
  )
}
