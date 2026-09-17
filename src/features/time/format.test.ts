import { describe, expect, it } from 'vitest'

import { formatDuration, parseDuration } from '@/features/time/format'

describe('parseDuration', () => {
  it('lit les minutes seules', () => {
    expect(parseDuration('90')).toBe(90)
    expect(parseDuration('5')).toBe(5)
  })

  it('lit la forme heures-minutes', () => {
    expect(parseDuration('1h30')).toBe(90)
    expect(parseDuration('1 h 30')).toBe(90)
    expect(parseDuration('2h')).toBe(120)
    expect(parseDuration('0h45')).toBe(45)
  })

  it('lit les heures décimales, virgule comprise', () => {
    expect(parseDuration('1.5')).toBe(90)
    expect(parseDuration('1,5')).toBe(90)
    expect(parseDuration('0,25')).toBe(15)
  })

  it('refuse ce qui n’est pas une durée', () => {
    expect(parseDuration('')).toBeNull()
    expect(parseDuration('   ')).toBeNull()
    expect(parseDuration('beaucoup')).toBeNull()
    expect(parseDuration('h30')).toBeNull()
    expect(parseDuration('-30')).toBeNull()
  })

  it('refuse plus de 59 minutes après l’heure, qui est une faute de frappe', () => {
    expect(parseDuration('1h75')).toBeNull()
  })
})

describe('formatDuration', () => {
  it('écrit comme on parle d’un temps passé', () => {
    expect(formatDuration(90)).toBe('1h30')
    expect(formatDuration(120)).toBe('2h')
    expect(formatDuration(45)).toBe('45 min')
    expect(formatDuration(65)).toBe('1h05')
    expect(formatDuration(0)).toBe('0h')
  })
})
