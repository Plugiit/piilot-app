/**
 * Espaces de Piilot et leurs domaines.
 *
 * Par defaut tout tient sur un domaine. Quand l'instance donne a chaque espace
 * le sien (AUTH_URL, TEAM_URL, ADMIN_URL, CLIENT_URL cote serveur), le serveur
 * les ecrit dans une balise meta d'index.html, et ce module s'en sert pour
 * changer de domaine quand une navigation sort de l'espace courant — le
 * serveur fait de meme pour une adresse ouverte directement.
 *
 * La correspondance chemin → espace est le miroir de
 * api/internal/spaces/spaces.go : les deux doivent bouger ensemble.
 */

export type Space = 'auth' | 'team' | 'admin' | 'client'

export type SpaceUrls = Record<Space, string>

/** Domaines des espaces, ou `null` en mode mono-domaine. */
export function readSpaces(doc: Document | undefined = globalThis.document): SpaceUrls | null {
  const content = doc?.querySelector('meta[name="piilot-spaces"]')?.getAttribute('content')
  if (content == null || content === '') return null

  try {
    const parsed = JSON.parse(content) as Partial<SpaceUrls>
    if (parsed.auth && parsed.team && parsed.admin && parsed.client) return parsed as SpaceUrls
  } catch {
    // Une meta illisible vaut mono-domaine : mieux vaut rester ou l'on est.
  }

  return null
}

const SPACES = readSpaces()

/** Vrai quand chaque espace a son domaine. */
export function spacesEnabled(): boolean {
  return SPACES !== null
}

/**
 * Espace d'une adresse de l'application. `null` pour ce qui vaut partout : la
 * racine, que chaque espace redirige vers son accueil, et le compte.
 */
export function spaceOfPath(path: string): Space | null {
  const under = (prefix: string) => path === prefix || path.startsWith(`${prefix}/`)

  if (path === '/' || path === '') return null
  if (under('/compte')) return null
  if (under('/login') || under('/invitation') || under('/mot-de-passe-oublie') || under('/reinitialiser')) return 'auth'
  if (under('/client')) return 'client'
  // Le tableau de bord de l'agence est la racine du module PM : seule
  // l'adresse exacte releve de l'administration.
  if (path === '/pm' || path === '/pm/' || under('/parametres')) return 'admin'

  return 'team'
}

/** Espace du domaine courant, ou `null` hors des domaines configures. */
export function currentSpace(spaces: SpaceUrls | null = SPACES, origin: string = globalThis.location?.origin ?? ''): Space | null {
  if (spaces === null) return null

  return (Object.keys(spaces) as Space[]).find((space) => spaces[space] === origin) ?? null
}

/**
 * Adresse absolue vers laquelle partir quand `path` n'appartient pas a
 * l'espace du domaine courant, ou `null` quand on y est deja.
 */
export function crossSpaceUrl(
  path: string,
  search = '',
  spaces: SpaceUrls | null = SPACES,
  origin: string = globalThis.location?.origin ?? '',
): string | null {
  if (spaces === null) return null

  const current = currentSpace(spaces, origin)
  if (current === null) return null

  let target = spaceOfPath(path)
  if (target === null) {
    // Le compte n'a pas sa place dans le portail ni sur la connexion.
    if (path.startsWith('/compte') && (current === 'auth' || current === 'client')) target = 'team'
    else return null
  }
  if (target === current) return null

  return `${spaces[target]}${path}${search}`
}
