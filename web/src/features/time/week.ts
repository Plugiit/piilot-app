/**
 * Feuille de temps hebdomadaire : le pointage d'une semaine, rangee par
 * projet et par tache, une colonne par jour.
 *
 * Aucune requete a part : c'est la feuille de la periode, lue du lundi au
 * dimanche, regroupee ici. Une semaine tient en quelques dizaines de lignes.
 */
import { parseDay, toDay } from '@/features/time/period'
import type { ServiceTag, TimeEntry } from '@/types/api'

export interface WeekRef {
  id: string
  name: string
}

/** Une rangee de la grille : un projet, et une tache ou aucune. */
export interface WeekRow {
  key: string
  project: WeekRef
  task: WeekRef | null
  /** Service de la derniere saisie de la rangee, repris pour les nouvelles. */
  service: ServiceTag | null
  /** Les saisies de chaque jour, du lundi au dimanche. */
  cells: TimeEntry[][]
  total: number
}

/** Cle d'une rangee : le projet et la tache, celle-ci absente comprise. */
export function rowKey(projectId: string, taskId: string | null | undefined): string {
  return `${projectId}:${taskId ?? ''}`
}

/** Le lundi de la semaine d'un jour. */
export function mondayOf(day: string): string {
  const date = parseDay(day)
  date.setDate(date.getDate() - ((date.getDay() + 6) % 7))

  return toDay(date)
}

/** Les sept jours d'une semaine, a partir de son lundi. */
export function weekDays(monday: string): string[] {
  const start = parseDay(monday)

  return Array.from({ length: 7 }, (_, i) =>
    toDay(new Date(start.getFullYear(), start.getMonth(), start.getDate() + i)),
  )
}

/**
 * Range les saisies d'une semaine en rangees.
 *
 * `extra` ajoute des rangees encore vides — celles qu'on vient d'ouvrir pour
 * pointer sur un projet qui n'a rien cette semaine.
 */
export function buildWeekRows(
  items: TimeEntry[],
  days: string[],
  extra: { project: WeekRef; task: WeekRef | null }[] = [],
): WeekRow[] {
  const rows = new Map<string, WeekRow>()

  const rowFor = (project: WeekRef, task: WeekRef | null) => {
    const key = rowKey(project.id, task?.id)
    let row = rows.get(key)
    if (row === undefined) {
      row = { key, project, task, service: null, cells: days.map(() => []), total: 0 }
      rows.set(key, row)
    }

    return row
  }

  for (const entry of items) {
    const index = days.indexOf(entry.spent_on)
    if (index === -1) continue

    const row = rowFor(entry.project, entry.task)
    row.cells[index]!.push(entry)
    row.total += entry.minutes
    row.service = entry.service ?? row.service
  }

  for (const { project, task } of extra) rowFor(project, task)

  // Par projet puis par tache, la rangee « sans tache » en tete de son projet :
  // l'ordre ne bouge pas quand une cellule change, contrairement a un tri sur
  // le temps.
  return [...rows.values()].sort(
    (a, b) =>
      a.project.name.localeCompare(b.project.name, 'fr') ||
      (a.task === null ? -1 : b.task === null ? 1 : a.task.name.localeCompare(b.task.name, 'fr')),
  )
}

/** Total de chaque jour, sur toutes les rangees. */
export function dayTotals(rows: WeekRow[]): number[] {
  return Array.from({ length: 7 }, (_, i) =>
    rows.reduce((sum, row) => sum + (row.cells[i] ?? []).reduce((s, e) => s + e.minutes, 0), 0),
  )
}
