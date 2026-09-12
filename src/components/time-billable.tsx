import { Clock01Icon } from '@hugeicons/core-free-icons'

import { PanelCard } from '@/components/panel-card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { LIGHT_TOOLTIP } from '@/lib/tooltip'
import { cn } from '@/lib/utils'

/**
 * Temps de la periode, en secondes.
 *
 * Figes, mais coherents entre eux : les chiffres affiches et les longueurs de
 * la barre sortent des memes valeurs, donc l'un ne peut pas contredire
 * l'autre. Le dessin, lui, annonce un total qui ne tombe pas juste avec ses
 * propres postes.
 */
const BUDGET = 12 * 3600
const BILLABLE = 3 * 3600 + 24 * 60 + 7
const INTERNAL = 4 * 3600 + 51 * 60 + 38
const REMAINING = BUDGET - BILLABLE - INTERNAL

/** Montant deja facturable, au taux de l'agence. */
const AMOUNT = 1020

const SEGMENTS = [
  { label: 'Non facturable', color: '#7bd25b', seconds: INTERNAL },
  { label: 'Facturable', color: '#4956f4', seconds: BILLABLE },
  { label: 'Restant', color: '#e6e6e6', seconds: REMAINING },
]

const AMOUNT_FORMAT = new Intl.NumberFormat('fr-FR', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
})

function formatDuration(seconds: number) {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  return [hours, minutes, seconds % 60].map((part) => String(part).padStart(2, '0')).join(':')
}

/**
 * Nombre de crans de la jauge.
 *
 * La barre etait peinte par un degrade repete au pas de 5px, soit plus de cent
 * raies sur la largeur d'une carte — trop fines pour se compter, et surtout
 * recalees a zero au debut de chaque poste : la derniere raie d'un poste
 * touchait la premiere du suivant, et une raie sur deux se retrouvait coupee.
 *
 * Des crans reels, repartis par une grille flex, resolvent les deux : ils sont
 * assez larges pour se lire, et leur ecart est tenu par un `gap` qu'aucun
 * arrondi ne peut refermer.
 *
 * Soixante-quatre : a demi-chemin entre les cent-vingt raies d'origine, trop
 * fines pour se distinguer, et les trente-deux d'un premier essai, ou la jauge
 * se lisait comme une suite de blocs plutot que comme une trame.
 */
const TICKS = 64

/**
 * Poste auquel appartient un cran.
 *
 * Le milieu du cran decide, pas son bord : une frontiere qui tombe au milieu
 * d'un cran le donne au poste qui en occupe la plus grande part, plutot que de
 * le couper en deux.
 */
function segmentAt(index: number) {
  const position = ((index + 0.5) / TICKS) * BUDGET

  let cumulated = 0
  for (const segment of SEGMENTS) {
    cumulated += segment.seconds
    if (position <= cumulated) return segment
  }

  return SEGMENTS[SEGMENTS.length - 1]!
}

const SUMMARY = [
  { label: 'Heures saisies', value: formatDuration(BILLABLE + INTERNAL) },
  { label: 'Heures facturables', value: formatDuration(BILLABLE) },
  { label: 'Heures restantes', value: formatDuration(REMAINING) },
  { label: 'Montant facturable', value: AMOUNT_FORMAT.format(AMOUNT) },
]

export function TimeBillable() {
  return (
    <TooltipProvider>
      <PanelCard icon={Clock01Icon} title="TEMPS FACTURABLE">
        <div className="flex flex-1 flex-col justify-between gap-2.5">
          {/* Le dessin aligne les quatre chiffres sur une ligne fixe, separes par
              des filets. En grille ils passent a deux colonnes quand la carte se
              resserre, et le filet suit — c'est celui de gauche, donc il tombe
              de lui-meme en debut de ligne. */}
          <div className="grid grid-cols-2 gap-y-3 @xl:grid-cols-4">
            {SUMMARY.map((entry, index) => (
              <div
                key={entry.label}
                className={cn(
                  'flex flex-col gap-0.5 px-3 first:pl-0',
                  index % 2 === 1 && 'border-l border-[#e6e6e6]',
                  '@xl:border-l @xl:first:border-l-0',
                )}
              >
                <p className="truncate text-[12px] text-[#64748b]">{entry.label}</p>
                <p className="text-[12px] font-semibold text-[#0f172a] tabular-nums">{entry.value}</p>
              </div>
            ))}
          </div>

          {/* Les postes se nommaient dans une legende sous la barre. Ils se
              nomment desormais dans l'infobulle, avec leur duree : la legende
              disait a quoi correspondaient trois couleurs, l'infobulle dit ce que
              vaut chacune. */}
          <Tooltip>
            <TooltipTrigger asChild>
              <div
                aria-label={SEGMENTS.map((s) => `${s.label} : ${formatDuration(s.seconds)}`).join(', ')}
                className="flex h-[38px] items-stretch gap-0.5"
              >
                {Array.from({ length: TICKS }, (_, index) => (
                  <span
                    key={index}
                    aria-hidden
                    className="min-w-px flex-1 rounded-[2px]"
                    style={{ backgroundColor: segmentAt(index).color }}
                  />
                ))}
              </div>
            </TooltipTrigger>

            <TooltipContent className={cn(LIGHT_TOOLTIP, 'flex-col items-start gap-1 px-3 py-2')}>
              <p className="text-[11px] leading-none text-[#777]">
                Budget de {formatDuration(BUDGET)}
              </p>

              <div className="flex flex-col gap-1 pt-0.5">
                {SEGMENTS.map((segment) => (
                  <p
                    key={segment.label}
                    className="flex items-center gap-1.5 text-[11px] leading-none text-[#777]"
                  >
                    <span
                      aria-hidden
                      className="size-2 shrink-0 rounded-[2px]"
                      style={{ backgroundColor: segment.color }}
                    />
                    {segment.label} · {formatDuration(segment.seconds)}
                  </p>
                ))}
              </div>
            </TooltipContent>
          </Tooltip>
        </div>
      </PanelCard>
    </TooltipProvider>
  )
}
