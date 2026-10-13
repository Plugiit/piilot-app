import { describe, expect, it } from 'vitest'

import { browserLabel, countryLabel, derived, formatSeconds, formatVariation, referrerLabel, variation } from './format'

describe('statistiques', () => {
  it('compare deux périodes, sans base pas de variation', () => {
    expect(variation(120, 100)).toBe(20)
    expect(variation(5, 0)).toBeNull()
    expect(formatVariation(20)).toBe('+20 %')
    expect(formatVariation(-12.5)).toBe('-12,5 %')
  })

  it('dérive le taux de rebond et la durée moyenne', () => {
    expect(derived({ visitors: 40, visits: 50, pageviews: 120, bounces: 20, total_time: 3000 })).toEqual({ bounceRate: 40, avgDuration: 60 })
    expect(derived({ visitors: 0, visits: 0, pageviews: 0, bounces: 0, total_time: 0 })).toEqual({ bounceRate: null, avgDuration: null })
  })

  it('écrit les durées et les libellés', () => {
    expect(formatSeconds(48)).toBe('48 s')
    expect(formatSeconds(125)).toBe('2 min 05 s')
    expect(countryLabel('FR').flag).toBe('🇫🇷')
    expect(countryLabel('').name).toBe('Inconnu')
    expect(browserLabel('edge-chromium')).toBe('Edge')
    expect(referrerLabel('')).toBe('Accès direct')
    expect(referrerLabel('www.google.com')).toBe('google.com')
  })
})
