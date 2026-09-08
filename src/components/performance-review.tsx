import { ChartHistogramIcon } from '@hugeicons/core-free-icons'

import { DashboardCard } from '@/components/dashboard-card'

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

export function PerformanceReview() {
  return (
    <DashboardCard icon={ChartHistogramIcon} title="AVANCEMENT DES TÂCHES">
      <div className="flex flex-1 flex-col gap-2.5">
        <div className="flex flex-col gap-[11px]">
          {ROWS.map((row) => (
            <div key={row.domain} className="flex items-center justify-between gap-3">
              {/* Largeur fixe pour que les trois barres partent du meme bord :
                  alignees sur le texte, elles commenceraient chacune ailleurs
                  et l'axe ne voudrait plus rien dire. */}
              <p className="w-[92px] shrink-0 truncate text-[12px] leading-[1.5] tracking-[-0.24px] text-[#111]/80">
                {row.domain}
              </p>

              {/* Le dessin fige la barre a 335px. Ici elle prend la place
                  restante, sinon elle deborderait du panneau ou flotterait au
                  milieu selon la largeur donnee a la carte. */}
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

                {/* Le reste a planifier occupe ce qui n'a pas ete pris : en
                    `flex-1`, il s'ajuste seul et la somme tombe toujours
                    juste. */}
                <div className="h-4 min-w-px flex-1 rounded-[4px] bg-[#f7f7f7]" />
              </div>
            </div>
          ))}

          <div className="flex items-center justify-between pl-[104px] text-[12px] leading-[1.5] text-[#111]/50">
            {SCALE.map((tick) => (
              <p key={tick}>{tick}</p>
            ))}
          </div>
        </div>

        <div className="flex flex-wrap justify-center gap-x-6 gap-y-2 border-t border-[#ebebeb] pt-2">
          {STATES.map((state) => (
            <div key={state.key} className="flex items-center gap-1.5">
              {/* Le dessin exporte ces pastilles en SVG. Un rond CSS les rend a
                  l'identique et garde la couleur au meme endroit que la barre
                  qu'il legende. */}
              <div
                aria-hidden
                className="size-2 shrink-0 rounded-full"
                style={{ backgroundColor: state.color }}
              />
              <p className="text-[12px] leading-[1.5] whitespace-nowrap text-[#00050a]">
                {state.label}
              </p>
            </div>
          ))}
        </div>
      </div>
    </DashboardCard>
  )
}
