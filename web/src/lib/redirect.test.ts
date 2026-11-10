import { describe, expect, it } from 'vitest'

import { safeRedirect } from '@/lib/redirect'

const ORIGIN = 'https://auth.agence.fr'
const SPACES = {
  auth: 'https://auth.agence.fr',
  team: 'https://team.agence.fr',
  admin: 'https://admin.agence.fr',
  client: 'https://client.agence.fr',
}

describe('safeRedirect', () => {
  it('garde un chemin de l’application', () => {
    expect(safeRedirect('/autoriser?demande=abc', ORIGIN, SPACES)).toBe('/autoriser?demande=abc')
    expect(safeRedirect('/pm', ORIGIN, null)).toBe('/pm')
  })

  it('garde une adresse sur l’un des domaines', () => {
    expect(safeRedirect('https://admin.agence.fr/parametres', ORIGIN, SPACES)).toBe('https://admin.agence.fr/parametres')
    expect(safeRedirect('https://auth.agence.fr/autoriser?demande=x', ORIGIN, null)).toBe(
      'https://auth.agence.fr/autoriser?demande=x',
    )
  })

  it('refuse ce qui sortirait de l’application', () => {
    for (const raw of [
      'https://evil.example/login',
      '//evil.example/x',
      '/\\evil.example/x',
      'javascript:alert(1)',
      'https://auth.agence.fr.evil.example/',
      'http://auth.agence.fr/pm',
      'pas une adresse',
    ]) {
      expect(safeRedirect(raw, ORIGIN, SPACES)).toBeUndefined()
    }
    expect(safeRedirect(undefined, ORIGIN, SPACES)).toBeUndefined()
    expect(safeRedirect('', ORIGIN, SPACES)).toBeUndefined()
  })
})
