import { describe, expect, it } from 'vitest'

import { budgetStateOf, formatHours } from '@/features/projects/format'

// Memes cas que TestBudgetStateOfClasseSelonLaPartConsommee cote Go : la page
// Budget recalcule l'etat a la frappe, et doit tomber sur le meme que l'API.
describe('budgetStateOf', () => {
  it('ne donne pas de budget a un projet interne ou sans heures vendues', () => {
    expect(budgetStateOf(100, 150, true)).toBe('none')
    expect(budgetStateOf(0, 12, false)).toBe('none')
  })

  it('passe a surveiller a 80 % et hors budget au-dela de la totalite', () => {
    expect(budgetStateOf(100, 79.75, false)).toBe('ok')
    expect(budgetStateOf(100, 80, false)).toBe('warning')
    expect(budgetStateOf(100, 100, false)).toBe('warning')
    expect(budgetStateOf(100, 100.5, false)).toBe('over')
  })
})

describe('formatHours', () => {
  it('ecrit une decimale au plus, a la francaise', () => {
    expect(formatHours(12)).toBe('12 h')
    expect(formatHours(12.5)).toBe('12,5 h')
    expect(formatHours(12.04)).toBe('12 h')
  })
})
