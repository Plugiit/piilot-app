/**
 * Periodes des rapports de temps.
 *
 * Toutes les dates sont des jours locaux au format AAAA-MM-JJ. `toISOString`
 * est proscrit ici : il passe en UTC, et a minuit heure de Paris il rendrait
 * la veille.
 */

export function toDay(date: Date): string {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')

  return `${y}-${m}-${d}`
}

export function parseDay(value: string): Date {
  const [y, m, d] = value.split('-').map(Number)

  return new Date(y!, m! - 1, d!)
}

function addDays(date: Date, days: number) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days)
}

/** Lundi de la semaine. */
function startOfWeek(date: Date) {
  return addDays(date, -((date.getDay() + 6) % 7))
}

function endOfMonth(year: number, month: number) {
  return new Date(year, month + 1, 0)
}

export const PRESETS = [
  'week',
  'last-week',
  'month',
  'last-month',
  'quarter',
  'year',
  'last-12',
] as const
export type Preset = (typeof PRESETS)[number]

export const PRESET_LABELS: Record<Preset, string> = {
  week: 'Cette semaine',
  'last-week': 'Semaine dernière',
  month: 'Ce mois-ci',
  'last-month': 'Mois dernier',
  quarter: 'Ce trimestre',
  year: 'Cette année',
  'last-12': '12 derniers mois',
}

export interface Period {
  from: string
  to: string
}

/** Bornes d'un raccourci de periode, relatives a `today`. */
export function presetPeriod(preset: Preset, today = new Date()): Period {
  const y = today.getFullYear()
  const m = today.getMonth()

  switch (preset) {
    case 'week': {
      const start = startOfWeek(today)
      return { from: toDay(start), to: toDay(addDays(start, 6)) }
    }
    case 'last-week': {
      const start = addDays(startOfWeek(today), -7)
      return { from: toDay(start), to: toDay(addDays(start, 6)) }
    }
    case 'month':
      return { from: toDay(new Date(y, m, 1)), to: toDay(endOfMonth(y, m)) }
    case 'last-month':
      return {
        from: toDay(new Date(y, m - 1, 1)),
        to: toDay(endOfMonth(y, m - 1)),
      }
    case 'quarter': {
      const first = m - (m % 3)
      return {
        from: toDay(new Date(y, first, 1)),
        to: toDay(endOfMonth(y, first + 2)),
      }
    }
    case 'year':
      return { from: toDay(new Date(y, 0, 1)), to: toDay(new Date(y, 11, 31)) }
    case 'last-12':
      return {
        from: toDay(new Date(y, m - 11, 1)),
        to: toDay(endOfMonth(y, m)),
      }
  }
}

/** Raccourci dont la periode est exactement celle-ci, s'il y en a un. */
export function presetOf(period: Period, today = new Date()): Preset | undefined {
  return PRESETS.find((preset) => {
    const candidate = presetPeriod(preset, today)
    return candidate.from === period.from && candidate.to === period.to
  })
}

/**
 * Periode precedente ou suivante, de meme forme.
 *
 * Des mois entiers avancent d'autant de mois — fevrier apres janvier, pas
 * « trente et un jours plus tard ». Toute autre periode avance de sa longueur :
 * une semaine de sept jours, une plage libre de la sienne.
 */
export function shiftPeriod(period: Period, direction: 1 | -1): Period {
  const from = parseDay(period.from)
  const to = parseDay(period.to)

  const wholeMonths = from.getDate() === 1 && addDays(to, 1).getDate() === 1 && to >= from

  if (wholeMonths) {
    const months =
      (to.getFullYear() - from.getFullYear()) * 12 + (to.getMonth() - from.getMonth()) + 1
    const start = new Date(from.getFullYear(), from.getMonth() + direction * months, 1)
    const end = endOfMonth(start.getFullYear(), start.getMonth() + months - 1)

    return { from: toDay(start), to: toDay(end) }
  }

  const length = Math.round((to.getTime() - from.getTime()) / 86_400_000) + 1

  return {
    from: toDay(addDays(from, direction * length)),
    to: toDay(addDays(to, direction * length)),
  }
}

const RANGE_FORMAT = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'short',
  year: 'numeric',
})

/** « 1 sept. 2026 – 30 sept. 2026 », ou le libelle du raccourci. */
export function periodLabel(period: Period, today = new Date()): string {
  const preset = presetOf(period, today)
  if (preset !== undefined) return PRESET_LABELS[preset]

  return `${RANGE_FORMAT.format(parseDay(period.from))} – ${RANGE_FORMAT.format(parseDay(period.to))}`
}

/** Libelle court d'une tranche du graphique. */
export function bucketLabel(start: string, bucket: 'day' | 'week' | 'month'): string {
  const date = parseDay(start)

  if (bucket === 'month') {
    return new Intl.DateTimeFormat('fr-FR', { month: 'short' }).format(date)
  }

  return new Intl.DateTimeFormat('fr-FR', {
    day: 'numeric',
    month: 'short',
  }).format(date)
}

/** Libelle long d'une tranche, pour l'infobulle. */
export function bucketTitle(start: string, bucket: 'day' | 'week' | 'month'): string {
  const date = parseDay(start)

  if (bucket === 'month') {
    return new Intl.DateTimeFormat('fr-FR', {
      month: 'long',
      year: 'numeric',
    }).format(date)
  }
  if (bucket === 'week') {
    return `Semaine du ${new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long' }).format(date)}`
  }

  return new Intl.DateTimeFormat('fr-FR', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  }).format(date)
}
