/**
 * Registre des entreprises : retrouver un etablissement par son SIRET.
 *
 * La source est l'API Recherche d'entreprises de l'Etat
 * (recherche-entreprises.api.gouv.fr) : gratuite, sans cle, et ouverte aux
 * appels du navigateur. C'est donc l'ecran qui l'interroge, et non le serveur
 * — l'API de Piilot ne fait aucun appel externe pendant une requete. Le
 * serveur ne recoit que le resultat, une fois le formulaire envoye.
 */

const REGISTRY_URL = 'https://recherche-entreprises.api.gouv.fr/search'

/** Etablissement tel que le formulaire s'en sert. */
export interface RegistryCompany {
  siret: string
  siren: string
  /** Raison sociale, telle que declaree. */
  legalName: string
  /** Forme juridique en clair, ou vide si le code est inconnu ici. */
  legalForm: string
  address: string
  postalCode: string
  city: string
  vatNumber: string
  /** L'etablissement est-il encore ouvert ? */
  active: boolean
  /** Le nom est-il masque par l'entreprise (diffusion partielle) ? */
  hidden: boolean
}

/** Retire espaces et points d'un SIRET tape ou colle. */
export function normalizeSiret(raw: string): string {
  return raw.replace(/[\s. ]/g, '')
}

/**
 * Cle de Luhn d'un SIRET. Les bureaux de La Poste (SIREN 356000000) font
 * exception : leur cle est la somme simple des chiffres, multiple de 5. La
 * meme regle est verifiee par le serveur.
 */
export function isValidSiret(siret: string): boolean {
  if (!/^\d{14}$/.test(siret)) return false

  let sum = 0
  let plain = 0
  for (let i = 0; i < 14; i++) {
    let d = Number(siret[i])
    plain += d
    if ((14 - i) % 2 === 0) {
      d *= 2
      if (d > 9) d -= 9
    }
    sum += d
  }

  return sum % 10 === 0 || (siret.startsWith('356000000') && plain % 5 === 0)
}

/** Numero de TVA intracommunautaire francais, deduit du SIREN. */
export function vatOfSiren(siren: string): string {
  let mod = 0
  for (const digit of siren) mod = (mod * 10 + Number(digit)) % 97
  const key = (12 + 3 * mod) % 97

  return `FR${String(key).padStart(2, '0')}${siren}`
}

/**
 * Compare deux raisons sociales sans tenir compte de la casse, des accents,
 * de la ponctuation ni des espaces : « Maison Aubert » et « MAISON-AUBERT »
 * designent la meme entreprise.
 */
export function sameLegalName(a: string, b: string): boolean {
  const key = (value: string) =>
    value
      .normalize('NFD')
      .replace(/[̀-ͯ]/g, '')
      .toUpperCase()
      .replace(/[^A-Z0-9]/g, '')

  return key(a) !== '' && key(a) === key(b)
}

/** Une reponse qui ne ressemble pas a celle attendue. */
export class RegistryError extends Error {}

interface RawEtablissement {
  siret?: string
  adresse?: string | null
  code_postal?: string | null
  libelle_commune?: string | null
  etat_administratif?: string | null
  numero_voie?: string | null
  indice_repetition?: string | null
  type_voie?: string | null
  libelle_voie?: string | null
  complement_adresse?: string | null
}

interface RawResult {
  siren: string
  nom_complet?: string | null
  nom_raison_sociale?: string | null
  nature_juridique?: string | null
  etat_administratif?: string | null
  statut_diffusion?: string | null
  siege?: RawEtablissement
  matching_etablissements?: RawEtablissement[]
}

/**
 * Cherche un etablissement par son SIRET. Rend `null` quand le registre ne le
 * connait pas, leve une erreur quand le registre ne repond pas.
 */
export async function lookupSiret(siret: string, signal?: AbortSignal): Promise<RegistryCompany | null> {
  const url = `${REGISTRY_URL}?q=${encodeURIComponent(siret)}&page=1&per_page=1`
  const res = await fetch(url, { signal, headers: { Accept: 'application/json' } })

  if (res.status === 429) throw new RegistryError('Le registre est très sollicité, réessayez dans un instant.')
  if (!res.ok) throw new RegistryError('Le registre ne répond pas pour le moment.')

  const body = (await res.json()) as { results?: RawResult[] }
  const result = body.results?.[0]
  if (result === undefined || !siret.startsWith(result.siren)) return null

  // La recherche porte sur toute l'entreprise : l'etablissement vise est le
  // siege, ou l'un de ceux que la recherche a reconnus.
  const etab =
    result.siege?.siret === siret
      ? result.siege
      : result.matching_etablissements?.find((item) => item.siret === siret)
  if (etab === undefined) return null

  const hidden = result.statut_diffusion === 'P'
  const legalName = hidden ? '' : (result.nom_raison_sociale ?? result.nom_complet ?? '')

  return {
    siret,
    siren: result.siren,
    legalName: legalName.trim(),
    legalForm: legalFormLabel(result.nature_juridique ?? ''),
    address: streetOf(etab),
    postalCode: etab.code_postal ?? '',
    city: etab.libelle_commune ?? '',
    vatNumber: vatOfSiren(result.siren),
    active: etab.etat_administratif === 'A',
    hidden,
  }
}

