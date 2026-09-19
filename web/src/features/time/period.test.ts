import { describe, expect, it } from 'vitest'

import { presetOf, presetPeriod, shiftPeriod } from './period'

// Un samedi de septembre.
const today = new Date(2026, 8, 19)

describe('presetPeriod', () => {
  it('commence la semaine le lundi', () => {
    expect(presetPeriod('week', today)).toEqual({
      from: '2026-09-14',
      to: '2026-09-20',
    })
    expect(presetPeriod('last-week', today)).toEqual({
      from: '2026-09-07',
      to: '2026-09-13',
    })
  })

  it('couvre les mois et trimestres entiers', () => {
    expect(presetPeriod('month', today)).toEqual({
      from: '2026-09-01',
      to: '2026-09-30',
    })
    expect(presetPeriod('last-month', today)).toEqual({
      from: '2026-08-01',
      to: '2026-08-31',
    })
    expect(presetPeriod('quarter', today)).toEqual({
      from: '2026-07-01',
      to: '2026-09-30',
    })
  })

  it('reste sous un an pour les douze derniers mois', () => {
    expect(presetPeriod('last-12', today)).toEqual({
      from: '2025-10-01',
      to: '2026-09-30',
    })
  })

  it('reconnait un raccourci depuis ses bornes', () => {
    expect(presetOf({ from: '2026-09-01', to: '2026-09-30' }, today)).toBe('month')
    expect(presetOf({ from: '2026-09-02', to: '2026-09-30' }, today)).toBeUndefined()
  })
})

describe('shiftPeriod', () => {
  it('avance de mois entiers, fevrier compris', () => {
    expect(shiftPeriod({ from: '2026-01-01', to: '2026-01-31' }, 1)).toEqual({
      from: '2026-02-01',
      to: '2026-02-28',
    })
    expect(shiftPeriod({ from: '2026-07-01', to: '2026-09-30' }, -1)).toEqual({
      from: '2026-04-01',
      to: '2026-06-30',
    })
  })

  it('avance une plage libre de sa longueur', () => {
    expect(shiftPeriod({ from: '2026-09-14', to: '2026-09-20' }, 1)).toEqual({
      from: '2026-09-21',
      to: '2026-09-27',
    })
  })
})
