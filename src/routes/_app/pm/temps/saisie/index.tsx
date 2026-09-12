import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Saisie du temps.
 *
 * L'ecran attend son modele de donnees : la route existe pour que la
 * navigation soit complete, le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/pm/temps/saisie/')({
  component: SaisieTempsPage,
})

function SaisieTempsPage() {
  return (
    <PageFrame title="Saisie du temps">
      <EmptyModule name="Saisie du temps" />
    </PageFrame>
  )
}
