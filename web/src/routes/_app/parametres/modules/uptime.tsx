import { ActivityIcon, Alert02Icon, ArrowLeft02Icon, Link04Icon, RefreshIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Outlet, redirect, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { toast } from 'sonner'

import uptimeUrl from '@/assets/sidebar/app-uptime-kuma.svg'
import { PageFrame } from '@/components/layout/page-frame'
import { TabBar, type Tab } from '@/components/layout/tab-bar'
import { Button } from '@/components/ui/button'
import { StatusPill } from '@/features/projects/ui'
import { uptimeKeys, uptimeSettingsQuery, useTestUptime } from '@/features/uptime/api'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import type { UptimeSettings } from '@/types/api'

/**
 * Module de disponibilite, branche sur Uptime Kuma.
 *
 * Un en-tete commun — l'etat de la releve et « Tester », utiles d'ou qu'on
 * soit — puis trois onglets : la connexion, les sites, les incidents. Rien
 * ici n'appelle Kuma : « Tester » demande une releve a la tache de fond, et
 * l'ecran en lit le resultat.
 */
export const Route = createFileRoute('/_app/parametres/modules/uptime')({
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'users.write')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  loader: ({ context }) => context.queryClient.ensureQueryData(uptimeSettingsQuery),
  component: UptimeLayout,
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

const STATE: Record<UptimeSettings['status']['state'], { label: string; color: string; pill: { bg: string; border: string; text: string } }> = {
  off: { label: 'Inactif', color: '#c4c4c4', pill: { bg: '#f3f4f4', border: '#e8e8e9', text: '#73757c' } },
  pending: { label: 'En attente de relève', color: '#eab308', pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' } },
  ok: { label: 'Actif', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } },
  error: { label: 'Erreur de relève', color: '#e5484d', pill: { bg: '#fdf3f3', border: '#f2d5d6', text: '#e5484d' } },
}

function UptimeLayout() {
  const { data } = useQuery(uptimeSettingsQuery)
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const lastPoll = data?.status.last_poll_at

  // Une releve terminee change les sites et les incidents : les onglets les
  // relisent plutot que d'attendre qu'on recharge la page.
  useEffect(() => {
    if (lastPoll === undefined) return
    void queryClient.invalidateQueries({ queryKey: uptimeKeys.allMonitors })
    void queryClient.invalidateQueries({ queryKey: ['uptime', 'incidents'] })
  }, [lastPoll, queryClient])

  if (data === undefined) return null

  const tabs: Tab[] = [
    { to: '/parametres/modules/uptime', label: 'Connexion', icon: Link04Icon },
    { to: '/parametres/modules/uptime/sites', label: 'Sites', icon: ActivityIcon, badge: data.status.down },
    { to: '/parametres/modules/uptime/incidents', label: 'Incidents', icon: Alert02Icon },
  ]

  return (
    <PageFrame title="Uptime Kuma" trail={[{ label: 'Modules', to: '/parametres/modules' }]}>
      <div className="flex flex-col">
        <div className="flex flex-wrap items-center gap-4 p-4">
          <Button type="button" variant="outline" size="icon-lg" aria-label="Revenir aux modules" onClick={() => void navigate({ to: '/parametres/modules' })}>
            <HugeiconsIcon icon={ArrowLeft02Icon} size={20} strokeWidth={1.8} />
          </Button>
          <img src={uptimeUrl} alt="" className="size-10 shrink-0 object-contain" />
          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <h1 className="flex items-center gap-2 truncate text-[20px] leading-[1.4] font-medium text-[#1b1b1b]">
              Uptime Kuma
              <StatusPill {...STATE[data.status.state]} />
            </h1>
            <StatusLine settings={data} />
          </div>
          <TestButton settings={data} />
        </div>

        <div className="flex items-center border-b border-[#e8e8e9] pl-4">
          <TabBar tabs={tabs} layoutId="uptime-module-tab" />
        </div>

        <Outlet />
      </div>
    </PageFrame>
  )
}

/** Ce que la derniere releve a trouve, en une ligne. */
function StatusLine({ settings }: { settings: UptimeSettings }) {
  const s = settings.status
  if (s.last_poll_error !== null) return <p className="truncate text-[14px] text-[#a30f2c]">{s.last_poll_error}</p>
  if (s.last_poll_at === null) {
    return <p className="truncate text-[14px] text-[#73757c]">Jamais relevé : renseignez la connexion, puis testez-la.</p>
  }
  return (
    <p className="truncate text-[14px] text-[#73757c]">
      {s.monitors} site{s.monitors > 1 ? 's' : ''} · <span className="text-[#006f1f]">{s.up} en ligne</span>
      {s.down > 0 && <span className="text-[#e5484d]"> · {s.down} hors ligne</span>} · relevé le {WHEN.format(new Date(s.last_poll_at))}
    </p>
  )
}

function TestButton({ settings }: { settings: UptimeSettings }) {
  const test = useTestUptime()
  const pending = test.isPending || settings.status.poll_pending

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      className="gap-1.5"
      disabled={pending || (settings.base_url === '' && !settings.base_url_from_env)}
      onClick={() => test.mutate(undefined, { onError: (e) => toast.error(e instanceof HttpError ? e.message : 'Demande impossible') })}
    >
      <HugeiconsIcon icon={RefreshIcon} size={14} strokeWidth={1.8} className={pending ? 'animate-spin' : undefined} />
      {pending ? 'Relève en cours…' : 'Tester la connexion'}
    </Button>
  )
}
