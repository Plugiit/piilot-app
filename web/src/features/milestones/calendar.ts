/**
 * Grille du planning : les semaines entieres qui couvrent un mois, du lundi
 * au dimanche. Six semaines au plus, soit 42 jours — sous la borne de 62 que
 * l'API impose a une periode.
 */
import { parseDay, toDay } from '@/features/time/period'

/** Le mois courant au format AAAA-MM. */
export function currentMonth(now: Date = new Date()): string {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

/** Le mois voisin, en arriere ou en avant. */
export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number)

  return currentMonth(new Date(y!, m! - 1 + delta, 1))
}

/** Les jours affiches pour un mois, semaines completes, du lundi au dimanche. */
export function monthGrid(month: string): string[] {
  const first = parseDay(`${month}-01`)
  const start = new Date(first.getFullYear(), first.getMonth(), 1 - ((first.getDay() + 6) % 7))
  const last = new Date(first.getFullYear(), first.getMonth() + 1, 0)
  const end = new Date(last.getFullYear(), last.getMonth(), last.getDate() + ((7 - last.getDay()) % 7))

  const days: string[] = []
  for (let d = start; d <= end; d = new Date(d.getFullYear(), d.getMonth(), d.getDate() + 1)) {
    days.push(toDay(d))
  }

  return days
}
