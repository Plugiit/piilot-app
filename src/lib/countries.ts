/**
 * Pays et indicatifs telephoniques.
 *
 * Une liste choisie plutot que les deux cent cinquante du monde : l'agence
 * travaille en Europe, en Amerique du Nord et au Maghreb, et une liste de
 * quarante entrees se parcourt sans champ de recherche. Ajouter un pays tient
 * en une ligne le jour ou il en faut un.
 *
 * Le drapeau n'est pas ecrit : il se deduit du code ISO. Les deux lettres se
 * transposent en « indicateurs regionaux », le couple de symboles que les
 * systemes affichent en drapeau. Cela evite quarante emoji colles a la main,
 * dont un seul de travers passerait inapercu.
 *
 * Reserve : Windows ne dessine pas ces drapeaux et affiche les deux lettres a
 * la place. C'est lisible, et le nom comme l'indicatif restent a cote.
 */

export interface Country {
  /** Code ISO 3166-1 alpha-2, et identifiant de l'entree. */
  code: string
  name: string
  /** Indicatif, signe plus compris. */
  dial: string
  /**
   * Prefixe national, a retirer devant l'indicatif.
   *
   * Le zero francais, le huit lituanien, le « 06 » hongrois : ils servent a
   * composer depuis l'interieur du pays et ne font pas partie du numero
   * international. Vide pour les pays qui n'en ont pas — l'Italie garde son
   * zero, l'Amerique du Nord n'en a jamais eu.
   */
  trunk: string
  /**
   * Decoupage du numero en groupes de chiffres, pour la lecture.
   *
   * Approximation assumee : la vraie regle depend souvent de l'operateur et de
   * la longueur, et la tenir exactement demanderait les metadonnees de
   * libphonenumber. Ce groupement sert le confort de lecture, il ne valide
   * rien. Les chiffres au-dela des groupes prevus sont rendus par paires.
   */
  groups: number[]
}

/** Le drapeau correspondant a un code ISO. */
export function flagOf(code: string): string {
  const BASE = 0x1f1e6 // le symbole « A » des indicateurs regionaux

  return String.fromCodePoint(
    ...[...code.toUpperCase()].map((letter) => BASE + letter.charCodeAt(0) - 65),
  )
}

/**
 * La France en tete : c'est le cas courant, et le faire remonter evite de
 * derouler la liste a chaque saisie. Le reste suit l'ordre alphabetique.
 */
export const COUNTRIES: Country[] = [
  { code: 'FR', name: 'France', dial: '+33', trunk: '0', groups: [1, 2, 2, 2, 2] },
  { code: 'DE', name: 'Allemagne', dial: '+49', trunk: '0', groups: [3, 3, 4] },
  { code: 'AD', name: 'Andorre', dial: '+376', trunk: '', groups: [3, 3] },
  { code: 'SA', name: 'Arabie saoudite', dial: '+966', trunk: '0', groups: [2, 3, 4] },
  { code: 'AR', name: 'Argentine', dial: '+54', trunk: '0', groups: [2, 4, 4] },
  { code: 'AU', name: 'Australie', dial: '+61', trunk: '0', groups: [3, 3, 3] },
  { code: 'AT', name: 'Autriche', dial: '+43', trunk: '0', groups: [3, 3, 4] },
  { code: 'BE', name: 'Belgique', dial: '+32', trunk: '0', groups: [3, 2, 2, 2] },
  { code: 'BR', name: 'Brésil', dial: '+55', trunk: '0', groups: [2, 5, 4] },
  { code: 'BG', name: 'Bulgarie', dial: '+359', trunk: '0', groups: [3, 3, 3] },
  { code: 'CA', name: 'Canada', dial: '+1', trunk: '', groups: [3, 3, 4] },
  { code: 'CN', name: 'Chine', dial: '+86', trunk: '0', groups: [3, 4, 4] },
  { code: 'CY', name: 'Chypre', dial: '+357', trunk: '', groups: [2, 6] },
  { code: 'KR', name: 'Corée du Sud', dial: '+82', trunk: '0', groups: [2, 4, 4] },
  { code: 'HR', name: 'Croatie', dial: '+385', trunk: '0', groups: [2, 3, 4] },
  { code: 'DK', name: 'Danemark', dial: '+45', trunk: '', groups: [2, 2, 2, 2] },
  { code: 'AE', name: 'Émirats arabes unis', dial: '+971', trunk: '0', groups: [2, 3, 4] },
  { code: 'ES', name: 'Espagne', dial: '+34', trunk: '', groups: [3, 3, 3] },
  { code: 'EE', name: 'Estonie', dial: '+372', trunk: '', groups: [4, 4] },
  { code: 'US', name: 'États-Unis', dial: '+1', trunk: '', groups: [3, 3, 4] },
  { code: 'FI', name: 'Finlande', dial: '+358', trunk: '0', groups: [2, 3, 4] },
  { code: 'GR', name: 'Grèce', dial: '+30', trunk: '', groups: [3, 3, 4] },
  { code: 'HU', name: 'Hongrie', dial: '+36', trunk: '06', groups: [2, 3, 4] },
  { code: 'IN', name: 'Inde', dial: '+91', trunk: '0', groups: [5, 5] },
  { code: 'IE', name: 'Irlande', dial: '+353', trunk: '0', groups: [2, 3, 4] },
  { code: 'IL', name: 'Israël', dial: '+972', trunk: '0', groups: [2, 3, 4] },
  { code: 'IT', name: 'Italie', dial: '+39', trunk: '', groups: [3, 3, 4] },
  { code: 'JP', name: 'Japon', dial: '+81', trunk: '0', groups: [2, 4, 4] },
  { code: 'LV', name: 'Lettonie', dial: '+371', trunk: '', groups: [4, 4] },
  { code: 'LB', name: 'Liban', dial: '+961', trunk: '0', groups: [2, 3, 3] },
  { code: 'LT', name: 'Lituanie', dial: '+370', trunk: '8', groups: [3, 5] },
  { code: 'LU', name: 'Luxembourg', dial: '+352', trunk: '', groups: [3, 3, 3] },
  { code: 'MT', name: 'Malte', dial: '+356', trunk: '', groups: [4, 4] },
  { code: 'MA', name: 'Maroc', dial: '+212', trunk: '0', groups: [1, 2, 2, 2, 2] },
  { code: 'MX', name: 'Mexique', dial: '+52', trunk: '', groups: [2, 4, 4] },
  { code: 'MC', name: 'Monaco', dial: '+377', trunk: '', groups: [2, 2, 2, 2] },
  { code: 'NO', name: 'Norvège', dial: '+47', trunk: '', groups: [3, 2, 3] },
  { code: 'NL', name: 'Pays-Bas', dial: '+31', trunk: '0', groups: [1, 4, 4] },
  { code: 'PL', name: 'Pologne', dial: '+48', trunk: '', groups: [3, 3, 3] },
  { code: 'PT', name: 'Portugal', dial: '+351', trunk: '', groups: [3, 3, 3] },
  { code: 'CZ', name: 'Tchéquie', dial: '+420', trunk: '', groups: [3, 3, 3] },
  { code: 'RO', name: 'Roumanie', dial: '+40', trunk: '0', groups: [3, 3, 3] },
  { code: 'GB', name: 'Royaume-Uni', dial: '+44', trunk: '0', groups: [4, 6] },
  { code: 'SN', name: 'Sénégal', dial: '+221', trunk: '', groups: [2, 3, 2, 2] },
  { code: 'SG', name: 'Singapour', dial: '+65', trunk: '', groups: [4, 4] },
  { code: 'SK', name: 'Slovaquie', dial: '+421', trunk: '0', groups: [3, 3, 3] },
  { code: 'SI', name: 'Slovénie', dial: '+386', trunk: '0', groups: [2, 3, 3] },
  { code: 'SE', name: 'Suède', dial: '+46', trunk: '0', groups: [2, 3, 2, 2] },
  { code: 'CH', name: 'Suisse', dial: '+41', trunk: '0', groups: [2, 3, 2, 2] },
  { code: 'TN', name: 'Tunisie', dial: '+216', trunk: '', groups: [2, 3, 3] },
  { code: 'TR', name: 'Turquie', dial: '+90', trunk: '0', groups: [3, 3, 2, 2] },
]

