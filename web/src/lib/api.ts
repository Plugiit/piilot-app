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
const BASE_URL: string = import.meta.env.VITE_API_URL ?? ''

export const api = createClient<paths>({
  baseUrl: BASE_URL,
  credentials: 'include',
  headers: { 'X-Requested-With': 'XMLHttpRequest' },
  fetch: fetchWithRefresh,
})

/** Routes qui portent leur propre preuve : un 401 chez elles est definitif. */
const NO_REFRESH = ['/api/v1/auth/login', '/api/v1/auth/refresh', '/api/v1/auth/logout', '/api/v1/auth/invitations', '/api/v1/auth/password']

let refreshing: Promise<boolean> | null = null

/**
 * Rafraichit la session, une seule fois a la fois.
 *
 * Dix requetes qui echouent ensemble a l'expiration du jeton d'acces ne
 * doivent pas lancer dix rafraichissements : le jeton de rafraichissement
 * tourne a chaque usage, et le second passage serait pris pour un rejeu —
 * qui revoque toute la famille. Le premier appel rafraichit, les autres
 * attendent sa reponse.
 */
function refreshSession(): Promise<boolean> {
  refreshing ??= fetch(apiUrl('/api/v1/auth/refresh'), {
    method: 'POST',
    credentials: 'include',
    headers: { 'X-Requested-With': 'XMLHttpRequest' },
  })
    .then((response) => response.ok)
    .catch(() => false)
    .finally(() => {
      refreshing = null
    })

  return refreshing
}

/**
 * `fetch` qui survit a l'expiration du jeton d'acces.
 *
 * Le jeton d'acces vit quinze minutes ; la session, trente jours. Entre les
 * deux, c'est le jeton de rafraichissement qui fait le lien — et c'est ici
 * qu'il sert : un 401 declenche un rafraichissement, puis la requete repart
 * telle quelle. Sans cela, chaque quart d'heure renvoyait a la connexion.
 *
 * La requete est clonee avant de partir : un corps deja lu ne se renvoie pas.
 */
export async function fetchWithRefresh(input: Request): Promise<Response> {
  const path = new URL(input.url).pathname
  const retry = NO_REFRESH.some((prefix) => path.startsWith(prefix)) ? null : input.clone()

  const response = await fetch(input)
  if (response.status !== 401 || retry === null) return response

  if (!(await refreshSession())) return response

  return fetch(retry)
}

/** Adresse absolue d'un chemin d'API — pour un lien que le navigateur suit lui-meme. */
export function apiUrl(path: string): string {
  return `${BASE_URL}${path}`
}

/**
 * Envoi d'un fichier en multipart.
 *
 * `openapi-fetch` serialise en JSON : le seul endpoint qui recoit un fichier
 * passe donc par `fetch` directement. Il reprend les memes reglages que le
 * client — cookie de session et en-tete anti-CSRF — et rend la meme
 * `HttpError`, pour que les appelants n'aient pas deux formes d'echec a
 * traiter.
 *
 * Le `Content-Type` n'est pas pose a la main : le navigateur doit y mettre la
 * frontiere multipart qu'il vient de tirer, et l'ecraser casserait le
 * decoupage cote serveur.
 */
export async function postFile<T>(path: string, field: string, file: File): Promise<T> {
  const form = new FormData()
  form.append(field, file)

  const response = await fetchWithRefresh(
    new Request(apiUrl(path), {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
      body: form,
    }),
  )

  if (!response.ok) {
    const payload = (await response.json().catch(() => ({}))) as Partial<ApiError>

    throw new HttpError(
      response.status,
      payload.code ?? 'UNKNOWN',
      payload.message ?? 'Une erreur est survenue',
      payload.details ?? {},
    )
  }

  return (await response.json()) as T
}

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
