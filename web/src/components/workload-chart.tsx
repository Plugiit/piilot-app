import { Calendar03Icon } from '@hugeicons/core-free-icons'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { PanelCard } from '@/components/panel-card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { dashboardQuery } from '@/features/projects/api'
import { BUDGET_STATE, formatHours } from '@/features/projects/format'
import { LIGHT_TOOLTIP } from '@/lib/tooltip'
import { cn } from '@/lib/utils'
import type { ProjectLoad } from '@/types/api'

/**
 * Parts d'une barre, du bas vers le haut.
 *
 * Le rouge n'est pas une troisieme nature : c'est la part des heures saisies
 * au-dela du budget vendu. Il prend la place du restant, qui n'existe plus des
 * qu'il y a depassement — un projet qui l'affiche se lit sans comparer deux
 * chiffres.
 */
const PARTS = [
  { key: 'consumed', label: 'Consommé', color: '#4956f4' },
  { key: 'remaining', label: 'Restant', color: '#e6e6e6' },
  { key: 'over', label: 'Hors budget', color: '#e5484d' },
] as const

type PartKey = (typeof PARTS)[number]['key']

/** Les trois parts d'un projet, en heures. Leur somme est la hauteur de sa barre. */
function partsOf(project: ProjectLoad): Record<PartKey, number> {
  return {
    consumed: Math.min(project.hours_spent, project.hours_sold),
    remaining: Math.max(project.hours_sold - project.hours_spent, 0),
    over: Math.max(project.hours_spent - project.hours_sold, 0),
  }
}

function heightOf(project: ProjectLoad) {
  return Math.max(project.hours_sold, project.hours_spent)
}

/**
 * Graduations de l'axe : cinq paliers ronds au-dessus de la plus haute barre.
 *
 * Calculees et non figees — le plafond depend des budgets du moment, et une
 * echelle ecrite en dur ecraserait les barres le jour ou elle serait depassee.
 */
function axisOf(series: ProjectLoad[]) {
  const tallest = Math.max(0, ...series.map(heightOf))
  const step = Math.max(5, Math.ceil(tallest / 4 / 5) * 5)

  return { ceiling: step * 4, ticks: [4, 3, 2, 1, 0].map((level) => level * step) }
}

/**
 * Une colonne du graphique : sa barre, l'infobulle qui la detaille, et le lien
 * vers le projet.
 *
 * C'est le couloir entier qui declenche l'infobulle, pas la barre : viser 14px
 * de large a la souris demanderait de la precision pour rien, et une barre
 * courte serait presque impossible a survoler.
 */
function BarColumn({ project, ceiling }: { project: ProjectLoad; ceiling: number }) {
  const hours = partsOf(project)
  const total = heightOf(project)
  const ratio = Math.round((project.hours_spent / project.hours_sold) * 100)

  // De haut en bas : le hors-budget couronne la barre, le consomme la fonde.
  const parts = PARTS.map((part) => ({ ...part, hours: hours[part.key] }))
    .filter((part) => part.hours > 0)
    .reverse()

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Link
          to="/pm/projets/$id"
          params={{ id: project.id }}
          aria-label={`${project.name} : ${ratio} % du budget consommé`}
          className="flex h-full min-w-0 flex-1 items-end justify-center"
        >
          {/* `items-end` sur la colonne : sans lui, la barre — qui tient sa
              hauteur d'un pourcentage — se cale en haut de son couloir et
              flotte au-dessus de l'axe. */}
          <div
            className="flex w-full max-w-[14px] flex-col justify-end gap-[3px]"
            style={{ height: `${(total / ceiling) * 100}%` }}
          >
            {parts.map((part) => (
              <div
                key={part.key}
                className="w-full shrink-0 rounded-full"
                style={{ backgroundColor: part.color, height: `${(part.hours / total) * 100}%` }}
              />
            ))}
          </div>
        </Link>
      </TooltipTrigger>

      <TooltipContent className={cn(LIGHT_TOOLTIP, 'flex-col items-start gap-1 px-3 py-2')}>
        <p className="text-[11px] leading-none text-[#777]">{project.client_name}</p>
        <p className="text-[13px] leading-none font-medium">{project.name}</p>
        <p
          className="text-[11px] leading-none font-medium"
          style={{ color: BUDGET_STATE[project.budget_state].color }}
        >
          {ratio} % du budget · {BUDGET_STATE[project.budget_state].label}
        </p>

        <div className="flex flex-col gap-1 pt-0.5">
          {parts.map((part) => (
            <p key={part.key} className="flex items-center gap-1.5 text-[11px] leading-none text-[#777]">
              <span
                aria-hidden
                className="size-2 shrink-0 rounded-[2px]"
                style={{ backgroundColor: part.color }}
              />
              {part.label} · {formatHours(part.hours)}
            </p>
          ))}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}

