import { ArrowRight01Icon, PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { portalTicketsQuery } from '@/features/portal/api'
import { portalTicketStatus, since } from '@/features/portal/format'
import { StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'

/**
 * Support : les demandes du client.
 *
 * Deux onglets plutot qu'un filtre : « en cours » est ce qu'on vient regarder,
 * « resolues » ce qu'on vient retrouver. Les deux vivent dans l'adresse.
 */
const searchSchema = z.object({
  etat: z.enum(['en-cours', 'resolues']).optional().catch(undefined),
  page: z.number().int().min(1).optional().catch(undefined),
})

export const Route = createFileRoute('/client/tickets/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ state: search.etat === 'resolues' ? 'closed' : 'open', page: search.page ?? 1 }),
  loader: ({ context, deps }) =>
    context.queryClient.ensureQueryData(portalTicketsQuery(deps.state as 'open' | 'closed', deps.page)),
  component: ClientTicketsPage,
})

function ClientTicketsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const state = search.etat === 'resolues' ? 'closed' : 'open'
  const page = search.page ?? 1
  const { data, isError, error } = useQuery(portalTicketsQuery(state, page))
  const items = data?.items ?? []
  const pages = data === undefined ? 1 : Math.max(1, Math.ceil(data.total / data.page_size))

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="font-heading text-[26px] leading-tight font-medium text-[#1b1b1b]">Support</h1>
          <p className="text-[15px] text-[#73757c]">Vos demandes à l’agence, et leurs réponses.</p>
        </div>
        <Button asChild size="lg" className="h-11 gap-1.5">
          <Link to="/client/tickets/nouvelle">
            <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
            Nouvelle demande
          </Link>
        </Button>
      </header>

      <div role="tablist" aria-label="Demandes" className="flex gap-5 border-b border-[#e8e8e9]">
        {(['en-cours', 'resolues'] as const).map((etat) => {
          const active = (search.etat ?? 'en-cours') === etat
          return (
            <button
              key={etat}
              type="button"
              role="tab"
              aria-selected={active}
              onClick={() => void navigate({ search: { etat: etat === 'en-cours' ? undefined : etat } })}
              className={cn(
                'relative cursor-pointer pb-2.5 text-[14px] transition-colors',
                active
                  ? 'font-medium text-[#1b1b1b] after:absolute after:inset-x-0 after:-bottom-px after:h-0.5 after:rounded-full after:bg-brand'
                  : 'text-[#73757c] hover:text-[#1b1b1b]',
              )}
            >
              {etat === 'en-cours' ? 'En cours' : 'Résolues et fermées'}
            </button>
          )
        })}
      </div>

      {isError && (
        <p className="rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[14px] text-[#e5484d]">
          {error instanceof HttpError ? error.message : 'Chargement impossible'}
        </p>
      )}

      {data !== undefined && items.length === 0 && (
        <p className="rounded-[14px] border border-dashed border-[#e3e4e6] bg-white p-8 text-center text-[15px] text-[#73757c]">
          {state === 'open'
            ? 'Aucune demande en cours. Un problème, une question ? Écrivez-nous ici plutôt que par e-mail : tout le suivi reste au même endroit.'
            : 'Aucune demande résolue pour l’instant.'}
        </p>
      )}

      {items.length > 0 && (
        <ul className="flex flex-col overflow-hidden rounded-[14px] border border-[#e8e8e9] bg-white">
          {items.map((ticket) => {
            const status = portalTicketStatus(ticket.status)
            return (
              <li key={ticket.id} className="border-b border-[#f3f4f4] last:border-b-0">
                <Link
                  to="/client/tickets/$id"
                  params={{ id: ticket.id }}
                  className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-[#fafafa]"
                >
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex items-baseline gap-2">
                      <span className="shrink-0 text-[13px] text-[#a2a3a7] tabular-nums">#{ticket.numero}</span>
                      <span className="truncate text-[15px] text-[#1b1b1b]">{ticket.subject}</span>
                    </span>
                    <span className="truncate text-[12px] text-[#8d8d8d]">
                      {ticket.project.name} · mise à jour {since(ticket.updated_at)}
                    </span>
                  </span>
                  <StatusPill label={status.label} color={status.color} pill={status.pill} className="hidden sm:inline-flex" />
                  <HugeiconsIcon icon={ArrowRight01Icon} size={16} strokeWidth={1.8} className="shrink-0 text-[#a2a3a7]" />
                </Link>
              </li>
            )
          })}
        </ul>
      )}

      {pages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => void navigate({ search: (prev) => ({ ...prev, page: page - 1 }) })}
          >
            Précédentes
          </Button>
          <span className="text-[13px] text-[#73757c] tabular-nums">
            {page} / {pages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= pages}
            onClick={() => void navigate({ search: (prev) => ({ ...prev, page: page + 1 }) })}
          >
            Suivantes
          </Button>
        </div>
      )}
    </div>
  )
}
