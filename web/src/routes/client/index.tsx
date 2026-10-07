import { ArrowRight01Icon, Flag02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'

import { portalProjectsQuery } from '@/features/portal/api'
import { PORTAL_PROJECT_STATUS, since } from '@/features/portal/format'
import { DONE_COLOR, parseApiDate, PROGRESS_COLOR, tracksSchedule } from '@/features/projects/format'
import { Meter, ProjectLogo, StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import type { PortalProject } from '@/types/api'

/**
 * « Mes projets » : l'accueil du portail.
 *
 * Ce qui attend une reponse vient d'abord, en bandeau : c'est la raison pour
 * laquelle on ouvre le portail. Puis les projets, chacun avec son avancement
 * et sa prochaine etape — de quoi savoir ou l'on en est sans appeler.
 */
export const Route = createFileRoute('/client/')({
  loader: ({ context }) => context.queryClient.ensureQueryData(portalProjectsQuery),
  component: ClientProjectsPage,
})

const DATE = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long' })

function ClientProjectsPage() {
  const { user } = Route.useRouteContext()
  const { data, isError, error } = useQuery(portalProjectsQuery)
  const projects = data?.items ?? []
  const pending = projects.filter((project) => project.deliverables_pending > 0)
  const waiting = pending.reduce((sum, project) => sum + project.deliverables_pending, 0)

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="font-heading text-[26px] leading-tight font-medium text-[#1b1b1b]">Bonjour {user.firstname}</h1>
        <p className="text-[15px] text-[#73757c]">Où en sont vos projets, et ce qui attend votre réponse.</p>
      </header>

      {isError && (
        <p className="rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[14px] text-[#e5484d]">
          {error instanceof HttpError ? error.message : 'Chargement impossible'}
        </p>
      )}

      {waiting > 0 && (
        <section className="flex flex-col gap-2 rounded-[14px] border border-[#ffd9c2] bg-[#fff6f0] p-4">
          <p className="text-[15px] font-medium text-[#1b1b1b]">
            {waiting === 1 ? 'Un livrable attend votre validation' : `${waiting} livrables attendent votre validation`}
          </p>
          <ul className="flex flex-col gap-1">
            {pending.map((project) => (
              <li key={project.id}>
                <Link
                  to="/client/projets/$id"
                  params={{ id: project.id }}
                  hash="a-valider"
                  className="flex items-center gap-1 text-[14px] text-[#b84a0c] hover:underline"
                >
                  {project.name} · {project.deliverables_pending} à valider
                  <HugeiconsIcon icon={ArrowRight01Icon} size={14} strokeWidth={1.8} />
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      {data !== undefined && projects.length === 0 && (
        <p className="rounded-[14px] border border-dashed border-[#e3e4e6] bg-white p-8 text-center text-[15px] text-[#73757c]">
          Aucun projet pour l’instant. Ils apparaîtront ici dès que l’agence les aura ouverts.
        </p>
      )}

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        {projects.map((project) => (
          <ProjectCard key={project.id} project={project} />
        ))}
      </div>
    </div>
  )
}

function ProjectCard({ project }: { project: PortalProject }) {
  const status = PORTAL_PROJECT_STATUS[project.status]
  const next = project.next_milestone
  const nextDue = parseApiDate(next?.due_on)

  return (
    <Link
      to="/client/projets/$id"
      params={{ id: project.id }}
      className="flex flex-col gap-4 rounded-[14px] border border-[#e8e8e9] bg-white p-4 transition-shadow hover:shadow-[0_6px_20px_-8px_rgb(16_24_40/0.18)]"
    >
      <span className="flex items-start gap-3">
        <ProjectLogo url={project.logo_url} size={36} className="rounded-[8px]" />
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="text-[17px] font-medium text-[#1b1b1b]">{project.name}</span>
          {project.description !== '' && (
            <span className="line-clamp-2 text-[14px] text-[#73757c]">{project.description}</span>
          )}
        </span>
        <StatusPill label={status.label} color={status.color} pill={status.pill} />
      </span>

      {/* Heberge, le projet n'avance plus : une jauge figee a 100 % ne dirait
          rien au client, qui veut savoir que son site est entre de bonnes
          mains. */}
      {tracksSchedule(project.status) ? (
        <span className="flex flex-col gap-1.5">
          <span className="flex items-baseline justify-between text-[13px] text-[#73757c]">
            <span>Avancement</span>
            <span className="font-medium text-[#1b1b1b] tabular-nums">{project.progress} %</span>
          </span>
          <Meter ratio={project.progress} color={project.progress >= 100 ? DONE_COLOR : PROGRESS_COLOR} className="h-1.5" />
        </span>
      ) : (
        <span className="flex items-center gap-1.5 text-[13px] text-[#73757c]">
          <span aria-hidden className="size-1.5 rounded-full" style={{ background: status.color }} />
          Site en ligne, hébergé et suivi par l’agence
        </span>
      )}

      <span className="flex flex-col gap-1 border-t border-[#f3f4f4] pt-3 text-[13px]">
        <span className="flex items-center gap-1.5 text-[#4b4b4f]">
          <HugeiconsIcon icon={Flag02Icon} size={14} strokeWidth={1.8} className="shrink-0 text-[#8d8d8d]" />
          {next === null ? (
            <span className="text-[#8d8d8d]">Toutes les étapes sont atteintes</span>
          ) : (
            <span className="truncate">
              Prochaine étape : <span className="text-[#1b1b1b]">{next.title}</span>
              {nextDue !== null && ` · ${DATE.format(nextDue)}`}
            </span>
          )}
        </span>
        <span className="flex items-center justify-between gap-2 text-[#8d8d8d]">
          <span>Dernière activité {since(project.last_activity_at)}</span>
          {project.deliverables_pending > 0 && (
            <span className="rounded-full bg-brand px-2 py-0.5 text-[12px] font-medium text-white">
              {project.deliverables_pending} à valider
            </span>
          )}
        </span>
      </span>
    </Link>
  )
}
