import { describe, expect, it } from 'vitest'

import { fileSize, since } from '@/features/portal/format'

const NOW = new Date('2026-10-03T12:00:00')

describe('since', () => {
  it('parle en jours la première semaine, en dates ensuite', () => {
    expect(since('2026-10-03T08:00:00', NOW)).toBe('aujourd’hui')
    expect(since('2026-10-02T08:00:00', NOW)).toBe('hier')
    expect(since('2026-09-30T08:00:00', NOW)).toBe('il y a 3 jours')
    expect(since('2026-09-12T08:00:00', NOW)).toBe('le 12 septembre')
  })
})

describe('fileSize', () => {
  it('choisit l’unité', () => {
    expect(fileSize(900)).toBe('900 o')
    expect(fileSize(20_480)).toBe('20 Ko')
    expect(fileSize(3_670_016)).toBe('3,5 Mo')
  })
})
