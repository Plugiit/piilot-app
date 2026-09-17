import { createFileRoute } from '@tanstack/react-router'

import { EmptyModule } from '@/components/layout/empty-module'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Comptes et roles.
 *
 * Le socle porte deja les permissions — `users.read`, `roles.write` et les
 * autres vivent en base depuis la migration RBAC — mais aucun endpoint ne les
 * expose : on cree un compte par `make seed`. L'ecran attend ces endpoints.
 */
export const Route = createFileRoute('/_app/parametres/comptes/')({
  component: ComptesPage,
})

function ComptesPage() {
  return (
    <PageFrame title="Comptes et rôles">
      <EmptyModule name="Comptes et rôles" />
    </PageFrame>
  )
}
