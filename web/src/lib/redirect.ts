import { readSpaces } from '@/lib/spaces'

/**
 * Le retour apres connexion, s'il reste dans l'application.
 *
 * `redirect` arrive dans l'adresse de /login : n'importe qui peut forger un
 * lien de connexion qui renverrait, une fois les identifiants saisis, vers un
 * site tiers. Seuls passent un chemin de l'application, ou une adresse sur
 * l'un de ses domaines.
 */
export function safeRedirect(
  raw: string | undefined,
  origin: string = globalThis.location?.origin ?? '',
  spaces: Record<string, string> | null = readSpaces(),
): string | undefined {
  if (raw === undefined || raw === '') return undefined

  // Chemin de l'application. « //hote » et « /\hote » sont des adresses
  // absolues pour un navigateur.
  if (raw.startsWith('/')) {
    return raw.startsWith('//') || raw.startsWith('/\\') ? undefined : raw
  }

  let url: URL
  try {
    url = new URL(raw)
  } catch {
    return undefined
  }
  const allowed = new Set([origin, ...Object.values(spaces ?? {})].filter((o) => o !== ''))
  return allowed.has(url.origin) ? raw : undefined
}
