import type { TaskPriority } from '@/types/api'

/**
 * Lecture d'un libelle tape a la volee : « Relancer le client demain !haute ».
 *
 * Les mots-cles sont retires du libelle et deviennent des champs : une tache
 * se cree d'une ligne, sans ouvrir trois menus. Tout est facultatif, et un mot
 * qui ressemble a un mot-cle sans en etre un reste dans le libelle — seuls les
 * jetons entiers comptent.
 */
export interface QuickTask {
  title: string
  priority?: TaskPriority
  /** AAAA-MM-JJ */
  dueOn?: string
}

const PRIORITIES: Record<string, TaskPriority> = {
  '!haute': 'high',
  '!high': 'high',
  '!urgent': 'high',
  '!moyenne': 'medium',
  '!basse': 'low',
  '!low': 'low',
}

const WEEKDAYS = ['dimanche', 'lundi', 'mardi', 'mercredi', 'jeudi', 'vendredi', 'samedi']

export function parseQuickTask(raw: string, today: Date = new Date()): QuickTask {
  const words = raw.trim().split(/\s+/)
  const kept: string[] = []
  const out: QuickTask = { title: '' }

  for (const word of words) {
    const lower = word.toLowerCase()

    if (PRIORITIES[lower] !== undefined) {
      out.priority = PRIORITIES[lower]
      continue
    }

    const date = parseDate(lower, today)
    if (date !== null) {
      out.dueOn = date
      continue
    }

    kept.push(word)
  }

  out.title = kept.join(' ')

  return out
}

/** « demain », « lundi », « 12/10 », « 12/10/2026 ». Nul si ce n'est pas une date. */
function parseDate(word: string, today: Date): string | null {
  const base = new Date(today.getFullYear(), today.getMonth(), today.getDate())

  if (word === "aujourd'hui" || word === 'aujourdhui') return iso(base)
  if (word === 'demain') return iso(shift(base, 1))
  if (word === 'après-demain' || word === 'apres-demain') return iso(shift(base, 2))

  const weekday = WEEKDAYS.indexOf(word)
  if (weekday >= 0) {
    // Le prochain jour de ce nom, jamais aujourd'hui : « lundi » tape un
    // lundi veut dire la semaine prochaine.
    let delta = (weekday - base.getDay() + 7) % 7
    if (delta === 0) delta = 7
    return iso(shift(base, delta))
  }

  const match = /^(\d{1,2})\/(\d{1,2})(?:\/(\d{2,4}))?$/.exec(word)
  if (match !== null) {
    const day = Number(match[1])
    const month = Number(match[2]) - 1
    let year = match[3] === undefined ? base.getFullYear() : Number(match[3])
    if (year < 100) year += 2000
    const date = new Date(year, month, day)
    if (date.getMonth() !== month || date.getDate() !== day) return null
    // Sans annee, une date deja passee vise l'annee prochaine.
    if (match[3] === undefined && date < base) date.setFullYear(year + 1)
    return iso(date)
  }

  return null
}

function shift(date: Date, days: number): Date {
  const out = new Date(date)
  out.setDate(out.getDate() + days)
  return out
}

function iso(date: Date): string {
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${m}-${d}`
}
