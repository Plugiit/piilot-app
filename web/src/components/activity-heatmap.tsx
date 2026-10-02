import { Activity03Icon, InformationCircleIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import { PanelCard } from '@/components/panel-card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { dashboardQuery } from '@/features/projects/api'
import { LIGHT_TOOLTIP } from '@/lib/tooltip'
import { cn } from '@/lib/utils'
import type { DayActivity as ApiDay } from '@/types/api'

/**
 * Paliers d'intensite, du vide au plus soutenu.
 *
 * L'echelle est une montee d'opacite sur l'orange de marque : quatre paliers
 * suffisent a lire une intensite sans inventer une palette. La journee sans
 * activite reste grise plutot qu'orange tres pale — sinon « rien » et « peu »
 * se confondent.
 *
 * Les classes sont ecrites en toutes lettres : Tailwind scanne le source, une
 * classe assemblee a l'execution ne serait jamais generee.
 */
const LEVELS = [
  'bg-[#f7f7f7]',
  'bg-[#ff782b]/25',
  'bg-[#ff782b]/50',
  'bg-[#ff782b]/75',
  'bg-[#ff782b]',
]

/** Palier d'une journee, d'apres son nombre d'activites. */
function levelOf(total: number) {
  if (total === 0) return 0
  if (total <= 2) return 1
  if (total <= 5) return 2
  if (total <= 9) return 3
  return 4
}

const MONTH_LABELS = Array.from({ length: 12 }, (_, month) =>
  new Intl.DateTimeFormat('fr-FR', { month: 'short' }).format(new Date(2000, month, 1)),
)

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', {
  weekday: 'long',
  day: 'numeric',
  month: 'long',
})

interface DayActivity {
  date: Date
  tasks: number
  tickets: number
  deliverables: number
  total: number
}

/** Cle d'une journee, au format des dates de l'API (« AAAA-MM-JJ »). */
function keyOf(year: number, month: number, day: number) {
  return `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`
}

/**
 * Les douze mois de `year`, chacun portant une journee par jour reel.
 *
 * L'API ne renvoie que les jours actifs : un jour absent est un jour vide, et
 * c'est ici que le calendrier complet se reconstitue.
 */
function buildYear(year: number, activity: ApiDay[]) {
  const byDay = new Map(activity.map((day) => [day.day, day]))

  return MONTH_LABELS.map((label, month) => ({
    label,
    // Le jour 0 du mois suivant est le dernier du mois courant : fevrier
    // compte donc 29 cases les annees bissextiles, sans table a maintenir.
    days: Array.from({ length: new Date(year, month + 1, 0).getDate() }, (_, index): DayActivity => {
      const found = byDay.get(keyOf(year, month, index + 1))
      const tasks = found?.tasks ?? 0
      const tickets = found?.tickets ?? 0
      const deliverables = found?.deliverables ?? 0

      return {
        date: new Date(year, month, index + 1),
        tasks,
        tickets,
        deliverables,
        total: tasks + tickets + deliverables,
      }
    }),
  }))
}

function plural(count: number, one: string, many: string) {
  return `${count} ${count > 1 ? many : one}`
}

/**
 * Une journee : la case coloree et ce qu'elle raconte au survol.
 *
 * La case n'est pas focusable. Une heatmap d'annee poserait 365 arrets de
 * tabulation avant le contenu suivant, ce qui couterait plus a la navigation
 * au clavier que le detail n'y apporte — il reste lisible par `aria-label`,
 * et l'agregat qu'il decompose sera de toute facon affiche ailleurs.
 */
