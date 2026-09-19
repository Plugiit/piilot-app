import { describe, expect, it } from 'vitest'

import { HttpError, unwrap } from '@/lib/api'

describe('unwrap', () => {
  it('retourne la donnee quand la reponse est un succes', () => {
    const data = { id: '1', name: 'Plugiit' }

    expect(unwrap({ data, response: new Response(null, { status: 200 }) })).toBe(data)
  })

  it("transforme l'erreur de l'API en HttpError typee", () => {
    const call = () =>
      unwrap({
        error: { code: 'VALIDATION_FAILED', message: 'Données invalides', details: { email: 'requis' } },
        response: new Response(null, { status: 422 }),
      })

    expect(call).toThrow(HttpError)

    try {
      call()
    } catch (error) {
      const httpError = error as HttpError
      expect(httpError.status).toBe(422)
      expect(httpError.code).toBe('VALIDATION_FAILED')
      expect(httpError.details).toEqual({ email: 'requis' })
      expect(httpError.isUnauthorized).toBe(false)
    }
  })

  it('signale les sessions expirees via isUnauthorized', () => {
    try {
      unwrap({
        error: { code: 'UNAUTHORIZED', message: 'Authentification requise' },
        response: new Response(null, { status: 401 }),
      })
      expect.unreachable("unwrap aurait du lever")
    } catch (error) {
      expect((error as HttpError).isUnauthorized).toBe(true)
    }
  })

  it('reste exploitable quand le corps d erreur est vide', () => {
    try {
      unwrap({ error: {}, response: new Response(null, { status: 500 }) })
      expect.unreachable("unwrap aurait du lever")
    } catch (error) {
      const httpError = error as HttpError
      expect(httpError.code).toBe('UNKNOWN')
      expect(httpError.details).toEqual({})
    }
  })
})
