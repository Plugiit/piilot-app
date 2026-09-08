import { Activity03Icon, InformationCircleIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMemo } from 'react'

import { DashboardCard } from '@/components/dashboard-card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

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

/**
 * Palier d'une journee, d'apres son nombre d'activites.
 *
 * C'est la seule des deux fonctions de seuils qui survivra au branchement de
 * l'API : `spanOf` ne sert qu'a fabriquer des nombres en attendant. Elles se
 * lisent ensemble — leurs bornes sont les memes, dans l'autre sens.
 */
function levelOf(total: number) {
  if (total === 0) return 0
  if (total <= 2) return 1
  if (total <= 5) return 2
  if (total <= 9) return 3
  return 4
}

/** Fourchette d'activites d'un palier. */
function spanOf(level: number) {
  if (level <= 1) return { floor: 1, ceiling: 2 }
  if (level === 2) return { floor: 3, ceiling: 5 }
  if (level === 3) return { floor: 6, ceiling: 9 }
  return { floor: 10, ceiling: 14 }
}

/**
 * Habillage clair des infobulles du graphique.
 *
 * Celle du projet est sombre par defaut ; ici elle se pose sur la carte
 * blanche et reprend ses bordures, sinon deux traitements sans rapport se
 * repondraient au meme endroit. La fleche est un carre pivote que shadcn peint
 * en `bg-foreground` : sans la reprendre, elle resterait noire sous une bulle
 * blanche. Elle se vise en descendant et non en enfant direct — Radix
 * l'enveloppe d'un span, son `ResizeObserver` mesurant mal les SVG.
 */
const LIGHT_TOOLTIP =
  'border border-[#ebebeb] bg-white text-[#171717] shadow-[0_4px_12px_rgb(16_24_40/0.08)] [&_svg]:bg-white [&_svg]:fill-white'

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

/**
 * Generateur pseudo-aleatoire deterministe (mulberry32).
 *
 * `Math.random` redistribuerait les intensites a chaque rendu et la grille
 * scintillerait au moindre re-rendu du tableau de bord. Une graine fixe donne
 * un motif stable, identique d'une session a l'autre.
 */
function seeded(seed: number) {
  return () => {
    seed = (seed + 0x6d2b79f5) | 0
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/** Les douze mois de `year`, chacun portant une journee par jour reel. */
function buildYear(year: number) {
  const random = seeded(year)

  return MONTH_LABELS.map((label, month) => ({
    label,
    // Le jour 0 du mois suivant est le dernier du mois courant : fevrier
    // compte donc 29 cases les annees bissextiles, sans table a maintenir.
    days: Array.from({ length: new Date(year, month + 1, 0).getDate() }, (_, index) => {
      const draw = random()

      // Un peu moins d'un tiers de journees vides, le reste reparti sur les
      // quatre paliers : la grille respire sans paraitre eteinte.
      const level = draw < 0.3 ? 0 : 1 + Math.min(3, Math.floor(((draw - 0.3) / 0.7) * 4))
      const { floor, ceiling } = spanOf(level)
      const total = level === 0 ? 0 : floor + Math.floor(random() * (ceiling - floor + 1))

      // Les trois natures se partagent le total plutot que d'etre tirees
      // separement : leur somme doit tomber juste, sinon le detail
      // contredirait le nombre annonce au-dessus de lui.
      const tasks = Math.round(total * (0.4 + random() * 0.3))
      const tickets = Math.round((total - tasks) * random())

      return {
        date: new Date(year, month, index + 1),
        tasks,
        tickets,
        deliverables: total - tasks - tickets,
        total,
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
 * Activite de l'annee, une case par jour.
 *
 * Vit hors de `components/ui/`, reserve aux composants shadcn — voir la note
 * de `stat-card.tsx`.
 *
 * Les nombres sont figes : aucun agregat d'activite n'est encore calcule cote
 * API. Le calendrier, lui, est juste — le nombre de cases suit toujours le
 * nombre de jours du mois.
 */
export function ActivityHeatmap() {
  const months = useMemo(() => buildYear(new Date().getFullYear()), [])

  return (
    // Un seul fournisseur pour toute la grille : les 365 infobulles partagent
    // son delai, et une seule peut etre ouverte a la fois.
    <TooltipProvider>
      <DashboardCard
        icon={Activity03Icon}
        title="ACTIVITÉ"
        action={
          <Tooltip>
            <TooltipTrigger className="flex shrink-0 items-center text-[#606060]">
              <HugeiconsIcon icon={InformationCircleIcon} size={16} strokeWidth={1.6} />
              <span className="sr-only">À propos de ce graphique</span>
            </TooltipTrigger>
            <TooltipContent className={LIGHT_TOOLTIP}>
              Une case par jour : plus elle est soutenue, plus l'activité a été forte.
            </TooltipContent>
          </Tooltip>
        }
      >
        {/* Les ruptures suivent la largeur de la carte, pas celle de la
              fenetre : le graphique partage desormais sa ligne avec l'agenda,
              et une regle en `xl:` lui ferait afficher douze mois dans les
              630px qui lui restent — deux jours par ligne, seize lignes.

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
      </DashboardCard>
    </TooltipProvider>
  )
}
