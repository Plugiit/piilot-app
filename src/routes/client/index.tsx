import { createFileRoute } from '@tanstack/react-router'

import { PageHeader } from '@/components/layout/page-header'

export const Route = createFileRoute('/client/')({
  component: ClientProjectsPage,
})

function ClientProjectsPage() {
  const { user } = Route.useRouteContext()

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Mes projets" subtitle={`Bonjour ${user.firstname}`} />

      <div className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6">
        <p className="text-sm text-muted-foreground">
          Le suivi de projet arrivera ici, une fois le module PM écrit côté API.
        </p>
      </div>
    </div>
  )
}
