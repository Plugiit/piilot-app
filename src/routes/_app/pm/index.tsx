import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Module Project Management.
 *
 * L'ecran est vide : le modele de donnees du module reste a concevoir, et
 * aucun endpoint ne le sert. La route existe pour que le rail ait une
 * destination — le contenu viendra avec les tables.
 */
export const Route = createFileRoute('/_app/pm/')({
  component: ProjectManagementPage,
})

function ProjectManagementPage() {
  return (
    <PageFrame title="Tableau de bord" description="Projets, jalons, temps passé et livrables.">
      <EmptyModule name="Tableau de bord" />
    </PageFrame>
  )
}
