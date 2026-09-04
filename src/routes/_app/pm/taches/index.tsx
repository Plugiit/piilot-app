import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Tâches.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/pm/taches/')({
  component: TachesPage,
})

function TachesPage() {
  return (
    <PageFrame title="Tâches" description="Les tâches en cours, tous projets confondus.">
      <EmptyModule name="Tâches" />
    </PageFrame>
  )
}
