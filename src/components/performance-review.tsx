import { ChartHistogramIcon } from '@hugeicons/core-free-icons'

import { PanelCard } from '@/components/panel-card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { LIGHT_TOOLTIP } from '@/lib/tooltip'
import { cn } from '@/lib/utils'

/**
 * Etats d'avancement, dans l'ordre ou ils s'empilent sur la barre.
 *
 * Les couleurs sont celles du dessin. Elles restent categorielles et non
 * sequentielles : une echelle d'orange, comme sur la heatmap, dirait « plus ou
 * moins » la ou il faut lire « autre chose ».
 */
const STATES = [
  { key: 'pending', label: 'En attente', color: '#f06fff' },
  { key: 'progress', label: 'En cours', color: '#b872e6' },
  { key: 'done', label: 'Terminé', color: '#7c8bfe' },
  { key: 'planned', label: 'Planifié', color: '#f7f7f7' },
] as const

/** Graduation de l'axe, et donc plafond d'une ligne. */
const SCALE = [0, 10, 20, 30, 40, 50]
const MAX = 50

/** Figes : le module n'a pas encore de quoi compter ses taches. */
const ROWS = [
  { domain: 'Design UI/UX', pending: 7, progress: 7, done: 7 },
  { domain: 'Développement', pending: 4, progress: 3, done: 9 },
  { domain: 'Marketing', pending: 9, progress: 6, done: 6 },
]

/** Ce qu'une ligne porte, planifie compris — le gris est ce qui reste a placer. */
function countsOf(row: (typeof ROWS)[number]) {
  const placed = row.pending + row.progress + row.done

  return {
    pending: row.pending,
    progress: row.progress,
    done: row.done,
    planned: Math.max(0, MAX - placed),
    placed,
  }
}

export function PerformanceReview() {
  return (
    <TooltipProvider>
      <PanelCard icon={ChartHistogramIcon} title="AVANCEMENT DES TÂCHES">
        <div className="flex flex-1 flex-col gap-2.5">
          <div className="flex flex-col gap-[11px]">
            {ROWS.map((row) => {
              const counts = countsOf(row)

              return (
                // La ligne entiere declenche l'infobulle, libelle compris : viser
                // une barre de 16px de haut demanderait de la precision pour
                // rien, et le segment le plus court fait quelques pixels.
                <Tooltip key={row.domain}>
                  <TooltipTrigger asChild>
                    <div
                      aria-label={`${row.domain} : ${counts.placed} tâches placées`}
                      className="flex items-center justify-between gap-3"
                    >
                      {/* Largeur fixe pour que les trois barres partent du meme
                          bord : alignees sur le texte, elles commenceraient
                          chacune ailleurs et l'axe ne voudrait plus rien dire. */}
                      <p className="w-[92px] shrink-0 truncate text-[12px] leading-[1.5] tracking-[-0.24px] text-[#111]/80">
                        {row.domain}
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
                              width: `${(row[state.key] / MAX) * 100}%`,
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
                    <p className="text-[11px] leading-none text-[#777]">{row.domain}</p>
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
              {SCALE.map((tick) => (
                <p key={tick}>{tick}</p>
              ))}
            </div>
          </div>
        </div>
      </PanelCard>
    </TooltipProvider>
  )
}