/** Le pays retenu par defaut, faute d'indicatif reconnu. */
export const DEFAULT_COUNTRY = COUNTRIES[0]!

/**
 * Separe un numero en indicatif et partie nationale.
 *
 * Les indicatifs sont essayes du plus long au plus court : « +33 » et « +3 »
 * commencent pareil, et tester dans l'ordre de la liste ferait reconnaitre le
 * mauvais pays. Plusieurs pays partagent un meme indicatif — +1 pour les
 * Etats-Unis et le Canada — et le premier de la liste l'emporte alors : le
 * numero compose est le meme, seule l'etiquette differe.
 */
export function splitPhone(value: string): { country: Country; national: string } {
  const trimmed = value.trim()

  const match = [...COUNTRIES]
    .sort((a, b) => b.dial.length - a.dial.length)
    .find((country) => trimmed.startsWith(country.dial))

  if (match === undefined) {
    return { country: DEFAULT_COUNTRY, national: trimmed }
  }

  return { country: match, national: trimmed.slice(match.dial.length) }
}

/** Retire le prefixe national quand il est present. */
export function stripTrunk(country: Country, digits: string): string {
  if (country.trunk !== '' && digits.startsWith(country.trunk)) {
    return digits.slice(country.trunk.length)
  }

  return digits
}

/**
 * Espace les chiffres d'un numero selon son pays.
 *
 * Les groupes prevus sont consommes dans l'ordre ; ce qui depasse est rendu
 * par paires plutot que d'un bloc, parce qu'un numero plus long que la regle
 * reste plus lisible coupe que colle.
 */
export function formatNational(country: Country, digits: string): string {
  const parts: string[] = []
  let rest = digits

  for (const size of country.groups) {
    if (rest === '') break

    parts.push(rest.slice(0, size))
    rest = rest.slice(size)
  }

  while (rest !== '') {
    parts.push(rest.slice(0, 2))
    rest = rest.slice(2)
  }

  return parts.join(' ')
}

/**
 * Numero tel qu'on le lit : l'indicatif, puis le national espace selon le pays.
 *
 * Les listes affichent la valeur stockee, qui est la forme internationale
 * compacte — « +33612345678 » se lit mal. Le formulaire fait deja ce decoupage
 * pour la saisie ; celle-ci le refait pour la lecture seule.
 */
export function formatPhone(value: string): string {
  const { country, national } = splitPhone(value)

  return national === '' ? '' : `${country.dial} ${formatNational(country, national)}`
}
