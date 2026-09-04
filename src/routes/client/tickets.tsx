import { createFileRoute } from '@tanstack/react-router'

import { PageHeader } from '@/components/layout/page-header'

export const Route = createFileRoute('/client/tickets')({
  component: ClientTicketsPage,
})

function ClientTicketsPage() {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Support" subtitle="Vos demandes en cours" />

      <div className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6">
        <p className="text-sm text-ink-muted">
          Les tickets arriveront ici, une fois le module écrit côté API.
        </p>
      </div>
    </div>
  )
}
