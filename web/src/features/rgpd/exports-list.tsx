import { Download04Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'

import { Button } from '@/components/ui/button'
import { fileSize } from '@/features/portal/format'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'

import { exportDownloadUrl, exportsQuery } from './api'
import { EXPORT_STATUS_LABEL, exportFailureLabel, formatWhen, SUBJECT_LABEL } from './labels'

/** Historique des exports : le zip se télécharge tant qu'il est prêt et non expiré. */
export function ExportsList({ page, onPage }: { page: number; onPage: (page: number) => void }) {
  const { data, isError, error } = useQuery(exportsQuery(page))

  const total = data?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / (data?.page_size ?? 25)))

  if (isError) return <p className="text-[13px] text-[#e5484d]">{error instanceof HttpError ? error.message : 'Chargement impossible'}</p>

  return (
    <>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[720px] text-left text-[13.5px]">
          <thead className="bg-[#f3f4f4] text-[#73757c]">
            <tr>
              <th className="px-3 py-2.5 font-normal">Demandé le</th>
              <th className="px-3 py-2.5 font-normal">Sujet</th>
              <th className="px-3 py-2.5 font-normal">Statut</th>
              <th className="px-3 py-2.5 font-normal">Taille</th>
              <th className="px-3 py-2.5 font-normal">Disponible jusqu’au</th>
              <th className="px-3 py-2.5 font-normal"><span className="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            {(data?.items ?? []).map((item) => (
              <tr key={item.id} className="border-b border-[#f3f4f4] align-top">
                <td className="px-3 py-2.5 whitespace-nowrap text-[#4b4b4f] tabular-nums">{formatWhen(item.created_at)}</td>
                <td className="max-w-[260px] truncate px-3 py-2.5 text-[#1b1b1b]" title={item.subject_label}>
                  {item.subject_label}
                  <span className="ml-2 text-[12px] text-[#777]">{SUBJECT_LABEL[item.subject_type]}</span>
                </td>
                <td className={cn('px-3 py-2.5', item.status === 'failed' ? 'text-[#a30f2c]' : 'text-[#1b1b1b]')}>
                  {EXPORT_STATUS_LABEL[item.status]}
                  {item.status === 'failed' && <span className="block text-[12px]">{exportFailureLabel(item.error)}</span>}
                </td>
                <td className="px-3 py-2.5 text-[#4b4b4f] tabular-nums">{item.status === 'ready' ? fileSize(item.size_bytes) : '—'}</td>
                <td className="px-3 py-2.5 whitespace-nowrap text-[#4b4b4f] tabular-nums">
                  {item.expires_at === null || item.status === 'expired' ? '—' : formatWhen(item.expires_at)}
                </td>
                <td className="px-3 py-2.5 text-right whitespace-nowrap">
                  {item.status === 'ready' && (
                    <Button asChild variant="outline" size="sm" className="gap-1.5">
                      {/* `download` : le serveur répond déjà en pièce jointe. */}
                      <a href={exportDownloadUrl(item.id)} download>
                        <HugeiconsIcon icon={Download04Icon} size={14} strokeWidth={1.8} />
                        Télécharger
                      </a>
                    </Button>
                  )}
                </td>
              </tr>
            ))}
            {data !== undefined && data.items.length === 0 && (
              <tr>
                <td colSpan={6} className="px-3 py-8 text-center text-[#73757c]">
                  Aucun export pour l’instant. Lancez-en un depuis la fiche d’un client ou l’écran des comptes.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <Pager total={total} page={page} pages={pages} noun="export" onPage={onPage} />
    </>
  )
}

export function Pager({ total, page, pages, noun, onPage }: { total: number; page: number; pages: number; noun: string; onPage: (page: number) => void }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2">
      <p className="text-[12px] text-[#777]">
        {total} {noun}
        {total > 1 ? 's' : ''} · page {page} sur {pages}
      </p>
      <div className="flex gap-2">
        <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPage(page - 1)}>
          Précédent
        </Button>
        <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => onPage(page + 1)}>
          Suivant
        </Button>
      </div>
    </div>
  )
}
