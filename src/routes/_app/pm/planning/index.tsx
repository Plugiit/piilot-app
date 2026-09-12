import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Planning.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/pm/planning/')({
  component: PlanningPage,
})

function PlanningPage() {
  return (
    <PageFrame title="Planning">
      <EmptyModule name="Planning" />
    </PageFrame>
  )
}
