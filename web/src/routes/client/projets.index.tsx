import { Alert02Icon, Calendar03Icon, CheckmarkCircle02Icon, Clock01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'

import { portalProjectsQuery } from '@/features/portal/api'
import { PORTAL_PROJECT_STATUS, since } from '@/features/portal/format'
import { parseApiDate } from '@/features/projects/format'
import { ProgressRing, ProjectLogo, StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import type { PortalProject } from '@/types/api'

/**
 * « Projets » : tous les projets du client.
 *
 * Une phrase qui resume, puis les projets en cours — ceux qui attendent sa
 * reponse d'abord —, et les projets livres ou en ligne. Les cartes reprennent
 * celles de la liste des projets de l'admin, avec ce que le client a besoin
 * de lire : l'etat, la prochaine etape, l'avancement, et ce qui l'attend.
 */
export const Route = createFileRoute('/client/projets/')({
  loader: ({ context }) => context.queryClient.ensureQueryData(portalProjectsQuery),
  component: ClientProjectsPage,
})

const DATE = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })

/** En cours pour le client : tant que le projet n'est ni livre ni en ligne. */
const ONGOING: PortalProject['status'][] = ['cadrage', 'production', 'attente']

function ClientProjectsPage() {
  const { data, isError, error } = useQuery(portalProjectsQuery)
  const projects = data?.items ?? []

  // En cours : ce qui attend une reponse d'abord, puis le plus recemment actif.
  const active = projects
    .filter((p) => ONGOING.includes(p.status))
    .sort((a, b) => b.deliverables_pending - a.deliverables_pending || b.last_activity_at.localeCompare(a.last_activity_at))
  const done = projects.filter((p) => !ONGOING.includes(p.status))
  const waiting = projects.reduce((sum, p) => sum + p.deliverables_pending, 0)

  return (
    <div className="flex w-full flex-col gap-8 py-2">
      <header className="flex flex-col gap-2">
        <h1 className="font-heading text-[28px] leading-tight font-medium text-[#1b1b1b]">Vos projets</h1>
        {data !== undefined && projects.length > 0 && <Summary active={active.length} done={done.length} waiting={waiting} />}
      </header>

      {isError && (
        <p className="rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[14px] text-[#e5484d]">
          {error instanceof HttpError ? error.message : 'Chargement impossible'}
        </p>
      )}

      {data !== undefined && projects.length === 0 && (
        <p className="rounded-[16px] border border-dashed border-[#e3e4e6] p-10 text-center text-[15px] text-[#73757c]">
          Aucun projet pour l’instant. Ils apparaîtront ici dès que l’agence les aura ouverts.
        </p>
      )}

      {active.length > 0 && (
        <section className="flex flex-col gap-3">
          <SectionTitle label="En cours" count={active.length} />
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
            {active.map((project) => (
              <ProjectCard key={project.id} project={project} />
            ))}
          </div>
        </section>
      )}

      {done.length > 0 && (
        <section className="flex flex-col gap-3">
          <SectionTitle label="Livrés et en ligne" count={done.length} />
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
            {done.map((project) => (
              <ProjectCard key={project.id} project={project} />
            ))}
          </div>
        </section>
      )}
    </div>
  )
}

