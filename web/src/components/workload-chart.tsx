import { Calendar03Icon } from '@hugeicons/core-free-icons'

import { PanelCard } from '@/components/panel-card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { LIGHT_TOOLTIP } from '@/lib/tooltip'
import { cn } from '@/lib/utils'

/**
 * Natures d'heures, du bas de la barre vers le haut.
 *
 * Le rouge n'est pas une troisieme nature : c'est la part de la charge qui
 * passe au-dela du budget vendu. Un projet qui l'affiche est en depassement,
 * ce qui se lit sans avoir a comparer deux chiffres.
 */
const NATURES = [
  { key: 'confirmed', label: 'Confirmé', color: '#0db471' },
  { key: 'forecast', label: 'Prévisionnel', color: '#4956f4' },
  { key: 'over', label: 'Hors budget', color: '#e5484d' },
] as const

interface ProjectLoad {
  /** Etiquette de l'axe : abregee, une barre ne porte pas un nom entier. */
  short: string
  name: string
  confirmed: number
  forecast: number
  /** Part de la charge au-dela du budget vendu. */
  over: number
}

/**
 * Charge figee, par projet.
 *
 * Une seule serie : le graphique dit le volume d'heures que porte chaque
 * projet, decompose en trois natures. Il portait auparavant un selecteur de
 * fenetre (semaine, quinzaine, mois) qui multipliait la meme lecture par trois
 * sans rien en dire de plus.
 */
const LOAD: ProjectLoad[] = [
  { short: 'Vitrine', name: 'Refonte vitrine', confirmed: 26, forecast: 10, over: 0 },
  { short: 'Mobile', name: 'Application mobile', confirmed: 34, forecast: 14, over: 9 },
  { short: 'Institut.', name: 'Site institutionnel', confirmed: 18, forecast: 12, over: 0 },
  { short: 'Holding', name: 'Site vitrine holding', confirmed: 12, forecast: 18, over: 0 },
  { short: 'Identité', name: 'Identité visuelle', confirmed: 6, forecast: 14, over: 0 },
]

function formatHours(hours: number) {
  // Une decimale : la moyenne tombe rarement rond, et deux chiffres apres la
  // virgule donneraient une precision que des heures saisies a la demi-heure
  // n'ont pas.
  const rounded = Math.round(hours * 10) / 10

  return `${rounded.toString().replace('.', ',')} h`
}

/**
 * Graduations de l'axe : cinq paliers ronds au-dessus de la plus haute barre.
 *
 * Calculees et non figees — le plafond depend de la charge du moment, et une
 * echelle ecrite en dur ecraserait les barres le jour ou elle serait depassee.
 */
function axisOf(series: ProjectLoad[]) {
  const tallest = Math.max(...series.map((p) => p.confirmed + p.forecast + p.over))
  const step = Math.max(5, Math.ceil(tallest / 4 / 5) * 5)

  return { ceiling: step * 4, ticks: [4, 3, 2, 1, 0].map((level) => level * step) }
}

/**
 * Une colonne du graphique : sa barre, et l'infobulle qui la detaille.
 *
 * C'est le couloir entier qui declenche l'infobulle, pas la barre : viser 14px
 * de large a la souris demanderait de la precision pour rien, et une barre
 * courte serait presque impossible a survoler.
 *
 * Les natures se nommaient auparavant dans une legende au pied de la carte.
 * Elles se nomment desormais ici, avec leur part en heures : la legende disait
 * a quoi correspondaient trois couleurs, l'infobulle dit ce que vaut chacune
 * pour le projet qu'on regarde.
 */
function BarColumn({ project, ceiling }: { project: ProjectLoad; ceiling: number }) {
  const total = project.confirmed + project.forecast + project.over

  // De haut en bas : le hors-budget couronne la barre, le confirme la fonde.
  const parts = NATURES.map((nature) => ({ ...nature, hours: project[nature.key] }))
    .filter((part) => part.hours > 0)
    .reverse()

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div
          aria-label={`${project.name} : ${formatHours(total)}`}
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
        </div>
      </TooltipTrigger>

      <TooltipContent className={cn(LIGHT_TOOLTIP, 'flex-col items-start gap-1 px-3 py-2')}>
        <p className="text-[11px] leading-none text-[#777]">{project.name}</p>
        <p className="text-[13px] leading-none font-medium">{formatHours(total)}</p>

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

export function WorkloadChart() {
  const { ceiling, ticks } = axisOf(LOAD)

  // La moyenne, et non le total : le titre annonce une charge « par projet »,
  // c'est donc ce que le chiffre doit dire. Un total repondrait a une autre
  // question, celle de la charge de l'agence.
  const total = LOAD.reduce((sum, p) => sum + p.confirmed + p.forecast + p.over, 0)
  const average = LOAD.length === 0 ? 0 : total / LOAD.length

  return (
    <TooltipProvider>
      <PanelCard icon={Calendar03Icon} title="CHARGE PAR PROJET">
        <div className="flex flex-1 flex-col gap-3">
          <div className="flex items-baseline gap-2">
            <p className="text-[32px] leading-[1.3] font-semibold text-[#1f1f1f] tabular-nums">
              {formatHours(average)}
            </p>

            {/* Le chiffre seul se lirait comme un total. Trois mots suffisent a
                dire lequel des deux on regarde. */}
            <p className="text-[12px] leading-[1.5] text-[#8d8d8d]">en moyenne par projet</p>
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
                {/* Grille horizontale : les barres se lisent sinon a vue de nez
                    des que l'axe s'eloigne. */}
                {ticks.map((tick) => (
                  <div
                    key={tick}
                    aria-hidden
                    className="absolute inset-x-0 border-t border-dashed border-[#ebebeb]"
                    style={{ bottom: `${(tick / ceiling) * 100}%` }}
                  />
                ))}

                <div className="relative flex h-full items-end justify-between gap-1">
                  {LOAD.map((project) => (
                    <BarColumn key={project.short} project={project} ceiling={ceiling} />
                  ))}
                </div>
              </div>

              <div className="flex justify-between gap-1">
                {LOAD.map((project) => (
                  <p
                    key={project.short}
                    className="min-w-0 flex-1 truncate text-center text-[14px] leading-[1.5] tracking-[-0.28px] text-[#a8a7ab]"
                  >
                    {project.short}
                  </p>
                ))}
              </div>
            </div>
          </div>
        </div>
      </PanelCard>
    </TooltipProvider>
  )
}
