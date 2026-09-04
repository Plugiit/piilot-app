import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Modèles de projet.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/pm/modeles/')({
  component: ModelesPage,
})

function ModelesPage() {
  return (
    <PageFrame title="Modèles de projet" description="Structures de projet réutilisables.">
      <EmptyModule name="Modèles de projet" />
    </PageFrame>
  )
}
