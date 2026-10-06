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
 * Espace d'une adresse de l'application.
 *
 * `team` designe le back-office en general : il se sert sur le domaine de
 * l'equipe comme sur celui de l'administration, ou les administrateurs font
 * tout leur travail. `admin` designe les seuls ecrans d'administration — le
 * tableau de bord de l'agence et les parametres. `null` pour la racine, que
 * chaque domaine redirige vers son accueil.
 */
export function spaceOfPath(path: string): Space | null {
  const under = (prefix: string) => path === prefix || path.startsWith(`${prefix}/`)

  if (path === '/' || path === '') return null
  if (under('/login') || under('/invitation') || under('/mot-de-passe-oublie') || under('/reinitialiser')) return 'auth'
  if (under('/client')) return 'client'
  // Le tableau de bord de l'agence est la racine du module PM : seule
  // l'adresse exacte releve de l'administration.
  if (path === '/pm' || path === '/pm/' || under('/parametres')) return 'admin'

  return 'team'
}

/** Domaine du back-office pour un role : l'administration pour un admin. */
export function backOfficeOf(role: string): Space {
  return role === 'admin' ? 'admin' : 'team'
}

/** Espace du domaine courant, ou `null` hors des domaines configures. */
export function currentSpace(spaces: SpaceUrls | null = SPACES, origin: string = globalThis.location?.origin ?? ''): Space | null {
  if (spaces === null) return null

  return (Object.keys(spaces) as Space[]).find((space) => spaces[space] === origin) ?? null
}

/** Origine d'un espace, en mode multi-domaines. */
export function spaceUrl(space: Space, spaces: SpaceUrls | null = SPACES): string | null {
  return spaces === null ? null : spaces[space]
}

/**
 * Adresse absolue vers laquelle partir quand `path` n'appartient pas au
 * domaine courant, ou `null` quand on y est deja.
 *
 * `role` vient de la session quand elle est connue : un administrateur va
 * droit sur son domaine, sans passer par celui de l'equipe. Inconnue — avant
 * la connexion —, le back-office se rejoint par le domaine de l'equipe.
 */
export function crossSpaceUrl(
  path: string,
  search = '',
  role?: string,
  spaces: SpaceUrls | null = SPACES,
  origin: string = globalThis.location?.origin ?? '',
): string | null {
  if (spaces === null) return null

  const current = currentSpace(spaces, origin)
  if (current === null) return null

  const wanted = spaceOfPath(path)
  if (wanted === null) return null

  // Un client n'a pas de back-office : la garde de celui-ci le renvoie a son
  // portail, qui le menera ensuite sur son domaine.
  if (role === 'client' && (wanted === 'team' || wanted === 'admin')) return null

  let target: Space
  if (wanted === 'team') {
    if (role !== undefined) target = backOfficeOf(role)
    else if (current === 'team' || current === 'admin') return null
    else target = 'team'
  } else if (wanted === 'admin' && role !== undefined && role !== 'admin') {
    // L'equipe n'a pas d'administration a rejoindre : la garde du
    // back-office la renvoie a son accueil, sur son domaine.
    return null
  } else {
    target = wanted
  }

  if (target === current) return null

  return `${spaces[target]}${path}${search}`
}
