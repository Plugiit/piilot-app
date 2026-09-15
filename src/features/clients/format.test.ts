import { describe, expect, it } from 'vitest'

import { clientAge } from '@/features/clients/format'

/** Date de reference fixe : un test d'anciennete ne doit pas dependre du jour. */
const NOW = new Date('2026-06-15T12:00:00Z')

/** Date situee `days` jours avant la reference. */
function daysAgo(days: number): string {
  return new Date(NOW.getTime() - days * 24 * 60 * 60 * 1000).toISOString()
}

describe('clientAge', () => {
  it('abrege la duree selon son ordre de grandeur', () => {
    expect(clientAge(daysAgo(0), NOW).label).toBe('0 j')
    expect(clientAge(daysAgo(3), NOW).label).toBe('3 j')
    expect(clientAge(daysAgo(14), NOW).label).toBe('2 sem.')
    expect(clientAge(daysAgo(60), NOW).label).toBe('2 mois')
    expect(clientAge(daysAgo(400), NOW).label).toBe('1 an')
    expect(clientAge(daysAgo(800), NOW).label).toBe('2 ans')
  })

  it('bascule en alerte aux seuils de 30 et 90 jours', () => {
    expect(clientAge(daysAgo(29), NOW).level).toBe('fresh')
    expect(clientAge(daysAgo(30), NOW).level).toBe('warn')
    expect(clientAge(daysAgo(89), NOW).level).toBe('warn')
    expect(clientAge(daysAgo(90), NOW).level).toBe('stale')
  })

  it("retombe a zero quand l'horloge du poste avance sur le serveur", () => {
    // Un ecart negatif afficherait « -1 j dans cette etape », ce qui inquiete
    // sans rien dire de vrai.
    const future = new Date(NOW.getTime() + 60 * 60 * 1000).toISOString()

    expect(clientAge(future, NOW).label).toBe('0 j')
    expect(clientAge(future, NOW).level).toBe('fresh')
  })
})
