import { Settings02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, redirect } from '@tanstack/react-router'

import umamiUrl from '@/assets/integrations/umami.svg'
import uptimeUrl from '@/assets/sidebar/app-uptime-kuma.svg'
import { CARD_ACTION, IntegrationCard } from '@/components/integration-card'
import { PageFrame } from '@/components/layout/page-frame'
import { StatusPill } from '@/features/projects/ui'
import { modulesQuery } from '@/features/uptime/api'
import { can } from '@/lib/auth'
import type { Module } from '@/types/api'

/**
 * Les modules : des fonctions qui s'appuient sur un service exterieur, et
 * qui ne servent qu'aux agences qui l'utilisent. Une carte par service, comme
 * les integrations du compte ; chacun s'active depuis sa page.
 */
export const Route = createFileRoute('/_app/parametres/modules/')({
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'users.write')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  loader: ({ context }) => context.queryClient.ensureQueryData(modulesQuery),
  component: ModulesPage,
})

const STATE: Record<Module['state'], { label: string; color: string; pill: { bg: string; border: string; text: string } }> = {
  off: { label: 'Inactif', color: '#c4c4c4', pill: { bg: '#f3f4f4', border: '#e8e8e9', text: '#73757c' } },
  pending: { label: 'En attente', color: '#eab308', pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' } },
  ok: { label: 'Actif', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } },
  error: { label: 'Erreur', color: '#e5484d', pill: { bg: '#fdf3f3', border: '#f2d5d6', text: '#e5484d' } },
}

/** Le logo de chaque module, par son code. */
const LOGOS: Record<Module['code'], string> = { uptime: uptimeUrl, analytics: umamiUrl }

/** La page de chaque module. */
const PAGES = { uptime: '/parametres/modules/uptime', analytics: '/parametres/modules/umami' } as const

function ModulesPage() {
  const { data } = useQuery(modulesQuery)

  return (
    <PageFrame title="Modules">
      <section className="flex w-full flex-col gap-3 p-4">
        <header className="flex flex-col gap-0.5">
          <h2 className="text-[18px] leading-[1.5] font-medium tracking-[-0.18px] text-[#1b1b1b]">Modules</h2>
          <p className="text-[14px] leading-[1.5] text-[#73757c]">
            Des fonctions qui s’appuient sur un service extérieur. Activez celles dont l’agence se sert.
          </p>
        </header>

        <div className="grid w-full gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {(data?.items ?? []).map((m) => {
            const state = STATE[m.state]
            return (
              <IntegrationCard
                key={m.code}
                logo={LOGOS[m.code]}
                name={m.provider}
                description={m.description}
                badge={<StatusPill label={state.label} color={state.color} pill={state.pill} />}
                action={
                  <Link to={PAGES[m.code]} className={CARD_ACTION}>
                    <HugeiconsIcon icon={Settings02Icon} size={16} strokeWidth={1.8} />
                    {m.enabled ? 'Configurer' : 'Activer'}
                  </Link>
                }
              />
            )
          })}
        </div>
      </section>
    </PageFrame>
  )
}
