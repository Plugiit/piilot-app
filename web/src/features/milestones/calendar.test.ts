import { describe, expect, it } from 'vitest'

import { monthGrid, shiftMonth } from '@/features/milestones/calendar'

describe('monthGrid', () => {
  it('couvre le mois en semaines entières, du lundi au dimanche', () => {
    const days = monthGrid('2026-10')

    // Le 1er octobre 2026 est un jeudi, le 31 un samedi.
    expect(days[0]).toBe('2026-09-28')
    expect(days.at(-1)).toBe('2026-11-01')
    expect(days.length % 7).toBe(0)
  })

  it('ne dépasse jamais six semaines', () => {
    for (let m = 1; m <= 12; m++) {
      expect(monthGrid(`2027-${String(m).padStart(2, '0')}`).length).toBeLessThanOrEqual(42)
    }
  })
})

describe('shiftMonth', () => {
  it('passe d’une année à l’autre', () => {
    expect(shiftMonth('2026-12', 1)).toBe('2027-01')
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
  })
})