/**
 * Voie de l'etablissement, sans le code postal ni la commune qui ont leurs
 * propres champs. Le siege arrive en morceaux ; les autres etablissements en
 * une ligne, dont on retire la fin.
 */
function streetOf(etab: RawEtablissement): string {
  const parts = [etab.numero_voie, etab.indice_repetition, etab.type_voie, etab.libelle_voie].filter(
    (part): part is string => typeof part === 'string' && part !== '',
  )
  if (parts.length > 0) {
    const street = parts.join(' ')
    return etab.complement_adresse ? `${etab.complement_adresse}, ${street}` : street
  }

  const full = etab.adresse ?? ''
  const tail = [etab.code_postal, etab.libelle_commune].filter(Boolean).join(' ')

  return tail !== '' && full.endsWith(tail) ? full.slice(0, -tail.length).trim() : full
}

/**
 * Formes juridiques les plus courantes, par code INSEE a quatre chiffres. Les
 * autres se nomment par leur famille (deux premiers chiffres) : la saisie
 * reste modifiable, et une famille juste vaut mieux qu'un code nu.
 */
const LEGAL_FORMS: Record<string, string> = {
  '1000': 'Entrepreneur individuel',
  '5202': 'Société en nom collectif (SNC)',
  '5306': 'Société en commandite simple',
  '5308': 'Société en commandite par actions',
  '5385': 'Société d’exercice libéral en commandite par actions',
  '5485': 'Société d’exercice libéral à responsabilité limitée (SELARL)',
  '5498': 'SARL unipersonnelle (EURL)',
  '5499': 'Société à responsabilité limitée (SARL)',
  '5599': 'SA à conseil d’administration',
  '5699': 'SA à directoire',
  '5710': 'Société par actions simplifiée (SAS)',
  '5720': 'Société par actions simplifiée à associé unique (SASU)',
  '5785': 'Société d’exercice libéral par actions simplifiée (SELAS)',
  '5800': 'Société européenne',
  '6220': 'Groupement d’intérêt économique (GIE)',
  '6540': 'Société civile immobilière (SCI)',
  '6599': 'Société civile',
  '9210': 'Association non déclarée',
  '9220': 'Association déclarée',
  '9300': 'Fondation',
}

const LEGAL_FAMILIES: Record<string, string> = {
  '10': 'Entrepreneur individuel',
  '21': 'Indivision',
  '22': 'Société créée de fait',
  '23': 'Société en participation',
  '31': 'Personne morale de droit étranger',
  '32': 'Personne morale de droit étranger',
  '41': 'Établissement public industriel ou commercial',
  '51': 'Société coopérative commerciale',
  '52': 'Société en nom collectif (SNC)',
  '53': 'Société en commandite',
  '54': 'Société à responsabilité limitée (SARL)',
  '55': 'Société anonyme à conseil d’administration',
  '56': 'Société anonyme à directoire',
  '57': 'Société par actions simplifiée (SAS)',
  '58': 'Société européenne',
  '61': 'Caisse d’épargne et de prévoyance',
  '62': 'Groupement d’intérêt économique (GIE)',
  '63': 'Société coopérative agricole',
  '64': 'Société d’assurance mutuelle',
  '65': 'Société civile',
  '69': 'Personne morale de droit privé',
  '71': 'Administration de l’État',
  '72': 'Collectivité territoriale',
  '73': 'Établissement public administratif',
  '74': 'Personne morale de droit public',
  '81': 'Organisme de protection sociale',
  '82': 'Organisme mutualiste',
  '83': 'Comité social et économique',
  '84': 'Organisme professionnel',
  '85': 'Organisme de retraite',
  '91': 'Syndicat de propriétaires',
  '92': 'Association loi 1901',
  '93': 'Fondation',
  '99': 'Personne morale de droit privé',
}

export function legalFormLabel(code: string): string {
  return LEGAL_FORMS[code] ?? LEGAL_FAMILIES[code.slice(0, 2)] ?? ''
}
