import { describe, expect, it } from 'vitest'

import { userInitials } from '@/lib/initials'

describe('userInitials', () => {
  it('prend le prénom et le nom, l’adresse à défaut', () => {
    expect(userInitials({ firstname: 'élise', lastname: 'Martin', email: 'e@x.fr' })).toBe('ÉM')
    expect(userInitials({ firstname: 'Léa', lastname: '', email: 'lea@x.fr' })).toBe('L')
    expect(userInitials({ firstname: '', lastname: '', email: 'contact@x.fr' })).toBe('CO')
  })
})
