import createClient from 'openapi-fetch'

import type { ApiError, paths } from '@/types/api'

/**
 * Client HTTP unique de l'application.
 *
 * `credentials: 'include'` fait suivre le cookie de session httpOnly : le
 * jeton n'est jamais lisible par le JavaScript, donc une faille XSS ne permet
 * pas de l'exfiltrer. En contrepartie l'API doit autoriser explicitement
 * l'origine de l'admin (ADMIN_ORIGINS cote Go).
 */
export const api = createClient<paths>({
  baseUrl: import.meta.env.VITE_API_URL ?? '',
  credentials: 'include',
  headers: { 'X-Requested-With': 'XMLHttpRequest' },
})

/** Erreur portant le code stable renvoye par l'API. */
export class HttpError extends Error {
  readonly status: number
  readonly code: string
  readonly details: Record<string, unknown>

  constructor(status: number, code: string, message: string, details: Record<string, unknown> = {}) {
    super(message)
    this.name = 'HttpError'
    this.status = status
    this.code = code
    this.details = details
  }

  /** Vrai quand la session a expire ou n'existe pas. */
  get isUnauthorized() {
    return this.status === 401
  }
}

/**
 * Normalise la reponse d'openapi-fetch en valeur ou exception.
 *
 * TanStack Query attend une promesse rejetee pour declencher ses retries et
 * ses etats d'erreur ; sans ce pont, chaque hook devrait tester `error`
 * manuellement.
 */
export function unwrap<T>(result: {
  data?: T
  error?: unknown
  response: Response
}): T {
  if (result.error !== undefined) {
    const payload = result.error as Partial<ApiError>

    throw new HttpError(
      result.response.status,
      payload.code ?? 'UNKNOWN',
      payload.message ?? 'Une erreur est survenue',
      payload.details ?? {},
    )
  }

  return result.data as T
}