/** La phrase d'en-tete : ce qui avance, et ce qui attend le client. */
function Summary({ active, done, waiting }: { active: number; done: number; waiting: number }) {
  const parts = [
    active > 0 && `${active} ${active === 1 ? 'projet en cours' : 'projets en cours'}`,
    done > 0 && `${done} ${done === 1 ? 'livré ou en ligne' : 'livrés ou en ligne'}`,
  ].filter(Boolean)
  return (
    <p className="text-[16px] text-[#73757c]">
      {parts.join(', ')}.{' '}
      {waiting > 0 ? (
        <span className="font-medium text-[#b84a0c]">
          {waiting === 1 ? 'Un livrable attend votre réponse.' : `${waiting} livrables attendent votre réponse.`}
        </span>
      ) : (
        'Rien n’attend votre réponse.'
      )}
    </p>
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

/**
 * Carte d'un projet, au dessin de celle de l'admin : l'etat en haut, et ce
 * qui attend le client en face ; l'identite au milieu ; une ligne de reperes
 * — prochaine etape, derniere activite, avancement. Toute la carte ouvre le
 * projet, par le lien du titre etendu sur elle.
 */
function ProjectCard({ project }: { project: PortalProject }) {
  const status = PORTAL_PROJECT_STATUS[project.status]
  const ongoing = ONGOING.includes(project.status)
  const next = project.next_milestone
  const nextDue = parseApiDate(next?.due_on)
  const late = nextDue !== null && nextDue.getTime() < Date.now() - 86_400_000
  const pending = project.deliverables_pending

  return (
    <article className="relative flex flex-col gap-3 rounded-[12px] border border-[#e8e8e9] bg-white p-3 transition-colors hover:border-[#d0d1d3] hover:bg-[#fcfcfc]">
      <div className="flex items-center justify-between gap-2">
        <StatusPill label={status.label} color={status.color} pill={status.pill} />
        {pending > 0 && (
          <span className="bg-brand shrink-0 rounded-full px-2 py-0.5 text-[12px] font-medium whitespace-nowrap text-white">
            {pending === 1 ? '1 livrable à valider' : `${pending} livrables à valider`}
          </span>
        )}
      </div>

      <div className="flex flex-col gap-1.5">
        <div className="flex min-w-0 flex-col gap-0.5">
          <span className="flex min-w-0 items-center gap-2">
            <ProjectLogo url={project.logo_url} size={24} bare />
            <Link
              to="/client/projets/$id"
              params={{ id: project.id }}
              hash={pending > 0 ? 'a-valider' : undefined}
              className="truncate text-[16px] leading-[1.5] font-medium text-[#1b1b1b] after:absolute after:inset-0 after:rounded-[12px] focus-visible:outline-none focus-visible:after:ring-3 focus-visible:after:ring-brand/30"
            >
              {project.name}
            </Link>
          </span>
          {/* A defaut de description, ou en est le projet : une carte sans
              deuxieme ligne se tasse et la grille perd son alignement. */}
          <p className="truncate text-[14px] leading-[1.5] text-[#73757c]">
            {project.description === '' ? `Mis à jour ${since(project.last_activity_at)}` : project.description}
          </p>
        </div>

        <div className="flex items-center gap-2">
          <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-3 gap-y-1">
            {ongoing && (
              <span
                className={
                  late
                    ? 'flex min-w-0 items-center gap-1 text-[12px] font-medium text-[#e5484d]'
                    : 'flex min-w-0 items-center gap-1 text-[12px] text-[#73757c]'
                }
              >
                <HugeiconsIcon
                  icon={next === null ? CheckmarkCircle02Icon : late ? Alert02Icon : Calendar03Icon}
                  size={16}
                  strokeWidth={1.6}
                  className="shrink-0"
                />
                <span className="truncate">
                  {next === null
                    ? project.progress >= 100
                      ? 'Toutes les étapes sont atteintes'
                      : 'Prochaine étape à venir'
                    : `${next.title}${nextDue === null ? '' : ` · ${DATE.format(nextDue)}`}`}
                </span>
              </span>
            )}
            {(!ongoing || project.description !== '') && (
              <span className="flex items-center gap-1 text-[12px] whitespace-nowrap text-[#73757c]">
                <HugeiconsIcon icon={Clock01Icon} size={16} strokeWidth={1.6} />
                {since(project.last_activity_at)}
              </span>
            )}
          </div>

          {ongoing && (
            <span className="flex shrink-0 items-center gap-1 rounded-[8px] bg-[#f3f4f4] p-1.5 text-[12px] font-medium text-[#1b1b1b]">
              <ProgressRing value={project.progress} />
              <span className="tabular-nums">{project.progress} %</span>
            </span>
          )}
        </div>
      </div>
    </article>
  )
}
