import { ArrowLeft01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { portalInterventionsQuery } from '@/features/maintenance/api'
import { monthLabel, monthOf } from '@/features/maintenance/format'
import { ConsumptionBar, KindPill } from '@/features/maintenance/ui'
import { portalProjectQuery } from '@/features/portal/api'
import { portalReportsQuery } from '@/features/reports/api'
import { formatDuration } from '@/features/time/format'

/**
 * La maintenance d'un projet, pour le client : ce que l'agence a fait sur
 * son site, et ou en est le temps inclus de sa formule ce mois-ci.
 */
export const Route = createFileRoute('/client/projets_/$id/maintenance')({
  validateSearch: z.object({ page: z.number().int().min(1).optional().catch(undefined) }),
  loader: ({ context, params }) => context.queryClient.ensureQueryData(portalProjectQuery(params.id)),
  component: MaintenancePage,
})

const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })

function MaintenancePage() {
  const { id } = Route.useParams()
  const { page = 1 } = Route.useSearch()
  const navigate = Route.useNavigate()
  const { data: project } = useQuery(portalProjectQuery(id))
  const { data, error } = useQuery(portalInterventionsQuery(id, page))
  const pages = Math.max(1, Math.ceil((data?.total ?? 0) / 20))

  if (project === undefined) return null

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3">
        <Link to="/client/projets/$id" params={{ id }} className="flex items-center gap-1 text-[14px] text-[#73757c] hover:text-[#1b1b1b]">
          <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
          {project.name}
        </Link>
        <h1 className="font-heading text-[24px] leading-tight font-medium text-[#1b1b1b]">Maintenance</h1>
      </div>

      {error !== null && (
        <p className="rounded-[14px] border border-[#e8e8e9] bg-white p-4 text-[14px] text-[#4b4b4f]">La maintenance de ce projet n’est pas disponible.</p>
      )}

      <Reports projectId={id} />

      {data?.current_month !== null && data?.current_month !== undefined && (
        <section className="flex flex-col gap-3 rounded-[14px] border border-[#e8e8e9] bg-white p-4">
          <header className="flex flex-wrap items-baseline justify-between gap-2">
            <h2 className="text-[15px] font-medium text-[#1b1b1b]">{monthLabel(monthOf(new Date()))}</h2>
            <span className="text-[13px] text-[#73757c]">Formule {data.current_month.plan_name}</span>
          </header>
          <ConsumptionBar consumed={data.current_month.consumed_minutes} included={data.current_month.included_minutes} />
          {data.current_month.consumed_minutes > data.current_month.visible_minutes && (
            <p className="text-[12px] text-[#73757c]">
              Dont {formatDuration(data.current_month.consumed_minutes - data.current_month.visible_minutes)} d’interventions techniques internes, non détaillées ci-dessous.
            </p>
          )}
          <p className="text-[12px] text-[#8d8d8d]">Le temps inclus vaut pour le mois : il ne se reporte pas sur le suivant.</p>
        </section>
      )}

      <section className="flex flex-col gap-3 rounded-[14px] border border-[#e8e8e9] bg-white p-4">
        <h2 className="text-[15px] font-medium text-[#1b1b1b]">Interventions</h2>
        {data !== undefined && data.items.length === 0 ? (
          <p className="py-6 text-center text-[14px] text-[#73757c]">Aucune intervention pour l’instant.</p>
        ) : (
          <ul className="flex flex-col">
            {(data?.items ?? []).map((i) => (
              <li key={i.id} className="flex flex-wrap items-start gap-x-3 gap-y-1 border-b border-[#f3f4f4] py-3 last:border-b-0">
                <span className="flex min-w-0 flex-1 flex-col gap-1">
                  <span className="text-[14px] text-[#1b1b1b]">{i.note}</span>
                  <span className="text-[12px] text-[#8d8d8d]">{DAY.format(new Date(i.spent_on))}</span>
                </span>
                <KindPill kind={i.kind} />
                <span className="w-[56px] text-right text-[13px] text-[#4b4b4f] tabular-nums">{formatDuration(i.minutes)}</span>
              </li>
            ))}
          </ul>
        )}
        {pages > 1 && (
          <div className="flex items-center justify-end gap-2 text-[13px] text-[#73757c]">
            Page {page} sur {pages}
            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => void navigate({ search: { page: page - 1 } })}>
              Précédent
            </Button>
            <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => void navigate({ search: { page: page + 1 } })}>
              Suivant
            </Button>
          </div>
        )}
      </section>
    </div>
  )
}

const SENT = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long' })

/** Les rapports mensuels recus, du plus recent au plus ancien. */
function Reports({ projectId }: { projectId: string }) {
  const { data } = useQuery(portalReportsQuery(projectId))
  if (data === undefined || data.items.length === 0) return null
  return (
    <section className="flex flex-col gap-3 rounded-[14px] border border-[#e8e8e9] bg-white p-4">
      <h2 className="text-[15px] font-medium text-[#1b1b1b]">Rapports mensuels</h2>
      <ul className="flex flex-col">
        {data.items.map((r) => (
          <li key={r.id} className="border-b border-[#f3f4f4] last:border-b-0">
            <Link to="/client/rapports/$id" params={{ id: r.id }} className="-mx-1.5 flex items-center gap-3 rounded-[8px] px-1.5 py-2.5 transition-colors hover:bg-[#fafafa]">
              <span className="min-w-0 flex-1 text-[14px] text-[#1b1b1b]">{monthLabel(r.month)}</span>
              {r.sent_at !== null && <span className="text-[12px] text-[#8d8d8d]">reçu le {SENT.format(new Date(r.sent_at))}</span>}
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}
