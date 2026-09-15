import { ChartHistogramIcon } from '@hugeicons/core-free-icons'
import { useQuery } from '@tanstack/react-query'

import { PanelCard } from '@/components/panel-card'
import { dashboardQuery } from '@/features/projects/api'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { LIGHT_TOOLTIP } from '@/lib/tooltip'
import { cn } from '@/lib/utils'
import type { TaskProgress } from '@/types/api'

/**
 * Etats d'avancement, dans l'ordre ou ils s'empilent sur la barre.
 *
 * Les couleurs sont celles du dessin. Elles restent categorielles et non
 * sequentielles : une echelle d'orange, comme sur la heatmap, dirait « plus ou
 * moins » la ou il faut lire « autre chose ».
 */
const STATES = [
  { key: 'pending', label: 'À faire', color: '#f06fff' },
  { key: 'progress', label: 'En cours', color: '#b872e6' },
  { key: 'done', label: 'Terminé', color: '#7c8bfe' },
  { key: 'planned', label: 'Planifié', color: '#f7f7f7' },
] as const

/**
 * Plafond de l'axe, deduit des donnees.
 *
 * Il etait fige a 50, ce qui convenait aux chiffres inventes et a rien
 * d'autre : une nature qui porterait soixante taches aurait deborde de sa
 * barre. On arrondit a la dizaine superieure pour que les six graduations
 * tombent rondes, et on garde 50 comme plancher — sous cette valeur, l'axe se
 * regraduerait a chaque tache creee, et les barres sauteraient d'un rendu a
 * l'autre sans que rien de visible ait change.
 */
function scaleOf(rows: TaskProgress[]) {
  const busiest = rows.reduce((top, row) => Math.max(top, row.todo + row.progress + row.done), 0)
  const max = Math.max(50, Math.ceil(busiest / 10) * 10)

  return { max, ticks: [0, 1, 2, 3, 4, 5].map((step) => (max / 5) * step) }
}

/** Ce qu'une ligne porte, planifie compris — le gris est ce qui reste a placer. */
function countsOf(row: TaskProgress, max: number) {
  const placed = row.todo + row.progress + row.done

  return {
    pending: row.todo,
    progress: row.progress,
    done: row.done,
    planned: Math.max(0, max - placed),
    placed,
  }
}

export function PerformanceReview() {
  const { data, isPending } = useQuery(dashboardQuery)

  const rows = data?.task_progress ?? []
  const { max, ticks } = scaleOf(rows)

  return (
    <TooltipProvider>
      <PanelCard icon={ChartHistogramIcon} title="AVANCEMENT DES TÂCHES">
        <div className="flex flex-1 flex-col gap-2.5">
          {/* Le panneau montrait trois domaines inventes quoi qu'il arrive. Il
              dit maintenant quand il n'a rien a montrer, plutot que d'afficher
              des barres qui ne correspondent a aucune tache. */}
          {!isPending && rows.length === 0 && (
            <p className="py-6 text-center text-[13px] text-[#8d8d8d]">
              Aucune tâche à afficher.
            </p>
          )}

          {isPending && (
            <div className="flex flex-col gap-[11px]">
              {[0, 1, 2].map((line) => (
                <div key={line} className="h-4 animate-pulse rounded-[4px] bg-[#f2f2f2]" />
              ))}
            </div>
          )}

          <div className={cn('flex flex-col gap-[11px]', rows.length === 0 && 'hidden')}>
            {rows.map((row) => {
              const counts = countsOf(row, max)

              return (
                // La ligne entiere declenche l'infobulle, libelle compris : viser
                // une barre de 16px de haut demanderait de la precision pour
                // rien, et le segment le plus court fait quelques pixels.
                <Tooltip key={row.tag}>
                  <TooltipTrigger asChild>
                    <div
                      aria-label={`${row.tag} : ${counts.placed} tâches placées`}
                      className="flex items-center justify-between gap-3"
                    >
                      {/* Largeur fixe pour que les trois barres partent du meme
                          bord : alignees sur le texte, elles commenceraient
                          chacune ailleurs et l'axe ne voudrait plus rien dire. */}
                      <p className="w-[92px] shrink-0 truncate text-[12px] leading-[1.5] tracking-[-0.24px] text-[#111]/80">
                        {row.tag}
                      </p>

                      {/* Le dessin fige la barre a 335px. Ici elle prend la place
                          restante, sinon elle deborderait du panneau ou
                          flotterait au milieu selon la largeur de la carte. */}
                      <div className="flex min-w-0 flex-1 items-center gap-1">
                        {STATES.filter((state) => state.key !== 'planned').map((state) => (
                          <div
                            key={state.key}
                            className="h-4 rounded-[4px]"
                            style={{
                              backgroundColor: state.color,
                              width: `${(counts[state.key] / max) * 100}%`,
                            }}
                          />
                        ))}

                        {/* Le reste a planifier occupe ce qui n'a pas ete pris :
                            en `flex-1`, il s'ajuste seul et la somme tombe
                            toujours juste. */}
                        <div className="h-4 min-w-px flex-1 rounded-[4px] bg-[#f7f7f7]" />
                      </div>
                    </div>
                  </TooltipTrigger>

                  {/* Les etats se nommaient dans une legende sous le graphique.
                      Ils se nomment desormais ici, avec leur compte : la legende
                      disait a quoi correspondaient quatre couleurs, l'infobulle
                      dit ce que vaut chacune pour la ligne qu'on regarde. */}
                  <TooltipContent className={cn(LIGHT_TOOLTIP, 'flex-col items-start gap-1 px-3 py-2')}>
                    <p className="text-[11px] leading-none text-[#777]">{row.tag}</p>
                    <p className="text-[13px] leading-none font-medium">
                      {counts.placed} tâches placées
                    </p>

                    <div className="flex flex-col gap-1 pt-0.5">
                      {STATES.map((state) => (
                        <p
                          key={state.key}
                          className="flex items-center gap-1.5 text-[11px] leading-none text-[#777]"
                        >
                          <span
                            aria-hidden
                            className="size-2 shrink-0 rounded-full"
                            style={{ backgroundColor: state.color }}
                          />
                          {state.label} · {counts[state.key]}
                        </p>
                      ))}
                    </div>
                  </TooltipContent>
                </Tooltip>
              )
            })}

            <div className="flex items-center justify-between pl-[104px] text-[12px] leading-[1.5] text-[#111]/50">
              {ticks.map((tick) => (
                <p key={tick}>{tick}</p>
              ))}
            </div>
          </div>
        </div>
      </PanelCard>
    </TooltipProvider>
  )
}
