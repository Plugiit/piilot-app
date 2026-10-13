import { describe, expect, it } from 'vitest'

import { monthLabel, shiftMonth } from './format'

describe('maintenance', () => {
  it('nomme et décale les mois', () => {
    expect(monthLabel('2026-10')).toBe('Octobre 2026')
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
    expect(shiftMonth('2026-12', 1)).toBe('2027-01')
  })
})
