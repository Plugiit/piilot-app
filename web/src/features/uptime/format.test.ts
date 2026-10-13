import { describe, expect, it } from 'vitest'

import { formatDuration, formatUptime, uptimeColor } from './format'

describe('disponibilité', () => {
  it('écrit un pourcentage à la française, ou un tiret sans contrôle', () => {
    expect(formatUptime(99.95)).toBe('99,95 %')
    expect(formatUptime(100)).toBe('100 %')
    expect(formatUptime(null)).toBe('—')
  })

  it('colore une case selon sa disponibilité', () => {
    expect(uptimeColor(null)).toBe('#ebebeb')
    expect(uptimeColor(100)).toBe('#0db471')
    expect(uptimeColor(99.5)).toBe('#7fd3a8')
    expect(uptimeColor(97)).toBe('#eab308')
    expect(uptimeColor(80)).toBe('#e5484d')
  })

  it('dit une durée en mots courts', () => {
    expect(formatDuration(42)).toBe('42 s')
    expect(formatDuration(240)).toBe('4 min')
    expect(formatDuration(2 * 3600 + 600)).toBe('2 h 10')
    expect(formatDuration(3 * 3600)).toBe('3 h')
    expect(formatDuration(3 * 86400 + 4 * 3600)).toBe('3 j 4 h')
  })
})
