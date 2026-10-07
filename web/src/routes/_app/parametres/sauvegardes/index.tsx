import { Alert02Icon, CheckmarkCircle02Icon, Clock01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'

import { PageFrame } from '@/components/layout/page-frame'
import { Card } from '@/components/settings-ui'
import { fileSize } from '@/features/portal/format'
import { backupStatusQuery } from '@/features/system/api'
import { can } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { BackupInfo } from '@/types/api'

/**
 * Etat des sauvegardes : la derniere reussie, et le journal des dernieres.
 *
 * C'est le service backup du docker-compose qui les fait et les note en base ;
 * cet ecran ne fait que lire. Une instance sans sauvegarde recente le dit en
 * rouge, et les admins en sont prevenus dans la cloche.
 */
export const Route = createFileRoute('/_app/parametres/sauvegardes/')({
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'system.update')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  component: BackupsPage,
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { weekday: 'long', day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })
const SHORT = new Intl.DateTimeFormat('fr-FR', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })

const STATUS: Record<BackupInfo['status'], { label: string; className: string; icon: typeof Clock01Icon }> = {
  done: { label: 'Réussie', className: 'text-[#006f1f]', icon: CheckmarkCircle02Icon },
  failed: { label: 'Échouée', className: 'text-[#a30f2c]', icon: Alert02Icon },
  running: { label: 'En cours', className: 'text-[#73757c]', icon: Clock01Icon },
}

function duration(item: BackupInfo): string {
  if (item.finished_at === null) return '—'
  const seconds = Math.max(1, Math.round((Date.parse(item.finished_at) - Date.parse(item.started_at)) / 1000))
  return seconds < 60 ? `${seconds} s` : `${Math.round(seconds / 60)} min`
}

function BackupsPage() {
  const { data, isPending } = useQuery(backupStatusQuery)

  return (
    <PageFrame title="Sauvegardes">
      <div className="flex flex-col gap-3 p-4">
        <Card title="Dernière sauvegarde réussie" description="La base et les pièces jointes, dans une seule archive, chaque nuit.">
          {isPending || data === undefined ? (
            <div className="h-[72px] animate-pulse rounded-[10px] bg-[#fafafa]" />
          ) : data.last_success === null ? (
            <Banner tone="danger">
              Aucune sauvegarde n’a encore réussi. Vérifiez que le service <code>backup</code> du
              docker-compose tourne : <code>docker compose logs backup</code>.
            </Banner>
          ) : (
            <div className="flex flex-col gap-2">
              <Banner tone={data.stale ? 'danger' : 'ok'}>
                {data.stale ? 'Elle date : ' : ''}
                {WHEN.format(new Date(data.last_success.started_at))} · {fileSize(data.last_success.size_bytes)}
                {data.stale && ` — plus de ${data.stale_after.replace('h0m0s', ' h')} sans sauvegarde réussie.`}
              </Banner>
              <p className="text-[13px] text-[#73757c]">Posée dans {data.last_success.location}.</p>
            </div>
          )}
        </Card>

        <Card title="Journal" description="Les vingt derniers passages, échecs compris.">
          {isPending || data === undefined ? (
            <div className="h-[120px] animate-pulse rounded-[10px] bg-[#fafafa]" />
          ) : data.items.length === 0 ? (
            <p className="px-2 py-4 text-[14px] text-[#73757c]">Rien encore.</p>
          ) : (
            <ul className="flex flex-col">
              {data.items.map((item) => {
                const status = STATUS[item.status]
                return (
                  <li key={item.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-[#f3f4f4] px-2 py-2.5 text-[14px] last:border-b-0">
                    <span className={cn('flex w-[110px] shrink-0 items-center gap-1.5', status.className)}>
                      <HugeiconsIcon icon={status.icon} size={15} strokeWidth={1.8} />
                      {status.label}
                    </span>
                    <span className="w-[150px] shrink-0 text-[#1b1b1b] tabular-nums">{SHORT.format(new Date(item.started_at))}</span>
                    <span className="w-[80px] shrink-0 text-[#73757c] tabular-nums">{item.status === 'done' ? fileSize(item.size_bytes) : '—'}</span>
                    <span className="w-[60px] shrink-0 text-[#73757c] tabular-nums">{duration(item)}</span>
                    {item.error !== '' && <span className="min-w-0 flex-1 truncate text-[13px] text-[#a30f2c]" title={item.error}>{item.error}</span>}
                  </li>
                )
              })}
            </ul>
          )}
        </Card>

        <Card title="Restaurer" description="Une sauvegarde n’a de valeur que si sa restauration marche : la CI du projet en restaure une à chaque poussée.">
          <div className="flex flex-col gap-2 text-[14px] text-[#1b1b1b]">
            <p>Sur le serveur, la plus récente :</p>
            <code className="rounded-[8px] border border-[#e8e8e9] bg-[#fafafa] px-3 py-2 font-mono text-[13px]">
              docker compose exec backup backup restore
            </code>
            <p className="text-[13px] text-[#73757c]">
              Une archive précise : <code>backup restore /app/data/backups/piilot-AAAAMMJJ-HHMMSS.tar</code>. Les archives sont
              dans le volume <code>piilot-backups</code>, et sur le stockage S3 si vous en avez donné un. Le détail est dans
              le README, section « Sauvegardes ».
            </p>
          </div>
        </Card>
      </div>
    </PageFrame>
  )
}

function Banner({ tone, children }: { tone: 'ok' | 'danger'; children: React.ReactNode }) {
  return (
    <p
      className={cn(
        'flex items-start gap-2 rounded-[10px] px-3 py-2.5 text-[14px]',
        tone === 'ok' ? 'bg-[#dcf7ea] text-[#006f1f]' : 'bg-[#ffe8ec] text-[#a30f2c]',
      )}
    >
      <HugeiconsIcon icon={tone === 'ok' ? CheckmarkCircle02Icon : Alert02Icon} size={18} strokeWidth={1.8} className="mt-0.5 shrink-0" />
      <span>{children}</span>
    </p>
  )
}
