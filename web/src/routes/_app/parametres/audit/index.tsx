import { Download04Icon, Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'

import { FilterMenu } from '@/components/filter-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { AUDIT_ACTION, AUDIT_FAMILIES, auditExportUrl, auditQuery, type AuditFilters } from '@/features/audit/api'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import { useSearchField } from '@/lib/search-field'
import type { AuditItem } from '@/types/api'

/**
 * Le journal d'audit : qui a fait quoi, quand, d'ou. En lecture seule — la
 * base refuse qu'une ligne soit modifiee — et exportable en CSV pour qui doit
 * en rendre compte.
 */
const searchSchema = z.object({
  page: z.number().int().min(1).catch(1),
  action: z.string().optional(),
  search: z.string().optional(),
  from: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional().catch(undefined),
  to: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional().catch(undefined),
})

export const Route = createFileRoute('/_app/parametres/audit/')({
  validateSearch: searchSchema,
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'audit.read')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  component: AuditPage,
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })
const PAGE = 50

function AuditPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const filters: AuditFilters = { action: search.action, search: search.search, from: search.from, to: search.to }
  const { data, isError, error } = useQuery(auditQuery(filters, search.page))

  function setFilter(patch: Partial<AuditFilters>) {
    void navigate({ search: (prev) => ({ ...prev, ...patch, page: 1 }), replace: true })
  }
  const [draft, setDraft] = useSearchField(search.search ?? '', (value) => setFilter({ search: value === '' ? undefined : value }))

  const total = data?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / PAGE))

  return (
    <PageFrame title="Journal d’audit">
      <div className="flex min-h-full flex-col">
        <div className="flex flex-wrap items-center gap-2 p-4">
          <div className="relative min-w-[200px] flex-1">
            <HugeiconsIcon icon={Search01Icon} size={16} strokeWidth={1.6} className="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-[#8d8d8d]" />
            <Input
              type="search"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              placeholder="Compte, adresse IP, identifiant…"
              aria-label="Rechercher dans le journal"
              className="h-9 pl-8 text-[13px]"
            />
          </div>
          <FilterMenu
            name="Action"
            all="Toutes les actions"
            value={search.action}
            options={AUDIT_FAMILIES}
            onChange={(value) => setFilter({ action: value })}
          />
          <Input type="date" aria-label="Du" value={search.from ?? ''} onChange={(e) => setFilter({ from: e.target.value || undefined })} className="h-9 w-[150px] text-[13px]" />
          <Input type="date" aria-label="Au" value={search.to ?? ''} onChange={(e) => setFilter({ to: e.target.value || undefined })} className="h-9 w-[150px] text-[13px]" />
          <Button asChild variant="outline" size="lg" className="gap-1.5">
            <a href={auditExportUrl(filters)}>
              <HugeiconsIcon icon={Download04Icon} size={16} strokeWidth={1.8} />
              Exporter
            </a>
          </Button>
        </div>

        {isError ? (
          <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
            {error instanceof HttpError ? error.message : 'Chargement impossible'}
          </p>
        ) : (
          <div className="flex min-h-0 flex-1 flex-col overflow-x-auto">
            <table className="w-full min-w-[860px] text-left text-[13.5px]">
              <thead className="bg-[#f3f4f4] text-[#73757c]">
                <tr>
                  <th className="px-3 py-2.5 font-normal">Date</th>
                  <th className="px-3 py-2.5 font-normal">Compte</th>
                  <th className="px-3 py-2.5 font-normal">Action</th>
                  <th className="px-3 py-2.5 font-normal">Détails</th>
                  <th className="px-3 py-2.5 font-normal">Adresse IP</th>
                </tr>
              </thead>
              <tbody>
                {(data?.items ?? []).map((item) => (
                  <Line key={item.id} item={item} />
                ))}
                {data !== undefined && data.items.length === 0 && (
                  <tr>
                    <td colSpan={5} className="px-3 py-8 text-center text-[#73757c]">
                      Rien pour ces filtres.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>

            <div className="flex flex-wrap items-center justify-between gap-2 p-4">
              <p className="text-[12px] text-[#777]">
                {total} ligne{total > 1 ? 's' : ''} · page {search.page} sur {pages}
                {data !== undefined && ` · conservées ${data.retention_days} jours`}
              </p>
              <div className="flex gap-2">
                <Button variant="outline" size="sm" disabled={search.page <= 1} onClick={() => void navigate({ search: (p) => ({ ...p, page: search.page - 1 }) })}>
                  Précédent
                </Button>
                <Button variant="outline" size="sm" disabled={search.page >= pages} onClick={() => void navigate({ search: (p) => ({ ...p, page: search.page + 1 }) })}>
                  Suivant
                </Button>
              </div>
            </div>
          </div>
        )}
      </div>
    </PageFrame>
  )
}

function details(item: AuditItem): string {
  const parts = Object.entries(item.details).map(([k, v]) => `${k} : ${Array.isArray(v) ? v.join(', ') : String(v)}`)
  if (item.target_id !== '') parts.unshift(`${item.target_type || 'cible'} ${item.target_id.slice(0, 8)}`)
  return parts.join(' · ')
}

function Line({ item }: { item: AuditItem }) {
  const failed = item.action === 'auth.login_failed' || item.action === 'auth.login_rate_limited'
  return (
    <tr className="border-b border-[#f3f4f4] align-top">
      <td className="px-3 py-2.5 whitespace-nowrap text-[#4b4b4f] tabular-nums">{WHEN.format(new Date(item.at))}</td>
      <td className="max-w-[220px] truncate px-3 py-2.5 text-[#1b1b1b]">{item.actor_email || '—'}</td>
      <td className={failed ? 'px-3 py-2.5 text-[#a30f2c]' : 'px-3 py-2.5 text-[#1b1b1b]'} title={item.action}>
        {AUDIT_ACTION[item.action] ?? item.action}
      </td>
      <td className="max-w-[320px] px-3 py-2.5 break-words text-[#73757c]">{details(item)}</td>
      <td className="px-3 py-2.5 whitespace-nowrap text-[#73757c] tabular-nums" title={item.user_agent}>
        {item.ip}
      </td>
    </tr>
  )
}
