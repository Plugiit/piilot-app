import { Download04Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, redirect } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'
import { z } from 'zod'

import { PageFrame } from '@/components/layout/page-frame'
import { Card } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { exportUrl, importsQuery, useDeleteImport } from '@/features/data-import/api'
import { ENTITIES, entityLabel, isActive, STATUS_LABEL } from '@/features/data-import/entities'
import { UploadStep } from '@/features/data-import/upload-step'
import { fileSize } from '@/features/portal/format'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { ImportItem } from '@/types/api'

/**
 * Import et export de données.
 *
 * Importer : on dépose un CSV, l'assistant (`$id`) fait le reste. L'historique
 * garde chaque import avec son rapport. Exporter : un CSV par entité, au
 * format que l'import relit — l'aller-retour ne perd rien.
 */
export const Route = createFileRoute('/_app/parametres/import/')({
  validateSearch: z.object({ page: z.number().int().min(1).catch(1) }),
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'data.import')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  component: ImportPage,
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })

function ImportPage() {
  const { user } = Route.useRouteContext()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const { data, isError, error } = useQuery(importsQuery(search.page))
  const [removing, setRemoving] = useState<ImportItem | null>(null)
  const remove = useDeleteImport()

  const total = data?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / (data?.page_size ?? 25)))

  return (
    <PageFrame title="Import et export">
      <div className="flex flex-col gap-3 p-4">
        <Card title="Importer un fichier CSV" description="Clients, contacts, projets ou temps passé. Une simulation précède toujours l’import réel.">
          <UploadStep onUploaded={(id) => void navigate({ to: '/parametres/import/$id', params: { id }, search: { step: 'apercu', page: 1 } })} />
        </Card>

        <Card title="Historique" description="Chaque import garde son rapport. Retirer un import n’annule pas les données créées.">
          {isError ? (
            <p className="text-[13px] text-[#e5484d]">{error instanceof HttpError ? error.message : 'Chargement impossible'}</p>
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="w-full min-w-[720px] text-left text-[13.5px]">
                  <thead className="bg-[#f3f4f4] text-[#73757c]">
                    <tr>
                      <th className="px-3 py-2.5 font-normal">Date</th>
                      <th className="px-3 py-2.5 font-normal">Fichier</th>
                      <th className="px-3 py-2.5 font-normal">Contenu</th>
                      <th className="px-3 py-2.5 font-normal">Statut</th>
                      <th className="px-3 py-2.5 font-normal">Créées / ignorées / erreurs</th>
                      <th className="px-3 py-2.5 font-normal"><span className="sr-only">Actions</span></th>
                    </tr>
                  </thead>
                  <tbody>
                    {(data?.items ?? []).map((item) => (
                      <tr key={item.id} className="border-b border-[#f3f4f4] align-top">
                        <td className="px-3 py-2.5 whitespace-nowrap text-[#4b4b4f] tabular-nums">{WHEN.format(new Date(item.created_at))}</td>
                        <td className="max-w-[240px] truncate px-3 py-2.5 text-[#1b1b1b]" title={item.file_name}>
                          {item.file_name}
                          <span className="ml-2 text-[12px] text-[#777]">{fileSize(item.size_bytes)}</span>
                        </td>
                        <td className="px-3 py-2.5 text-[#1b1b1b]">{entityLabel(item.entity)}</td>
                        <td className={cn('px-3 py-2.5', item.status === 'failed' ? 'text-[#a30f2c]' : 'text-[#1b1b1b]')}>
                          {STATUS_LABEL[item.status]}
                          {item.dry_run && item.status === 'done' && <span className="text-[#73757c]"> (simulation)</span>}
                        </td>
                        <td className="px-3 py-2.5 text-[#4b4b4f] tabular-nums">
                          {item.status === 'uploaded' ? '—' : `${item.created_n} / ${item.skipped_n} / ${item.error_n}`}
                        </td>
                        <td className="px-3 py-2.5 text-right whitespace-nowrap">
                          <Button asChild variant="ghost" size="sm">
                            <Link
                              to="/parametres/import/$id"
                              params={{ id: item.id }}
                              search={{ step: item.status === 'uploaded' ? 'apercu' : item.dry_run && item.status === 'done' ? 'simulation' : 'rapport', page: 1 }}
                              preload="intent"
                            >
                              {item.status === 'uploaded' ? 'Reprendre' : 'Ouvrir'}
                            </Link>
                          </Button>
                          <Button variant="ghost" size="sm" disabled={isActive(item.status)} onClick={() => setRemoving(item)}>
                            Retirer
                          </Button>
                        </td>
                      </tr>
                    ))}
                    {data !== undefined && data.items.length === 0 && (
                      <tr>
                        <td colSpan={6} className="px-3 py-8 text-center text-[#73757c]">
                          Aucun import pour l’instant.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-[12px] text-[#777]">
                  {total} import{total > 1 ? 's' : ''} · page {search.page} sur {pages}
                </p>
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" disabled={search.page <= 1} onClick={() => void navigate({ search: { page: search.page - 1 } })}>
                    Précédent
                  </Button>
                  <Button variant="outline" size="sm" disabled={search.page >= pages} onClick={() => void navigate({ search: { page: search.page + 1 } })}>
                    Suivant
                  </Button>
                </div>
              </div>
            </>
          )}
        </Card>

        {can(user, 'data.export') && (
          <Card title="Exporter" description="Un fichier CSV par type de données, relisible par l’import.">
            <div className="flex flex-wrap gap-2">
              {ENTITIES.map((entity) => (
                <Button key={entity.value} asChild variant="outline" size="lg" className="gap-1.5">
                  {/* `download` : le serveur répond déjà en pièce jointe, l'attribut évite toute navigation. */}
                  <a href={exportUrl(entity.value)} download>
                    <HugeiconsIcon icon={Download04Icon} size={16} strokeWidth={1.8} />
                    {entity.plural}
                  </a>
                </Button>
              ))}
            </div>
          </Card>
        )}
      </div>

      <Dialog open={removing !== null} onOpenChange={(open) => !open && setRemoving(null)}>
        <DialogContent className="sm:max-w-[460px]">
          <DialogHeader>
            <DialogTitle>Retirer cet import ?</DialogTitle>
            <DialogDescription>
              {removing?.file_name} quitte l’historique avec son rapport. Les données déjà créées restent en place, et
              rejouer le fichier ne les recrée pas.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" size="lg" onClick={() => setRemoving(null)}>
              Annuler
            </Button>
            <Button
              variant="destructive"
              size="lg"
              disabled={remove.isPending}
              onClick={() => {
                if (removing === null) return
                remove.mutate(removing.id, {
                  onSuccess: () => {
                    setRemoving(null)
                    toast.success('Import retiré')
                  },
                  onError: (e) => toast.error(e instanceof HttpError ? e.message : 'Suppression impossible'),
                })
              }}
            >
              Retirer
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </PageFrame>
  )
}
