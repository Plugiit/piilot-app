import { describe, expect, it } from 'vitest'

import { displayName, lastSeen, roleOf } from '@/features/accounts/format'

const NOW = new Date('2026-10-02T12:00:00Z').getTime()

function ago(seconds: number) {
  return { last_login_at: new Date(NOW - seconds * 1000).toISOString() }
}

describe('lastSeen', () => {
  it('dit quand un compte ne s’est jamais connecté', () => {
    expect(lastSeen({ last_login_at: null }, NOW)).toBe('Aucune connexion')
  })

  it('donne un ordre de grandeur plutôt qu’une date', () => {
    expect(lastSeen(ago(20), NOW)).toBe('À l’instant')
    expect(lastSeen(ago(5 * 60), NOW)).toBe('il y a 5 minutes')
    expect(lastSeen(ago(3 * 86_400), NOW)).toBe('il y a 3 jours')
    expect(lastSeen(ago(86_400), NOW)).toBe('hier')
  })
})

describe('displayName', () => {
  it('rend le nom, l’adresse à défaut', () => {
    expect(displayName({ firstname: 'Léa', lastname: 'Bernard', email: 'lea@x.fr' })).toBe('Léa Bernard')
    expect(displayName({ firstname: '', lastname: '', email: 'lea@x.fr' })).toBe('lea@x.fr')
  })
})

describe('roleOf', () => {
  it('connaît les trois rôles et ne plante pas sur un rôle inconnu', () => {
    expect(roleOf('team').label).toBe('Équipe')
    expect(roleOf('auditeur').label).toBe('auditeur')
  })
})
