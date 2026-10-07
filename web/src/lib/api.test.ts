import { afterEach, describe, expect, it, vi } from 'vitest'

import { fetchWithRefresh } from './api'

function response(status: number) {
  return new Response(null, { status })
}

describe('fetchWithRefresh', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('rafraichit la session sur un 401 puis rejoue la requete', async () => {
    const calls: string[] = []
    vi.stubGlobal('fetch', vi.fn(async (input: Request | string) => {
      const url = typeof input === 'string' ? input : input.url
      calls.push(new URL(url, 'http://x').pathname)
      if (url.endsWith('/auth/refresh')) return response(200)
      return response(calls.length === 1 ? 401 : 200)
    }))

    const res = await fetchWithRefresh(new Request('http://x/api/v1/admin/projects'))
    expect(res.status).toBe(200)
    expect(calls).toEqual(['/api/v1/admin/projects', '/api/v1/auth/refresh', '/api/v1/admin/projects'])
  })

  it('ne rejoue pas quand le rafraichissement echoue', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: Request | string) => {
      const url = typeof input === 'string' ? input : input.url
      return response(url.endsWith('/auth/refresh') ? 401 : 401)
    }))

    const res = await fetchWithRefresh(new Request('http://x/api/v1/auth/me'))
    expect(res.status).toBe(401)
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(2)
  })

  it('laisse la connexion et le rafraichissement repondre 401 sans insister', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => response(401)))

    await fetchWithRefresh(new Request('http://x/api/v1/auth/login', { method: 'POST' }))
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1)
  })

  it('un seul rafraichissement pour plusieurs 401 simultanes', async () => {
    let refreshes = 0
    vi.stubGlobal('fetch', vi.fn(async (input: Request | string) => {
      const url = typeof input === 'string' ? input : input.url
      if (url.endsWith('/auth/refresh')) {
        refreshes += 1
        await new Promise((resolve) => setTimeout(resolve, 10))
        return response(200)
      }
      return response(refreshes === 0 ? 401 : 200)
    }))

    const results = await Promise.all([
      fetchWithRefresh(new Request('http://x/api/v1/admin/projects')),
      fetchWithRefresh(new Request('http://x/api/v1/admin/clients')),
      fetchWithRefresh(new Request('http://x/api/v1/admin/tasks')),
    ])
    expect(results.map((r) => r.status)).toEqual([200, 200, 200])
    expect(refreshes).toBe(1)
  })
})
