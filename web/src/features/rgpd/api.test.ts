import { beforeEach, describe, expect, it, vi } from 'vitest'

const get = vi.fn()

vi.mock('@/lib/api', () => ({
  api: { GET: (...args: unknown[]) => get(...args) },
  apiUrl: (path: string) => path,
  unwrap: (result: { data: unknown }) => result.data,
}))

import { rgpdKeys, scheduledErasureQuery } from './api'

describe('scheduledErasureQuery', () => {
  beforeEach(() => get.mockReset())

  it('interroge l’API pour le seul sujet demandé', async () => {
    get.mockResolvedValue({ data: { items: [] } })

    const options = scheduledErasureQuery('user', 'u1', true)
    await (options.queryFn as () => Promise<unknown>)()

    expect(get).toHaveBeenCalledWith('/api/v1/admin/rgpd/erasures', {
      params: { query: { status: 'scheduled', subject_type: 'user', subject_id: 'u1', page: 1 } },
    })
  })

  it('a une clé par sujet et reste sous la racine rgpd', () => {
    expect(scheduledErasureQuery('client', 'c1', true).queryKey).toEqual(rgpdKeys.scheduled('client', 'c1'))
    expect(rgpdKeys.scheduled('client', 'c1')).not.toEqual(rgpdKeys.scheduled('user', 'c1'))
    expect(rgpdKeys.scheduled('user', 'u1').slice(0, 1)).toEqual(rgpdKeys.all)
  })

  it('reste inactive sans droit', () => {
    expect(scheduledErasureQuery('user', 'u1', false).enabled).toBe(false)
  })
})
