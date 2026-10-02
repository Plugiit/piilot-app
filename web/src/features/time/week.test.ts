import { describe, expect, it } from 'vitest'

import { buildWeekRows, dayTotals, mondayOf, weekDays } from '@/features/time/week'
import type { TimeEntry } from '@/types/api'

function entry(id: string, day: string, minutes: number, project: string, task?: string): TimeEntry {
  return {
    id,
    spent_on: day,
    minutes,
    note: '',
    project: { id: project, name: project.toUpperCase() },
    task: task === undefined ? null : { id: task, name: task },
    service: null,
  }
}

describe('mondayOf', () => {
  it('ramène au lundi, dimanche compris', () => {
    expect(mondayOf('2026-10-02')).toBe('2026-09-28')
    expect(mondayOf('2026-09-28')).toBe('2026-09-28')
    expect(mondayOf('2026-10-04')).toBe('2026-09-28')
  })
})

describe('weekDays', () => {
  it('rend sept jours, à cheval sur deux mois', () => {
    expect(weekDays('2026-09-28')).toEqual([
      '2026-09-28',
      '2026-09-29',
      '2026-09-30',
      '2026-10-01',
      '2026-10-02',
      '2026-10-03',
      '2026-10-04',
    ])
  })
})

describe('buildWeekRows', () => {
  const days = weekDays('2026-09-28')

  it('range par projet et par tâche, une colonne par jour', () => {
    const rows = buildWeekRows(
      [
        entry('1', '2026-09-28', 60, 'b'),
        entry('2', '2026-09-29', 30, 'a', 'maquette'),
        entry('3', '2026-09-29', 45, 'a', 'maquette'),
        entry('4', '2026-09-30', 90, 'a'),
      ],
      days,
    )

    expect(rows.map((row) => row.key)).toEqual(['a:', 'a:maquette', 'b:'])
    expect(rows[1]!.cells[1]!.map((e) => e.id)).toEqual(['2', '3'])
    expect(rows[1]!.total).toBe(75)
    expect(dayTotals(rows)).toEqual([60, 75, 90, 0, 0, 0, 0])
  })

  it('garde les rangées ouvertes à la main, même vides', () => {
    const rows = buildWeekRows([], days, [{ project: { id: 'c', name: 'C' }, task: null }])

    expect(rows).toHaveLength(1)
    expect(rows[0]!.total).toBe(0)
  })
})
