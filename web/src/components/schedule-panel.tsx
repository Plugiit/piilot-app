import { ArrowLeft01Icon, ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { AnimatePresence, motion, useReducedMotion } from 'framer-motion'
import { useMemo, useState } from 'react'

import { cn } from '@/lib/utils'

const WEEKDAY_FORMAT = new Intl.DateTimeFormat('fr-FR', { weekday: 'short' })
const MONTH_FORMAT = new Intl.DateTimeFormat('fr-FR', { month: 'long', year: 'numeric' })
const DAY_FORMAT = new Intl.DateTimeFormat('fr-FR', {
  weekday: 'long',
  day: 'numeric',
  month: 'long',
})

/** Filtres de la barre. `category` a null laisse tout passer. */
const FILTERS: { label: string; category: string | null }[] = [
  { label: 'Tout', category: null },
  { label: 'Jalons', category: 'JALON' },
  { label: 'Livrables', category: 'LIVRABLE' },
  { label: 'Réunions', category: 'RÉUNION' },
]

/**
 * Evenements de la semaine, indexes par jour — lundi en 0.
 *
 * Figes : le module n'a pas de modele de donnees. Ils different d'un jour a
 * l'autre parce qu'une liste identique partout rendrait la transition
 * illisible : on verrait glisser deux fois la meme chose. Deux jours sont
 * vides, l'agenda doit aussi savoir ne rien avoir a dire.
 */
/**
 * Evenement de l'agenda.
 *
 * Type local, et non celui d'une tache : ces lignes sont un decor de maquette
 * en attendant le module planning. Le tiroir de tache, lui, ne s'ouvre plus
 * que sur des taches reelles, depuis le tableau d'un projet.
 */
interface ScheduleEvent {
  id: string
  title: string
  category: string
  color: string
  when: string
}

const EVENTS_BY_DAY: ScheduleEvent[][] = [
  [
    {
      category: 'RÉUNION',
      color: '#00ac47',
      id: 'point-hebdomadaire',
      title: 'Point hebdomadaire équipe',
      when: '09:30 – 10:15',
    },
    {
      category: 'LIVRABLE',
      color: '#6060d8',
      id: 'maquettes-refonte-vitrine',
      title: 'Maquettes — Refonte vitrine',
      when: '14:00 – 16:00',
    },
  ],
  [
    {
      category: 'JALON',
      color: '#cf029f',
      id: 'recette-client-portail',
      title: 'Recette client — Portail',
      when: '11:00 – 13:00',
    },
  ],
  [
    {
      category: 'LIVRABLE',
      color: '#6060d8',
      id: 'livraison-v1-mobile',
      title: 'Livraison V1 — Application mobile',
      when: '09:00 – 11:00',
    },
    {
      category: 'RÉUNION',
      color: '#00ac47',
      id: 'cadrage-nouveau-projet',
      title: 'Cadrage — Nouveau projet',
      when: '15:00 – 16:30',
    },
    {
      category: 'JALON',
      color: '#cf029f',
      id: 'mise-en-production-intranet',
      title: 'Mise en production — Intranet',
      when: '17:00 – 18:00',
    },
  ],
  [
    {
      category: 'RÉUNION',
      color: '#00ac47',
      id: 'revue-tickets-ouverts',
      title: 'Revue des tickets ouverts',
      when: '10:00 – 11:00',
    },
  ],
  [
    {
      category: 'LIVRABLE',
      color: '#6060d8',
      id: 'direction-artistique-identite',
      title: 'Direction artistique — Identité',
      when: '09:00 – 12:00',
    },
    { category: 'JALON', color: '#cf029f', id: 'fin-de-sprint',
      title: 'Fin de sprint', when: '16:00 – 17:00' },
  ],
  [],
  [],
]

/** La semaine de lundi a dimanche qui contient `date`. */
function weekOf(date: Date) {
  const monday = new Date(date)

  // getDay() commence au dimanche : le decalage ramene au lundi precedent.
  monday.setDate(date.getDate() - ((date.getDay() + 6) % 7))

  return Array.from({ length: 7 }, (_, index) => {
    const day = new Date(monday)
    day.setDate(monday.getDate() + index)
    return day
  })
}

function isSameDay(a: Date, b: Date) {
  return (
    a.getDate() === b.getDate() &&
    a.getMonth() === b.getMonth() &&
    a.getFullYear() === b.getFullYear()
  )
}

/** Bouton de navigation de mois. */
function MonthArrow({
  direction,
  onClick,
}: {
  direction: 'previous' | 'next'
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={direction === 'previous' ? 'Mois précédent' : 'Mois suivant'}
      className="flex size-[26px] shrink-0 items-center justify-center rounded-[6px] border border-[#e6e6e6] bg-white text-[#111] transition-colors hover:bg-[#f0f0f0]"
    >
      <HugeiconsIcon
        icon={direction === 'previous' ? ArrowLeft01Icon : ArrowRight01Icon}
        size={16}
        strokeWidth={2}
      />
    </button>
  )
}

/**
 * Panneau lateral du tableau de bord.
 *
 * Vit hors de `components/ui/`, reserve aux composants shadcn — voir la note
 * de `stat-card.tsx`.
 */
export function SchedulePanel({ className }: { className?: string }) {
  const [selected, setSelected] = useState(() => new Date())

  // La semaine se deduit du jour choisi : changer de mois deplace le jour, et
  // la bande suit — une semaine gardee a part pourrait montrer un autre mois
  // que celui affiche au-dessus d'elle.
  const week = useMemo(() => weekOf(selected), [selected])

  // Le sens du glissement suit celui de la lecture : avancer dans la semaine
  // fait entrer la liste par la droite, reculer par la gauche.
  const [direction, setDirection] = useState(1)
  const reduced = useReducedMotion()

  const month = MONTH_FORMAT.format(selected)
  const [filter, setFilter] = useState<string | null>(null)

  const events = (EVENTS_BY_DAY[(selected.getDay() + 6) % 7] ?? []).filter(
    (event) => filter === null || event.category === filter,
  )
  const shift = reduced ? 0 : 20
  const transition = reduced
    ? ({ duration: 0 } as const)
    : ({ type: 'spring', visualDuration: 0.25, bounce: 0 } as const)

  // La bande ne se rejoue que si la semaine change : choisir un autre jour de
  // la meme semaine ne doit pas la faire glisser sous le doigt.
  const weekKey = week[0]?.toDateString()

  function selectDay(date: Date) {
    setDirection(date.getTime() < selected.getTime() ? -1 : 1)
    setSelected(date)
  }

  // On atterrit sur le premier du mois vise plutot que sur le meme quantieme :
  // le 31 n'existe pas partout, et un report silencieux au 1er ou au 3 serait
  // plus surprenant que d'annoncer le debut du mois.
  // Changer de filtre ne va ni en avant ni en arriere : la liste se contente
  // de fondre, la faire glisser suggererait un deplacement dans le temps.
  function selectFilter(category: string | null) {
    setDirection(0)
    setFilter(category)
  }

  function goToMonth(offset: number) {
    setDirection(offset)
    setSelected(new Date(selected.getFullYear(), selected.getMonth() + offset, 1))
  }

  return (
    <aside className={cn('flex flex-col gap-3 bg-white p-4', className)}>
      <h2 className="text-[16px] font-medium text-[#111]">Agenda</h2>

      <div className="border-surface-sunken flex flex-col gap-3 border-b pb-4">
        <div className="bg-surface flex items-center justify-between gap-2 rounded-[12px] px-2 py-2.5">
          <MonthArrow direction="previous" onClick={() => goToMonth(-1)} />
          <p className="truncate text-[14px] font-semibold text-[#1f2937] first-letter:uppercase">
            {month}
          </p>
          <MonthArrow direction="next" onClick={() => goToMonth(1)} />
        </div>

        {/* Sept colonnes egales plutot que les largeurs fixes du dessin : a
            340px de panneau, sept blocs de 36px et leurs ecarts de 16px
            debordent de pres de cinquante pixels.

            La bande glisse quand on change de semaine — donc au changement de
            mois. `overflow-hidden` la borne au panneau : sans lui, les jours
            sortants passeraient par-dessus la bordure. */}
        <div className="overflow-hidden">
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key={weekKey}
              initial={{ opacity: 0, x: shift * direction }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: -shift * direction, transition: { duration: 0.12 } }}
              transition={transition}
              className="grid grid-cols-7 gap-1"
            >
              {week.map((date) => {
                const active = isSameDay(date, selected)

                return (
                  <div key={date.toISOString()} className="flex flex-col items-center gap-2">
                    <p className="text-[12px] leading-[1.4] font-semibold text-[#444]">
                      {/* « lun. » perd son point : dans une colonne de cette
                          largeur il ne se lit pas, il encombre. */}
                      {WEEKDAY_FORMAT.format(date).replace('.', '')}
                    </p>

                    {/* Le jour choisi porte de nouveau la pastille blanche du
                        dessin : sur un panneau blanc elle ne tient que par son
                        ombre portee, que la maquette pose desormais. */}
                    <button
                      type="button"
                      aria-pressed={active}
                      aria-label={DAY_FORMAT.format(date)}
                      onClick={() => selectDay(date)}
                      className={cn(
                        'flex size-8 items-center justify-center rounded-full text-center text-[14px] leading-[1.5] font-semibold transition-colors',
                        active
                          ? 'bg-white text-[#ff782b] drop-shadow-[0px_4px_2px_rgba(0,0,0,0.06)]'
                          : 'text-[#111] hover:bg-[#f0f0f0]',
                      )}
                    >
                      {date.getDate()}
                    </button>
                  </div>
                )
              })}
            </motion.div>
          </AnimatePresence>
        </div>

        {/* Les marges negatives rendent au defilement les 16px que le panneau
            prend en padding : sans elles, la premiere et la derniere pilule
            resteraient coupees en butee. */}
        <div className="-mx-4 flex gap-3 overflow-x-auto px-4 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          {FILTERS.map((entry) => {
            const active = entry.category === filter

            return (
              <button
                key={entry.label}
                type="button"
                aria-pressed={active}
                onClick={() => selectFilter(entry.category)}
                className={cn(
                  'shrink-0 rounded-[12px] border px-4 py-2 text-[12px] font-medium whitespace-nowrap transition-colors',
                  // Le filtre actif prend l'orange plein plutot qu'une pastille
                  // blanche : sur un panneau blanc, le blanc sur blanc ne
                  // distinguait rien.
                  active
                    ? 'border-[#ff782b] bg-[#ff782b] text-white'
                    : 'border-surface-sunken bg-white text-[#111] hover:bg-[#f8f8f8]',
                )}
              >
                {entry.label}
              </button>
            )
          })}
        </div>
      </div>

      {/* `mode="wait"` sort la liste precedente avant d'installer la suivante :
          les deux se croiseraient sinon sur quelques pixels, ce qui brouille la
          lecture sur un panneau etroit. La sortie est un tween court plutot que
          le ressort d'entree — enchainer deux ressorts ferait trainer un
          changement de jour a une demi-seconde. */}
      <AnimatePresence mode="wait" initial={false}>
        <motion.div
          key={`${selected.toDateString()}-${filter ?? 'tout'}`}
          initial={{ opacity: 0, x: shift * direction }}
          animate={{ opacity: 1, x: 0 }}
          exit={{ opacity: 0, x: -shift * direction, transition: { duration: 0.12 } }}
          transition={transition}
          className="flex flex-col gap-5"
        >
          {events.length === 0 ? (
            <p className="text-[14px] text-[#111]/50">
              {filter === null
                ? 'Aucun événement ce jour-là.'
                : `Aucun événement de ce type ce jour-là.`}
            </p>
          ) : (
            events.map((event) => (
              <div key={event.title} className="flex w-full gap-1 rounded-[8px] text-left">
                {/* Le filet du dessin : un trait de 2px qui part a 7px du haut,
                    coiffe d'un demi-disque de 12px bombe vers la gauche, les
                    deux alignes sur le meme bord droit.

                    Dessine en CSS plutot qu'importe en SVG : le fichier exporte
                    fige sa couleur — la maquette en compte d'ailleurs deux
                    copies pour deux teintes — alors qu'ici elle suit la
                    categorie de l'evenement.

                    Le demi-disque est un rectangle de 6 sur 12 dont les deux
                    coins gauches sont arrondis de 6 : a cette proportion, le
                    rayon consomme toute la hauteur et l'arc est un demi-cercle
                    exact. */}
                <div aria-hidden className="relative w-[10px] shrink-0 self-stretch">
                  <span
                    className="absolute top-0 left-0 h-3 w-[6px] rounded-l-full"
                    style={{ backgroundColor: event.color }}
                  />
                  <span
                    className="absolute top-[7px] bottom-0 left-[4px] w-[2px]"
                    style={{ backgroundColor: event.color }}
                  />
                </div>

                <div className="flex min-w-0 flex-col gap-1.5">
                  <p
                    className="text-[12px] font-medium tracking-[0.48px]"
                    style={{ color: event.color }}
                  >
                    {event.category}
                  </p>

                  <div className="flex flex-col gap-0.5">
                    <p className="text-[14px] font-semibold text-[#111]">{event.title}</p>
                    <p className="text-[14px] leading-[1.5] text-[#111]/70">{event.when}</p>
                  </div>
                </div>
              </div>
            ))
          )}
        </motion.div>
      </AnimatePresence>
    </aside>
  )
}
