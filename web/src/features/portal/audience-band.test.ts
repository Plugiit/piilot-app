import { describe, expect, it } from 'vitest'

import { defaultSite } from './audience-band'

const site = (id: string, visitors: number) => ({ id, name: id, domain: '', project_ids: [], visitors, series: [] })

describe('defaultSite', () => {
  it('montre le site le plus visite', () => {
    expect(defaultSite([site('vitrine', 300), site('boutique', 9000), site('blog', 40)])).toBe('boutique')
  })

  it('rien sans site', () => {
    expect(defaultSite([])).toBeUndefined()
  })
})