/**
 * Charge par projet : les projets en cours les plus avances dans leur budget.
 *
 * L'API en renvoie huit, tries sur la part consommee. Le chiffre de tete
 * compte les depassements de tous les projets en cours, pas seulement de ceux
 * affiches : c'est lui qui dit s'il faut ouvrir la liste filtree.
 */
export function WorkloadChart() {
  const { data, isPending } = useQuery(dashboardQuery)
  const load = data?.workload ?? []
  const over = data?.budget.over ?? 0
  const warning = data?.budget.warning ?? 0
  const { ceiling, ticks } = axisOf(load)

  return (
    <TooltipProvider>
      <PanelCard
        icon={Calendar03Icon}
        title="CHARGE PAR PROJET"
        action={
          <Link
            to="/pm/projets"
            search={{ budget: over > 0 ? 'over' : 'warning', sort: 'budget', dir: 'desc', page: 1 }}
            className="text-[12px] whitespace-nowrap text-[#64748b] hover:text-[#111] hover:underline"
          >
            Voir les budgets
          </Link>
        }
      >
        <div className="flex flex-1 flex-col gap-3">
          {isPending && <div className="h-[260px] animate-pulse rounded-[4px] bg-[#f2f2f2]" />}

          {!isPending && load.length === 0 && (
            <p className="py-6 text-center text-[13px] text-[#8d8d8d]">
              Aucun projet en cours avec des heures vendues.
            </p>
          )}

          {!isPending && load.length > 0 && (
            <>
              <div className="flex items-baseline gap-2">
                <p
                  className="text-[32px] leading-[1.3] font-semibold tabular-nums"
                  style={{ color: over > 0 ? BUDGET_STATE.over.color : '#1f1f1f' }}
                >
                  {over}
                </p>

                <p className="text-[12px] leading-[1.5] text-[#8d8d8d]">
                  {over > 1 ? 'projets hors budget' : 'projet hors budget'}
                  {warning > 0 && ` · ${warning} à surveiller`}
                </p>
              </div>

              <div className="flex flex-1 gap-2">
                {/* L'axe porte la meme hauteur que la zone tracee, sinon ses
                    graduations ne tomberaient pas sur les bonnes hauteurs. */}
                <div className="flex h-[186px] shrink-0 flex-col justify-between text-right text-[14px] leading-none text-[#666] tabular-nums">
                  {ticks.map((tick) => (
                    <p key={tick}>{tick} h</p>
                  ))}
                </div>

                <div className="flex min-w-0 flex-1 flex-col gap-2">
                  <div className="relative h-[186px]">
                    {/* Grille horizontale : les barres se lisent sinon a vue de
                        nez des que l'axe s'eloigne. */}
                    {ticks.map((tick) => (
                      <div
                        key={tick}
                        aria-hidden
                        className="absolute inset-x-0 border-t border-dashed border-[#ebebeb]"
                        style={{ bottom: `${(tick / ceiling) * 100}%` }}
                      />
                    ))}

                    <div className="relative flex h-full items-end justify-between gap-1">
                      {load.map((project) => (
                        <BarColumn key={project.id} project={project} ceiling={ceiling} />
                      ))}
                    </div>
                  </div>

                  {/* Le client et non le projet sous la barre : « Refonte du
                      site… » revient sur la moitie des projets, le client les
                      distingue. Le nom complet est dans l'infobulle. */}
                  <div className="flex justify-between gap-1">
                    {load.map((project) => (
                      <p
                        key={project.id}
                        title={project.name}
                        className="min-w-0 flex-1 truncate text-center text-[12px] leading-[1.5] text-[#a8a7ab]"
                      >
                        {project.client_name}
                      </p>
                    ))}
                  </div>
                </div>
              </div>
            </>
          )}
        </div>
      </PanelCard>
    </TooltipProvider>
  )
}
