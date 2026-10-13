import { ArrowLeft01Icon, ArrowRight01Icon, FileChartLineIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { toast } from 'sonner'
import { z } from 'zod'

import { PageFrame } from '@/components/layout/page-frame'
import { PanelCard } from '@/components/panel-card'
import { Button } from '@/components/ui/button'
import { monthLabel, monthOf, shiftMonth } from '@/features/maintenance/format'
import { StatusPill } from '@/features/projects/ui'
import { reportListQuery, useGenerateReports, type ReportStatus } from '@/features/reports/api'
import { REPORT_STATUS } from '@/features/reports/format'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'

/**
 * Les rapports mensuels d'un mois : les brouillons a relire, ceux qui sont
 * prets, ceux qui sont partis. Les brouillons se creent seuls le 1er ; ce
 * bouton les cree tout de suite, pour un mois rate ou un projet ajoute.
 */
const searchSchema = z.object({
  mois: z.string().regex(/^\d{4}-\d{2}$/).optional().catch(undefined),
  filtre: z.enum(['draft', 'validated', 'sent']).optional().catch(undefined),
  page: z.number().int().min(1).optional().catch(undefined),
})

export const Route = createFileRoute('/_app/pm/rapports/')({
  validateSearch: searchSchema,
  component: ReportsPage,
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })

function previousMonth() {
  const d = new Date()
  return monthOf(new Date(d.getFullYear(), d.getMonth() - 1, 1))
}

function ReportsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const month = search.mois ?? previousMonth()
  const status: ReportStatus | null = search.filtre ?? null
  const page = search.page ?? 1
  const { data } = useQuery(reportListQuery(month, status, page))
  const generate = useGenerateReports()
  const pages = Math.max(1, Math.ceil((data?.total ?? 0) / 25))

  function go(next: { mois?: string; filtre?: ReportStatus | undefined; page?: number }) {
    void navigate({ search: (prev) => ({ ...prev, page: undefined, ...next }), replace: true })
  }

  const filters: { key: ReportStatus | null; label: string; count?: number }[] = [
    { key: null, label: 'Tous' },
    { key: 'draft', label: 'Brouillons', count: data?.counts.drafts },
    { key: 'validated', label: 'Validés', count: data?.counts.validated },
    { key: 'sent', label: 'Envoyés', count: data?.counts.sent },
  ]

  return (
    <PageFrame title="Rapports mensuels">
      <div className="flex flex-col gap-4 p-4">
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-1">
            <Button type="button" variant="outline" size="icon-sm" aria-label="Mois précédent" onClick={() => go({ mois: shiftMonth(month, -1) })}>
              <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
            </Button>
            <span className="min-w-[140px] text-center text-[15px] font-medium text-[#1b1b1b]">{monthLabel(month)}</span>
            <Button type="button" variant="outline" size="icon-sm" aria-label="Mois suivant" onClick={() => go({ mois: shiftMonth(month, 1) })}>
              <HugeiconsIcon icon={ArrowRight01Icon} size={16} strokeWidth={1.8} />
            </Button>
          </div>
          <div className="flex flex-wrap items-center gap-1">
            {filters.map((f) => (
              <button
                key={f.label}
                type="button"
                onClick={() => go({ filtre: f.key ?? undefined })}
                className={cn(
                  'flex h-8 items-center gap-1.5 rounded-full border px-3 text-[13px] transition-colors',
                  status === f.key ? 'border-[#1b1b1b] bg-[#1b1b1b] text-white' : 'border-[#e8e8e9] bg-white text-[#4b4b4f] hover:bg-[#f8f8f8]',
                )}
              >
                {f.label}
                {f.count !== undefined && <span className="tabular-nums opacity-70">{f.count}</span>}
              </button>
            ))}
          </div>
        </div>

        {data !== undefined && data.pending > 0 && (
          <div className="flex flex-wrap items-center gap-3 rounded-[12px] border border-[#ffe8b7] bg-[#fffaf0] px-4 py-3 text-[14px] text-[#9a6a00]">
            {data.pending} projet{data.pending > 1 ? 's' : ''} sous formule n’{data.pending > 1 ? 'ont' : 'a'} pas encore de rapport en {monthLabel(month).toLowerCase()}.
            <Button
              type="button"
              size="sm"
              className="ml-auto"
              disabled={generate.isPending}
              onClick={() =>
                generate.mutate(month, {
                  onSuccess: (r) => toast.success(`${r.created} brouillon${r.created > 1 ? 's' : ''} créé${r.created > 1 ? 's' : ''}`),
                  onError: (e) => toast.error(e instanceof HttpError ? e.message : 'Génération impossible'),
                })
              }
            >
              Générer les brouillons
            </Button>
          </div>
        )}

        <PanelCard icon={FileChartLineIcon} title="RAPPORTS" contentClassName="p-3">
          {data !== undefined && data.items.length === 0 ? (
            <p className="py-6 text-center text-[13px] text-[#73757c]">Aucun rapport pour {monthLabel(month).toLowerCase()}.</p>
          ) : (
            <ul className="flex flex-col">
              {(data?.items ?? []).map((r) => (
                <li key={r.id} className="border-b border-[#f3f4f4] last:border-b-0">
                  <Link
                    to="/pm/rapports/$id"
                    params={{ id: r.id }}
                    className="-mx-1.5 flex items-center gap-3 rounded-[8px] px-1.5 py-2.5 transition-colors hover:bg-[#fafafa]"
                  >
                    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                      <span className="truncate text-[14px] text-[#1b1b1b]">{r.project.name}</span>
                      <span className="truncate text-[12px] text-[#8d8d8d]">
                        {r.client_name !== '' && `${r.client_name} · `}formule {r.plan_name}
                        {r.has_editorial ? ' · mot du mois rédigé' : ''}
                      </span>
                    </span>
                    <span className="hidden text-[12px] text-[#73757c] sm:block">
                      {r.sent_at !== null ? `envoyé le ${WHEN.format(new Date(r.sent_at))}` : `généré le ${WHEN.format(new Date(r.generated_at))}`}
                    </span>
                    <StatusPill {...REPORT_STATUS[r.status]} />
                  </Link>
                </li>
              ))}
            </ul>
          )}
          {pages > 1 && (
            <div className="flex items-center justify-end gap-2 pt-2 text-[13px] text-[#73757c]">
              Page {page} sur {pages}
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => go({ page: page - 1 })}>
                Précédent
              </Button>
              <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => go({ page: page + 1 })}>
                Suivant
              </Button>
            </div>
          )}
        </PanelCard>
      </div>
    </PageFrame>
  )
}
