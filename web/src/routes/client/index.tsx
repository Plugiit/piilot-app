import { ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'

import { portalDashboardQuery, portalSitesQuery } from '@/features/portal/api'
import { History } from '@/features/portal/dashboard'
import { ActionsBox, Contacts, DoneProject, Greeting, JourneyCard, MonthTiles, RequestsCard } from '@/features/portal/home'
import { HttpError } from '@/lib/api'
import { sessionQuery } from '@/lib/auth'
import type { PortalDashboard } from '@/types/api'

/**
 * Accueil du portail.
 *
 * Un client passe de temps en temps, pour une raison. La page repond dans
 * l'ordre a ses deux questions : « qu'attend-on de moi ? » — une seule boite,
 * chaque chose avec son bouton — puis « ou en sont mes projets ? » — chacun
 * raconte en etapes, l'etape en cours marquee « vous etes ici », et de qui
 * depend la suite. A droite, ses interlocuteurs et ses demandes. Un appel
 * remplit tout l'ecran ; l'historique d'un projet s'ouvre a la demande.
 */
export const Route = createFileRoute('/client/')({
  loader: ({ context }) => context.queryClient.ensureQueryData(portalDashboardQuery()),
  component: ClientDashboardPage,
})

/** Au-dela, la frise des autres projets se lit sur la page Projets. */
const JOURNEYS_SHOWN = 5

function ClientDashboardPage() {
  const { data, isError, error } = useQuery(portalDashboardQuery())
  const { data: user } = useQuery(sessionQuery)

  if (isError) {
    return (
      <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[14px] text-[#e5484d]">
        {error instanceof HttpError ? error.message : 'Chargement impossible'}
      </p>
    )
  }

  return data === undefined ? null : <Dashboard data={data} firstname={user?.firstname ?? ''} />
}

function Dashboard({ data, firstname }: { data: PortalDashboard; firstname: string }) {
  // L'audience et la disponibilite des sites : les tables locales des modules,
  // celles du bandeau de la page Projets.
  const { data: sites } = useQuery(portalSitesQuery)
  const audience = sites?.audience ?? []
  const uptime = sites?.uptime ?? []
  const byId = new Map(data.projects.map((p) => [p.id, p]))
  // Les projets qui attendent le client d'abord, puis l'ordre de l'API : le
  // plus recemment actif en tete.
  const journeys = data.journeys
    .map((j) => ({ project: byId.get(j.project_id), steps: j.steps }))
    .filter((j): j is { project: NonNullable<typeof j.project>; steps: typeof j.steps } => j.project !== undefined)
    .sort((a, b) => b.project.deliverables_pending - a.project.deliverables_pending)
  const ongoing = new Set(data.journeys.map((j) => j.project_id))
  const done = data.projects.filter((p) => !ongoing.has(p.id))

  return (
    <div className="flex flex-1 flex-col xl:flex-row">
      <div className="flex min-w-0 flex-1 flex-col gap-6 p-4 sm:p-6">
        <Greeting firstname={firstname} data={data} />

        <MonthTiles data={data} audience={audience} uptime={uptime} />

        <ActionsBox actions={data.actions} />

        {data.projects.length === 0 && (
          <p className="rounded-[12px] border border-dashed border-[#e3e4e6] p-8 text-center text-[14px] text-[#73757c]">
            Aucun projet pour l’instant. Ils apparaîtront ici dès que l’agence les aura ouverts.
          </p>
        )}

        {journeys.length > 0 && (
          <section className="flex flex-col gap-3">
            <SectionTitle label="Vos projets en cours" count={journeys.length} />
            <div className="flex flex-col gap-3">
              {journeys.slice(0, JOURNEYS_SHOWN).map((j) => (
                <JourneyCard
                  key={j.project.id}
                  project={j.project}
                  steps={j.steps}
                  site={{
                    audience: audience.find((a) => a.project_ids.includes(j.project.id)),
                    uptime: uptime.filter((u) => u.project_id === j.project.id),
                  }}
                />
              ))}
            </div>
            {journeys.length > JOURNEYS_SHOWN && (
              <Link
                to="/client/projets"
                className="flex items-center gap-1 self-start rounded-[6px] px-1.5 py-1 text-[14px] font-medium text-[#4b4b4f] transition-colors hover:bg-[#f3f4f4] hover:text-[#1b1b1b]"
              >
                Voir vos {journeys.length} projets en cours
                <HugeiconsIcon icon={ArrowRight01Icon} size={16} strokeWidth={1.8} />
              </Link>
            )}
          </section>
        )}

        {done.length > 0 && (
          <section className="flex flex-col gap-3">
            <SectionTitle label="Livrés et en ligne" count={done.length} />
            <div className="grid grid-cols-1 gap-2 md:grid-cols-2 2xl:grid-cols-3">
              {done.map((p) => (
                <DoneProject key={p.id} project={p} />
              ))}
            </div>
          </section>
        )}
      </div>

      <aside className="border-surface-sunken flex shrink-0 flex-col gap-5 border-t bg-white p-4 sm:p-6 xl:w-[340px] xl:border-t-0 xl:border-l">
        {data.focus !== null && (
          <div className="border-surface-sunken border-b pb-5">
            <Contacts team={data.focus.team} />
          </div>
        )}
        <div className="border-surface-sunken border-b pb-5">
          <RequestsCard data={data} />
        </div>
        {/* L'historique de tous les projets : il porte son titre et sa recherche. */}
        <History items={data.activity} />
      </aside>
    </div>
  )
}

function SectionTitle({ label, count }: { label: string; count: number }) {
  return (
    <h2 className="flex items-center gap-2 text-[13px] font-medium tracking-[0.6px] text-[#8d8d8d] uppercase">
      {label}
      <span className="rounded-full bg-[#f3f4f4] px-2 py-0.5 text-[12px] tracking-normal text-[#73757c] tabular-nums">{count}</span>
    </h2>
  )
}
