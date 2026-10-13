import { ArrowDown01Icon, ArrowUp01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { portalDashboardQuery } from '@/features/portal/api'
import { StatTile } from '@/features/portal/dashboard'
import { PortalAside, ProjectJourney } from '@/features/portal/journey'
import { daysUntil, parseApiDate } from '@/features/projects/format'
import { HttpError } from '@/lib/api'
import type { PortalDashboard } from '@/types/api'

/**
 * Tableau de bord du portail : l'accueil du client.
 *
 * Quatre chiffres d'abord, tous projets confondus. Puis un projet suivi — le
 * plus recemment actif, ou celui choisi — avec ses etapes, ses interlocuteurs,
 * ses livrables et ses demandes, et les autres projets pour basculer. A
 * droite, ce qui attend le client et l'historique du projet. Un appel remplit
 * tout l'ecran.
 */
const searchSchema = z.object({
  projet: z.string().uuid().optional().catch(undefined),
})

export const Route = createFileRoute('/client/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ projet: search.projet }),
  loader: ({ context, deps }) => context.queryClient.ensureQueryData(portalDashboardQuery(deps.projet)),
  component: ClientDashboardPage,
})

const LONG = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long' })

function ClientDashboardPage() {
  const { projet } = Route.useSearch()
  const { data, isError, error } = useQuery(portalDashboardQuery(projet))

  if (isError) {
    return (
      <p className="mx-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[14px] text-[#e5484d]">
        {error instanceof HttpError ? error.message : 'Chargement impossible'}
      </p>
    )
  }

  return data === undefined ? null : <Dashboard data={data} />
}

function Dashboard({ data }: { data: PortalDashboard }) {
  const navigate = Route.useNavigate()
  const openTickets = data.tickets.received + data.tickets.in_progress + data.tickets.ready
  const resolved30 = data.ticket_days.slice(-SHOWN_DAYS).reduce((sum, d) => sum + d.resolved, 0)
  const due = parseApiDate(data.next_milestone?.due_on)
  const days = due === null ? null : daysUntil(due)
  const focus = data.focus

  // `replace` : changer de projet suivi affine la vue, sans etape a repasser
  // au retour arriere.
  function follow(id: string) {
    void navigate({ search: (prev) => ({ ...prev, projet: id }), replace: true })
  }

  return (
    // La disposition du tableau de bord de l'agence : une colonne de cartes,
    // et le panneau de droite qui la longe sur toute la hauteur.
    <div className="flex flex-1 flex-col overflow-hidden rounded-t-[20px] bg-white xl:flex-row">
      <div className="flex min-w-0 flex-1 flex-col gap-4 p-4">
        {/* Les quatre chiffres. */}
        <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
          <StatTile
            label="Avancement moyen"
            value={data.progress_average === null ? '—' : `${data.progress_average} %`}
            ratio={(data.progress_average ?? 0) / 100}
            footer={
              data.active_projects === 0
                ? 'Aucun projet en cours'
                : `${data.active_projects} projet${data.active_projects > 1 ? 's' : ''} en cours`
            }
          />

          <PendingTile data={data} />

          <StatTile
            label="Demandes en cours"
            value={openTickets}
            footer="Résolues sur 30 jours"
            aside={<Trend value={resolved30} />}
          />

          <StatTile
            label="Prochaine échéance"
            value={days === null ? '—' : days < 0 ? 'En retard' : days === 0 ? 'Aujourd’hui' : `${days} j`}
            footer={
              data.next_milestone === null
                ? 'Aucune étape à venir'
                : `${data.next_milestone.title}${due !== null ? ` · ${LONG.format(due)}` : ''}`
            }
          />
        </div>

        {focus === null ? (
          <p className="rounded-[12px] border border-dashed border-[#e3e4e6] p-8 text-center text-[14px] text-[#73757c]">
            Aucun projet pour l’instant. Ils apparaîtront ici dès que l’agence les aura ouverts.
          </p>
        ) : (
          <ProjectJourney
            key={focus.id}
            data={data}
            focus={focus}
            summary={data.projects.find((p) => p.id === focus.id)}
            projects={data.projects}
            follow={follow}
          />
        )}
      </div>

      <PortalAside
        actions={data.actions}
        activity={data.activity}
        focus={focus}
        className="border-surface-sunken shrink-0 border-t xl:w-[340px] xl:border-t-0 xl:border-l"
      />
    </div>
  )
}

function PendingTile({ data }: { data: PortalDashboard }) {
  return (
    <StatTile
      label="Livrables à valider"
      value={data.deliverables_pending}
      footer={data.deliverables_pending > 0 ? 'Votre réponse est attendue' : 'Validés ce mois-ci'}
      aside={data.deliverables_pending > 0 ? undefined : <Trend value={data.validated_this_month} />}
    />
  )
}

/** Les jours comptes au pied de la tuile ; l'API en renvoie six de plus. */
const SHOWN_DAYS = 30

/** Le chiffre du mois, en vert quand il y a eu quelque chose. */
function Trend({ value }: { value: number }) {
  const up = value > 0
  return (
    <span className="flex shrink-0 items-center gap-0.5" style={{ color: up ? '#006f1f' : '#8d8d8d' }}>
      <HugeiconsIcon icon={up ? ArrowUp01Icon : ArrowDown01Icon} size={14} strokeWidth={2} />
      <span className="tabular-nums">{value}</span>
    </span>
  )
}
