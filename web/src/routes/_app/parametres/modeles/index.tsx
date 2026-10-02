import { CheckmarkSquare02Icon, Flag02Icon, LayoutTable01Icon, PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'

import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { templateListQuery } from '@/features/templates/api'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'

/**
 * Modeles de projet.
 *
 * De quoi demarrer un projet type — site vitrine, refonte, maintenance — avec
 * ses jalons, ses taches et ses services deja poses. On choisit le modele a la
 * creation du projet ; le projet en recoit une copie, que l'on adapte ensuite.
 */
export const Route = createFileRoute('/_app/parametres/modeles/')({
  loader: ({ context }) => context.queryClient.query({ ...templateListQuery(), staleTime: 'static' }),
  component: ModelesPage,
})

const UPDATED = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })

function ModelesPage() {
  const { data, isError, error } = useQuery(templateListQuery())
  const { user } = Route.useRouteContext()
  const canEdit = can(user, 'projects.write')
  const items = data?.items ?? []

  return (
    <PageFrame title="Modèles de projet">
      <div className="flex min-h-full flex-col gap-4 p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="max-w-[560px] text-[13px] text-[#73757c]">
            Un modèle pose les jalons, les tâches et les services d’un projet type. Les échéances sont comptées
            en jours depuis le début du projet.
          </p>
          {canEdit && (
            <Button asChild size="lg" className="gap-1.5">
              <Link to="/parametres/modeles/$id" params={{ id: 'nouveau' }}>
                <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
                Nouveau modèle
              </Link>
            </Button>
          )}
        </div>

        {isError && (
          <p className="rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
            {error instanceof HttpError ? error.message : 'Chargement impossible'}
          </p>
        )}

        {!isError && items.length === 0 && (
          <div className="flex flex-col items-center gap-2 rounded-[12px] border border-dashed border-[#e8e8e9] p-10 text-center">
            <HugeiconsIcon icon={LayoutTable01Icon} size={22} strokeWidth={1.6} className="text-[#a2a3a7]" />
            <p className="text-[15px] font-medium text-[#1b1b1b]">Aucun modèle pour l’instant</p>
            <p className="max-w-[420px] text-[13px] text-[#73757c]">
              Commencez par le projet que l’agence lance le plus souvent : ses étapes et ses tâches habituelles.
            </p>
          </div>
        )}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {items.map((template) => (
            <Link
              key={template.id}
              to="/parametres/modeles/$id"
              params={{ id: template.id }}
              className="flex flex-col gap-2 rounded-[12px] border border-[#e8e8e9] bg-white p-3 transition-colors hover:border-[#d8d8d8] hover:bg-[#fcfcfc]"
            >
              <span className="text-[15px] font-medium text-[#1b1b1b]">{template.name}</span>
              {template.description !== '' && (
                <span className="line-clamp-2 text-[13px] text-[#73757c]">{template.description}</span>
              )}
              <span className="mt-auto flex items-center gap-3 pt-1 text-[12px] text-[#73757c]">
                <span className="flex items-center gap-1">
                  <HugeiconsIcon icon={Flag02Icon} size={13} strokeWidth={1.8} />
                  {template.milestones} jalon{template.milestones > 1 ? 's' : ''}
                </span>
                <span className="flex items-center gap-1">
                  <HugeiconsIcon icon={CheckmarkSquare02Icon} size={13} strokeWidth={1.8} />
                  {template.tasks} tâche{template.tasks > 1 ? 's' : ''}
                </span>
                <span className="ml-auto text-[#a2a3a7]">Modifié le {UPDATED.format(new Date(template.updated_at))}</span>
              </span>
            </Link>
          ))}
        </div>
      </div>
    </PageFrame>
  )
}