function DayCell({ day }: { day: DayActivity }) {
  const date = DATE_FORMAT.format(day.date)
  const summary = day.total === 0 ? 'aucune activité' : plural(day.total, 'activité', 'activités')

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div
          aria-label={`${date} : ${summary}`}
          className={cn('aspect-square rounded-[4px]', LEVELS[levelOf(day.total)])}
        />
      </TooltipTrigger>

      <TooltipContent className={cn(LIGHT_TOOLTIP, 'flex-col items-start gap-1 px-3 py-2')}>
        <p className="text-[11px] leading-none text-[#777] first-letter:uppercase">{date}</p>

        {day.total === 0 ? (
          <p className="text-[13px] leading-none font-medium">Aucune activité</p>
        ) : (
          <>
            <p className="text-[13px] leading-none font-medium">
              {plural(day.total, 'activité', 'activités')}
            </p>
            <p className="text-[11px] leading-none text-[#777]">
              {[
                day.tasks > 0 && plural(day.tasks, 'tâche', 'tâches'),
                day.tickets > 0 && plural(day.tickets, 'ticket', 'tickets'),
                day.deliverables > 0 && plural(day.deliverables, 'livrable', 'livrables'),
              ]
                .filter(Boolean)
                .join(' · ')}
            </p>
          </>
        )}
      </TooltipContent>
    </Tooltip>
  )
}

/**
 * Activite de l'annee, une case par jour : taches terminees, tickets ouverts
 * et versions de livrables deposees.
 *
 * Vit hors de `components/ui/`, reserve aux composants shadcn — voir la note
 * de `stat-card.tsx`.
 *
 * Les nombres viennent d'une table precalculee par l'API, un compteur par
 * jour tenu par declencheur : la carte ne coute aucun comptage.
 */
export function ActivityHeatmap() {
  const { data } = useQuery(dashboardQuery)
  const activity = data?.activity
  // L'API renvoie l'annee civile en cours : la grille couvre la meme.
  const year = new Date().getFullYear()
  const months = useMemo(() => buildYear(year, activity ?? []), [year, activity])

  return (
    // Un seul fournisseur pour toute la grille : les 365 infobulles partagent
    // son delai, et une seule peut etre ouverte a la fois.
    <TooltipProvider>
      <PanelCard
        icon={Activity03Icon}
        title="ACTIVITÉ PAR JOUR"
        action={
          <Tooltip>
            <TooltipTrigger className="flex shrink-0 items-center text-[#606060]">
              <HugeiconsIcon icon={InformationCircleIcon} size={16} strokeWidth={1.6} />
              <span className="sr-only">À propos de ce graphique</span>
            </TooltipTrigger>
            <TooltipContent className={LIGHT_TOOLTIP}>
              Une case par jour : tâches terminées, tickets ouverts et livrables déposés. Plus elle est soutenue, plus l'activité a été forte.
            </TooltipContent>
          </Tooltip>
        }
      >
        {/* Les ruptures suivent la largeur de la carte, pas celle de la
            fenetre : le graphique partage sa ligne avec l'agenda, et une regle
            en `xl:` lui ferait afficher douze mois dans les 630px qui lui
            restent — deux jours par ligne, seize lignes.

            Les douze mois ne s'alignent donc qu'a partir de 896px de carte.
            Ils passent par six, puis se replient librement sous 512px, ou un
            mois se reduirait a une colonne de 21px et son libelle ne
            tiendrait plus.

            L'ecart horizontal est celui des cases, pour que les colonnes de
            mois se lisent comme une grille continue. Le vertical reste plus
            large : il separe des lignes que le nom du mois ouvre. */}
        <div className="grid grid-cols-[repeat(auto-fit,minmax(62px,1fr))] gap-x-1 gap-y-4 @lg:grid-cols-6 @4xl:grid-cols-12">
          {months.map((month) => (
            <div key={month.label} className="flex w-full min-w-0 flex-col gap-4">
              <p className="truncate text-center text-[14px] leading-5 tracking-[-0.084px] text-[#171717]">
                {month.label}
              </p>

              {/* C'est ici que la largeur est absorbee : le mois ajoute une
                  colonne de jours des qu'il a la place pour une case de plus,
                  au lieu d'etirer les quatre du dessin. Un mois large est donc
                  plat — huit jours par ligne sur un 2560, quatre sur un 1512,
                  deux sur une tablette — et la case garde ses 18px partout.
                  `aspect-square` lui donne sa hauteur. */}
              <div className="grid grid-cols-[repeat(auto-fit,minmax(18px,1fr))] gap-1">
                {month.days.map((day) => (
                  <DayCell key={day.date.getDate()} day={day} />
                ))}
              </div>
            </div>
          ))}
        </div>
      </PanelCard>
    </TooltipProvider>
  )
}
