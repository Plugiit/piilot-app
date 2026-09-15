import type { StatusPill } from '@/features/projects/format'
import type { ClientStatus } from '@/types/api'

/**
 * Etapes du pipeline commercial.
 *
 * L'ordre vit ici et non en base : la base valide les valeurs par un CHECK,
 * elle n'a pas a savoir que « devis » vient apres « lead ». C'est cet ordre que
 * le kanban suit, colonne par colonne.
 *
 * Reserve heritee du schema : « devis » ne pointe vers aucun devis — la
 * facturation reste sur l'ancienne plateforme. L'etape ne vaut que ce que
 * l'equipe y met a la main.
 */
export const CLIENT_STATUS_ORDER: ClientStatus[] = ['lead', 'devis', 'actif', 'veille', 'perdu']

/**
 * Libelles et teintes, sur le meme gabarit que les statuts de projet : une
 * couleur pleine pour les pastilles, un triplet pour les etiquettes.
 */
export const CLIENT_STATUS: Record<
  ClientStatus,
  { label: string; color: string; pill: StatusPill }
> = {
  lead: {
    label: 'Piste',
    color: '#73757c',
    pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#5b5d63' },
  },
  devis: {
    label: 'Devis',
    color: '#4956f4',
    pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' },
  },
  actif: {
    label: 'Actif',
    color: '#0db471',
    pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' },
  },
  veille: {
    label: 'En veille',
    color: '#eab308',
    pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' },
  },
  perdu: {
    label: 'Perdu',
    color: '#d4183d',
    pill: { bg: '#ffe8ec', border: '#ffd0d8', text: '#a30f2c' },
  },
}

/**
 * Anciennete d'une carte dans son etape, telle que le kanban l'affiche.
 *
 * Le serveur renvoie une date et non une duree : un ecart calcule la-bas
 * serait faux des la seconde suivante, et le cache de TanStack Query le
 * garderait faux jusqu'a la prochaine invalidation.
 *
 * `level` decide de la teinte. Le seuil est le meme pour les cinq etapes :
 * c'est le choix retenu — une regle unique, sans exception a retenir, au prix
 * de colonnes « Actif » durablement en rouge, ce qui y est normal et non
 * alarmant.
 */
export type ClientAgeLevel = 'fresh' | 'warn' | 'stale'

export interface ClientAge {
  /** Duree abregee : « 3 j », « 5 sem. », « 8 mois », « 2 ans ». */
  label: string
  level: ClientAgeLevel
  /** Phrase entiere, pour le lecteur d'ecran et l'infobulle. */
  title: string
}

const DAY = 24 * 60 * 60 * 1000

export function clientAge(statusChangedAt: string, now: Date = new Date()): ClientAge {
  // `max(0)` : une horloge de poste en avance sur le serveur donnerait un ecart
  // negatif, et « -1 j dans cette etape » est pire qu'un arrondi a zero.
  const days = Math.max(0, Math.floor((now.getTime() - Date.parse(statusChangedAt)) / DAY))

  let label: string
  if (days < 7) label = `${days} j`
  else if (days < 30) label = `${Math.floor(days / 7)} sem.`
  else if (days < 365) label = `${Math.floor(days / 30)} mois`
  else {
    const years = Math.floor(days / 365)
    label = `${years} an${years > 1 ? 's' : ''}`
  }

  const level: ClientAgeLevel = days >= 90 ? 'stale' : days >= 30 ? 'warn' : 'fresh'

  return { label, level, title: `Dans cette étape depuis ${label.replace('.', '')}` }
}

/** Teintes de l'anciennete, du neutre a l'alerte. */
export const CLIENT_AGE_TINT: Record<ClientAgeLevel, string> = {
  fresh: '#73757c',
  warn: '#9a6a00',
  stale: '#a30f2c',
}
