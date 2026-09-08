import { ArrowUp01Icon, Calendar03Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { motion } from 'framer-motion'
import { useState } from 'react'

import { DashboardCard } from '@/components/dashboard-card'
import { useSlideTransition } from '@/lib/motion'
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

/**
 * Fenetres d'observation.
 *
 * Deux libelles chacune : en trois colonnes, la carte descend sous 200px et
 * « Quinzaine » a lui seul mange la moitie du groupe. Le nombre de jours dit
 * la meme chose en deux caracteres.
 */
const PERIODS = [
  { key: 'week', label: 'Semaine', short: '7 j' },
  { key: 'fortnight', label: 'Quinzaine', short: '15 j' },
  { key: 'month', label: 'Mois', short: '30 j' },
] as const

type PeriodKey = (typeof PERIODS)[number]['key']

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
 * Charge figee, par projet et par fenetre.
 *
 * Trois series distinctes plutot qu'une seule multipliee : un mois redistribue
 * la charge entre projets, il ne fait pas quatre fois la semaine.
 */
const LOAD: Record<PeriodKey, ProjectLoad[]> = {
  week: [
    { short: 'Vitrine', name: 'Refonte vitrine', confirmed: 8, forecast: 3, over: 0 },
    { short: 'Mobile', name: 'Application mobile', confirmed: 12, forecast: 4, over: 2 },
    { short: 'Institut.', name: 'Site institutionnel', confirmed: 5, forecast: 2, over: 0 },
    { short: 'Holding', name: 'Site vitrine holding', confirmed: 2, forecast: 4, over: 0 },
    { short: 'Identité', name: 'Identité visuelle', confirmed: 0, forecast: 3, over: 0 },
  ],
  fortnight: [
    { short: 'Vitrine', name: 'Refonte vitrine', confirmed: 14, forecast: 6, over: 0 },
    { short: 'Mobile', name: 'Application mobile', confirmed: 20, forecast: 8, over: 5 },
    { short: 'Institut.', name: 'Site institutionnel', confirmed: 10, forecast: 6, over: 0 },
    { short: 'Holding', name: 'Site vitrine holding', confirmed: 6, forecast: 9, over: 0 },
    { short: 'Identité', name: 'Identité visuelle', confirmed: 2, forecast: 7, over: 0 },
  ],
  month: [
    { short: 'Vitrine', name: 'Refonte vitrine', confirmed: 26, forecast: 10, over: 0 },
    { short: 'Mobile', name: 'Application mobile', confirmed: 34, forecast: 14, over: 9 },
    { short: 'Institut.', name: 'Site institutionnel', confirmed: 18, forecast: 12, over: 0 },
    { short: 'Holding', name: 'Site vitrine holding', confirmed: 12, forecast: 18, over: 0 },
    { short: 'Identité', name: 'Identité visuelle', confirmed: 6, forecast: 14, over: 0 },
  ],
}

function formatHours(hours: number) {
  return `${hours.toString().replace('.', ',')} h`
}

/**
 * Graduations de l'axe : cinq paliers ronds au-dessus de la plus haute barre.
 *
 * Calculees et non figees — le plafond d'une semaine n'est pas celui d'un
 * mois, et une echelle commune ecraserait la premiere.
 */
function axisOf(series: ProjectLoad[]) {
  const tallest = Math.max(...series.map((p) => p.confirmed + p.forecast + p.over))
  const step = Math.max(5, Math.ceil(tallest / 4 / 5) * 5)

  return { ceiling: step * 4, ticks: [4, 3, 2, 1, 0].map((level) => level * step) }
}

/** Une barre : ses natures empilees, rendues de haut en bas. */
function BarGroup({ project, ceiling }: { project: ProjectLoad; ceiling: number }) {
  const total = project.confirmed + project.forecast + project.over

  const parts = [
    { key: 'over', color: '#e5484d', hours: project.over },
    { key: 'forecast', color: '#4956f4', hours: project.forecast },
    { key: 'confirmed', color: '#0db471', hours: project.confirmed },
  ].filter((part) => part.hours > 0)

  return (
    <div
      className="flex w-full max-w-[14px] flex-col justify-end gap-[3px]"
      style={{ height: `${(total / ceiling) * 100}%` }}
    >
      {parts.map((part) => (
        <div
          key={part.key}
          // Le reflet interne est ce qui fait la capsule plutot que le
          // rectangle : sans lui, l'arrondi seul parait plat.
          className="w-full shrink-0 rounded-full shadow-[inset_0_4px_8px_0_rgb(255_255_255/0.29)]"
          style={{ backgroundColor: part.color, height: `${(part.hours / total) * 100}%` }}
        />
      ))}
    </div>
  )
}

export function WorkloadChart() {
  const [period, setPeriod] = useState<PeriodKey>('fortnight')

  const transition = useSlideTransition()

  const series = LOAD[period]
  const { ceiling, ticks } = axisOf(series)

  const planned = series.reduce((sum, p) => sum + p.confirmed + p.forecast + p.over, 0)
  const overrun = series.filter((p) => p.over > 0).length

  return (
    <DashboardCard icon={Calendar03Icon} title="CHARGE PAR PROJET">
      <div className="flex flex-1 flex-col gap-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-3">
              <p className="text-[32px] leading-[1.3] font-semibold text-[#1f1f1f] tabular-nums">
                {formatHours(planned)}
              </p>

              <div className="flex h-6 items-center gap-0.5 rounded-[40px] border border-[#e1f4df] bg-[#f1faf0] px-2 py-1">
                <HugeiconsIcon
                  icon={ArrowUp01Icon}
                  size={14}
                  strokeWidth={2}
                  className="text-[#4fbb46]"
                />
                <p className="text-[12px] leading-[1.5] font-semibold text-[#4fbb46]">7,3 %</p>
              </div>
            </div>

            <p className="flex flex-wrap items-center gap-1 text-[12px] leading-[1.5]">
              <span className="font-medium text-[#8d8d8d]">Réparties sur</span>
              <span className="font-semibold text-[#009d5c]">{series.length} projets</span>
              <span className="font-medium text-[#8d8d8d]">
                {overrun > 0 ? `dont ${overrun} hors budget` : 'tous dans le budget'}
              </span>
            </p>
          </div>

          {/* Meme mecanique que le rail de modules : pas de rembourrage sur le
              groupe, le retrait de 2px est porte par la pastille. Chaque case
              garde donc sa largeur quel que soit son etat, et rien ne bouge
              quand la selection change. */}
          <div className="flex items-center gap-1 rounded-[12px] bg-[#f0f0f0]">
            {PERIODS.map((entry) => {
              const active = entry.key === period

              return (
                <button
                  key={entry.key}
                  type="button"
                  aria-pressed={active}
                  onClick={() => setPeriod(entry.key)}
                  className="relative flex h-10 items-center justify-center rounded-[10px] px-2 @sm:px-3"
                >
                  {active && (
                    // Un seul noeud pour tout le groupe : il partage le
                    // `layoutId`, donc Framer Motion l'interpole de sa position
                    // precedente vers la nouvelle au lieu de le faire
                    // disparaitre ici et reapparaitre la.
                    <motion.span
                      layoutId="workload-period-highlight"
                      transition={transition}
                      className="absolute inset-[2px] rounded-[10px] border border-[#e6e6e6] bg-white"
                    />
                  )}

                  {/* Au-dessus de la pastille : sans quoi le libelle
                      disparaitrait derriere elle pendant le glissement. */}
                  <span
                    className={cn(
                      'relative z-10 text-[14px] leading-[1.5] font-medium',
                      active ? 'text-[#111]' : 'text-[#999]',
                    )}
                  >
                    <span className="@sm:hidden">{entry.short}</span>
                    <span className="hidden @sm:inline">{entry.label}</span>
                  </span>
                </button>
              )
            })}
          </div>
        </div>

        <div className="flex flex-1 gap-2">
          {/* L'axe porte la meme hauteur que la zone tracee, sinon ses
              graduations ne tomberaient pas sur les bonnes hauteurs. */}
          <div className="flex h-[180px] shrink-0 flex-col justify-between text-right text-[12px] leading-none text-[#666] tabular-nums">
            {ticks.map((tick) => (
              <p key={tick}>{tick} h</p>
            ))}
          </div>

          <div className="flex min-w-0 flex-1 flex-col gap-2">
            <div className="relative h-[180px]">
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
                {series.map((project) => (
                  <div
                    key={project.short}
                    className="flex h-full min-w-0 flex-1 justify-center"
                    title={`${project.name} — ${formatHours(project.confirmed + project.forecast + project.over)}`}
                  >
                    <BarGroup project={project} ceiling={ceiling} />
                  </div>
                ))}
              </div>
            </div>

            <div className="flex justify-between gap-1">
              {series.map((project) => (
                <p
                  key={project.short}
                  className="min-w-0 flex-1 truncate text-center text-[12px] leading-[1.5] tracking-[-0.28px] text-[#a8a7ab]"
                >
                  {project.short}
                </p>
              ))}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-center gap-x-[11px] gap-y-2 border-t border-[#ebebeb] pt-2">
          {NATURES.map((nature) => (
            <div key={nature.key} className="flex items-center gap-2">
              <div
                aria-hidden
                className="size-2 shrink-0 rounded-[2px]"
                style={{ backgroundColor: nature.color }}
              />
              <p className="text-[12px] leading-[1.5] font-medium whitespace-nowrap text-[#030512]">
                {nature.label}
              </p>
            </div>
          ))}
        </div>
      </div>
    </DashboardCard>
  )
}
