/**
 * Mesure d'audience de Piilot lui-meme, par Umami.
 *
 * Seule une instance qui la configure la recoit (UMAMI_TRACKER_URL et
 * UMAMI_TRACKER_WEBSITE_ID cote serveur) : le serveur ecrit l'adresse du
 * script dans une balise meta d'index.html, et ce module le charge.
 *
 * Le script n'est jamais servi tel quel dans index.html : il enverrait les
 * adresses visitees completes, et celles de l'application portent des secrets
 * et des donnees personnelles — jeton d'invitation ou de reinitialisation dans
 * le chemin, `?token=` d'une reponse a un livrable, `?email=` de la connexion.
 * Le filtre est pose avant que le script soit charge : si ce module ne
 * s'execute pas, rien ne part.
 */

export type TrackerConfig = { src: string; websiteId: string }

/** Ce que le script Umami s'apprete a envoyer ; seuls url et referrer nous importent. */
type TrackerPayload = { url?: string; referrer?: string; [key: string]: unknown }

const BEFORE_SEND = 'piilotTrackerBeforeSend'

declare global {
  interface Window {
    piilotTrackerBeforeSend?: (type: string, payload: TrackerPayload) => TrackerPayload | null
  }
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

// Les segments fixes des routes sont tous des mots en minuscules
// (« projets », « mot-de-passe-oublie ») : on garde ceux-la, on remplace le
// reste. Une liste des routes a jeton serait plus courte, mais une route
// ajoutee demain y manquerait sans que rien ne le signale.
const SLUG = /^[a-z]+(?:-[a-z]+)*$/

function redactSegment(segment: string): string {
  if (segment === '' || SLUG.test(segment)) return segment
  if (UUID.test(segment)) return ':id'
  return ':token'
}

/**
 * Adresse bonne a envoyer : sans query ni fragment, identifiants et jetons du
 * chemin remplaces. Chaine vide si l'adresse est illisible.
 */
export function sanitizeUrl(raw: string): string {
  if (raw === '') return ''

  let url: URL
  try {
    url = new URL(raw)
  } catch {
    return ''
  }

  url.search = ''
  url.hash = ''
  url.pathname = url.pathname.split('/').map(redactSegment).join('/')
  return url.toString()
}

/** Filtre appele par Umami avant chaque envoi. `null` annule l'envoi. */
export function beforeSend(_type: string, payload: TrackerPayload): TrackerPayload | null {
  if (typeof payload.url !== 'string') return null

  return {
    ...payload,
    url: sanitizeUrl(payload.url),
    referrer: typeof payload.referrer === 'string' ? sanitizeUrl(payload.referrer) : '',
  }
}

/** Configuration posee par le serveur, ou `null` si l'instance ne mesure rien. */
export function readTracker(doc: Document | undefined = globalThis.document): TrackerConfig | null {
  const content = doc?.querySelector('meta[name="piilot-tracker"]')?.getAttribute('content')
  if (content == null || content === '') return null

  try {
    const parsed = JSON.parse(content) as Partial<TrackerConfig>
    if (typeof parsed.src === 'string' && parsed.src.startsWith('https://') && typeof parsed.websiteId === 'string' && UUID.test(parsed.websiteId)) {
      return { src: parsed.src, websiteId: parsed.websiteId }
    }
  } catch {
    // Une meta illisible vaut absence de mesure.
  }

  return null
}

/** Pose le filtre puis charge le script Umami, si l'instance en a un. */
export function startTracker(doc: Document = document, win: Window = window): void {
  const config = readTracker(doc)
  if (config === null) return

  win[BEFORE_SEND] = beforeSend

  const script = doc.createElement('script')
  script.src = config.src
  script.dataset.websiteId = config.websiteId
  script.dataset.beforeSend = BEFORE_SEND
  // Redondant avec le filtre, mais coupe la query avant meme qu'il ne la voie.
  script.dataset.excludeSearch = 'true'
  script.dataset.excludeHash = 'true'
  doc.head.append(script)
}
