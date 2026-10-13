import { ArrowLeft02Icon, Globe02Icon, Link04Icon, RefreshIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Outlet, redirect, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { toast } from 'sonner'

import umamiUrl from '@/assets/integrations/umami.svg'
import { PageFrame } from '@/components/layout/page-frame'
import { TabBar, type Tab } from '@/components/layout/tab-bar'
import { Button } from '@/components/ui/button'
import { analyticsKeys, analyticsSettingsQuery, useCheckAnalytics } from '@/features/analytics/api'
import { StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import type { AnalyticsSettings } from '@/types/api'

/**
 * Module de statistiques, branche sur Umami : la connexion, puis les sites.
 * Rien ici n'appelle Umami — « Tester » demande une verification a la tache
 * de fond, et l'ecran en lit le resultat.
 */
export const Route = createFileRoute('/_app/parametres/modules/umami')({
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'users.write')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  loader: ({ context }) => context.queryClient.ensureQueryData(analyticsSettingsQuery),
  component: UmamiLayout,
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

const STATE: Record<AnalyticsSettings['status']['state'], { label: string; color: string; pill: { bg: string; border: string; text: string } }> = {
  off: { label: 'Inactif', color: '#c4c4c4', pill: { bg: '#f3f4f4', border: '#e8e8e9', text: '#73757c' } },
  pending: { label: 'En attente de vérification', color: '#eab308', pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' } },
  ok: { label: 'Actif', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } },
  error: { label: 'Erreur', color: '#e5484d', pill: { bg: '#fdf3f3', border: '#f2d5d6', text: '#e5484d' } },
}

function UmamiLayout() {
  const { data } = useQuery(analyticsSettingsQuery)
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const lastCheck = data?.status.last_check_at

  // Une verification terminee change la liste des sites.
  useEffect(() => {
    if (lastCheck === undefined) return
    void queryClient.invalidateQueries({ queryKey: analyticsKeys.allWebsites })
  }, [lastCheck, queryClient])

  if (data === undefined) return null

  const tabs: Tab[] = [
    { to: '/parametres/modules/umami', label: 'Connexion', icon: Link04Icon },
    { to: '/parametres/modules/umami/sites', label: 'Sites', icon: Globe02Icon },
  ]

  return (
    <PageFrame title="Umami" trail={[{ label: 'Modules', to: '/parametres/modules' }]}>
      <div className="flex flex-col">
        <div className="flex flex-wrap items-center gap-4 p-4">
          <Button type="button" variant="outline" size="icon-lg" aria-label="Revenir aux modules" onClick={() => void navigate({ to: '/parametres/modules' })}>
            <HugeiconsIcon icon={ArrowLeft02Icon} size={20} strokeWidth={1.8} />
          </Button>
          <img src={umamiUrl} alt="" className="size-10 shrink-0 object-contain" />
          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <h1 className="flex items-center gap-2 truncate text-[20px] leading-[1.4] font-medium text-[#1b1b1b]">
              Umami
              <StatusPill {...STATE[data.status.state]} />
            </h1>
            <StatusLine settings={data} />
          </div>
          <CheckButton settings={data} />
        </div>

        <div className="flex items-center border-b border-[#e8e8e9] pl-4">
          <TabBar tabs={tabs} layoutId="umami-module-tab" />
        </div>

        <Outlet />
      </div>
    </PageFrame>
  )
}

function StatusLine({ settings }: { settings: AnalyticsSettings }) {
  const s = settings.status
  if (s.last_check_error !== null) return <p className="truncate text-[14px] text-[#a30f2c]">{s.last_check_error}</p>
  if (s.last_check_at === null) {
    return <p className="truncate text-[14px] text-[#73757c]">Jamais vérifié : renseignez la connexion, puis testez-la.</p>
  }
  return (
    <p className="truncate text-[14px] text-[#73757c]">
      {s.websites} site{s.websites > 1 ? 's' : ''} · {s.linked} lié{s.linked > 1 ? 's' : ''} à un projet
      {settings.api_version !== 0 && ` · Umami ${settings.api_version}.x${settings.version_pinned ? ' (choisie)' : ''}`} · vérifié le{' '}
      {WHEN.format(new Date(s.last_check_at))}
      {s.last_check_note !== null && <span className="text-[#9a6a00]"> · {s.last_check_note}</span>}
    </p>
  )
}

function CheckButton({ settings }: { settings: AnalyticsSettings }) {
  const check = useCheckAnalytics()
  const pending = check.isPending || settings.status.check_pending

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      className="gap-1.5"
      disabled={pending || (settings.base_url === '' && !settings.base_url_from_env)}
      onClick={() => check.mutate(undefined, { onError: (e) => toast.error(e instanceof HttpError ? e.message : 'Demande impossible') })}
    >
      <HugeiconsIcon icon={RefreshIcon} size={14} strokeWidth={1.8} className={pending ? 'animate-spin' : undefined} />
      {pending ? 'Vérification…' : 'Tester la connexion'}
    </Button>
  )
}
