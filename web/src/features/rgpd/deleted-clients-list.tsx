import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { Button } from '@/components/ui/button'
import { HttpError } from '@/lib/api'

import { useExportAction } from './actions'
import { deletedClientsQuery, useScheduledErasure } from './api'
import { ErasureBanner } from './erasure-banner'
import { Pager } from './exports-list'
import { formatWhen } from './labels'
import { ErasureDialog } from './request-dialog'

function DeletedClientRow({ id, name, deletedAt }: { id: string; name: string; deletedAt: string }) {
  const [erasing, setErasing] = useState(false)
  const exporter = useExportAction({ type: 'client', id, name })
  const scheduled = useScheduledErasure('client', id, true)

  return (
    <tr className="border-b border-[#f3f4f4] align-top">
      <td className="max-w-[320px] truncate px-3 py-2.5 text-[#1b1b1b]" title={name}>
        {name}
      </td>
      <td className="px-3 py-2.5 whitespace-nowrap text-[#4b4b4f] tabular-nums">{formatWhen(deletedAt)}</td>
      <td className="px-3 py-2.5 text-right whitespace-nowrap">
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" disabled={exporter.pending} onClick={exporter.run}>
            Exporter les données
          </Button>
          {scheduled === undefined && (
            <Button variant="outline" size="sm" onClick={() => setErasing(true)}>
              Supprimer définitivement
            </Button>
          )}
        </div>
        {scheduled !== undefined && (
          <div className="mt-1.5 flex justify-end">
            <ErasureBanner erasure={scheduled} compact />
          </div>
        )}
        <ErasureDialog subject={{ type: 'client', id, name }} open={erasing} onOpenChange={setErasing} />
      </td>
    </tr>
  )
}

/**
 * Clients supprimés du CRM : la suppression y est logique, leurs données
 * existent toujours. Cette liste est le seul moyen de les exporter ou de les
 * effacer une fois qu'ils ont disparu du CRM.
 */
export function DeletedClientsList({ page, onPage }: { page: number; onPage: (page: number) => void }) {
  const { data, isError, isPending, error } = useQuery(deletedClientsQuery(page))

  const total = data?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / (data?.page_size ?? 25)))

  if (isError) return <p className="text-[13px] text-[#e5484d]">{error instanceof HttpError ? error.message : 'Chargement impossible'}</p>

  return (
    <>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[620px] text-left text-[13.5px]">
          <thead className="bg-[#f3f4f4] text-[#73757c]">
            <tr>
              <th className="px-3 py-2.5 font-normal">Client</th>
              <th className="px-3 py-2.5 font-normal">Supprimé du CRM le</th>
              <th className="px-3 py-2.5 font-normal">
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {(data?.items ?? []).map((item) => (
              <DeletedClientRow key={item.id} id={item.id} name={item.name} deletedAt={item.deleted_at} />
            ))}
            {isPending && (
              <tr>
                <td colSpan={3} className="px-3 py-8 text-center text-[#73757c]">
                  Chargement…
                </td>
              </tr>
            )}
            {data !== undefined && data.items.length === 0 && (
              <tr>
                <td colSpan={3} className="px-3 py-8 text-center text-[#73757c]">
                  Aucun client supprimé dont les données soient conservées.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <Pager total={total} page={page} pages={pages} noun="client" onPage={onPage} />
    </>
  )
}
