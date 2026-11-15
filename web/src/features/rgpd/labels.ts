import type { RgpdErasureItem, RgpdExportItem } from '@/types/api'

export type SubjectType = RgpdExportItem['subject_type']
export type ExportStatus = RgpdExportItem['status']
export type ErasureStatus = RgpdErasureItem['status']

export const SUBJECT_LABEL: Record<SubjectType, string> = {
  client: 'Client',
  user: 'Compte',
}

export const EXPORT_STATUS_LABEL: Record<ExportStatus, string> = {
  queued: 'En attente',
  running: 'En cours',
  ready: 'Prêt',
  failed: 'Échoué',
  expired: 'Expiré',
}

export const ERASURE_STATUS_LABEL: Record<ErasureStatus, string> = {
  scheduled: 'Planifié',
  cancelled: 'Annulé',
  running: 'En cours',
  done: 'Effectué',
  failed: 'Échoué',
}

export const ERASURE_STATUSES = Object.keys(ERASURE_STATUS_LABEL) as ErasureStatus[]

export const ERASURE_REASON_LABEL: Record<RgpdErasureItem['reason'], string> = {
  request: 'Demande',
  inactivity: 'Inactivité',
}

/** Codes d'échec d'un export (`error`), dans les mots de l'écran. */
export const EXPORT_FAILURE_LABEL: Record<string, string> = {
  too_large: 'L’archive dépasse la taille maximale autorisée.',
  interrupted: 'Le serveur a redémarré pendant la construction. Relancez l’export.',
  internal: 'Une erreur interne a interrompu l’export.',
}

export function exportFailureLabel(code: string): string {
  return EXPORT_FAILURE_LABEL[code] ?? 'L’export a échoué.'
}

/** Codes d'erreur renvoyés par l'API pour une demande d'effacement ou d'annulation. */
export const RGPD_ERROR_LABEL: Record<string, string> = {
  LAST_ADMIN: 'C’est le dernier administrateur actif : il doit en rester au moins un.',
  CONFLICT: 'Un effacement est déjà en cours pour ce sujet, ou n’est plus planifié.',
  NOT_FOUND: 'Ce sujet n’existe plus.',
}

/** Message d'une erreur RGPD : le libellé du code s'il est connu, sinon celui du serveur. */
export function rgpdErrorMessage(code: string, fallback: string): string {
  return RGPD_ERROR_LABEL[code] ?? fallback
}

/** Vrai tant que le serveur construit l'archive : l'écran interroge alors l'API en boucle. */
export function isExportActive(status: ExportStatus): boolean {
  return status === 'queued' || status === 'running'
}

const HOUR_MS = 3_600_000

/** Date d'exécution d'un effacement demandé maintenant, selon le délai de rétractation. */
export function plannedDate(now: Date, graceHours: number): Date {
  return new Date(now.getTime() + graceHours * HOUR_MS)
}

/** Délai de rétractation en mots : « 30 jours » quand il tombe juste, sinon en heures. */
export function graceLabel(hours: number): string {
  if (hours > 0 && hours % 24 === 0) {
    const days = hours / 24

    return `${days} jour${days > 1 ? 's' : ''}`
  }

  return `${hours} heure${hours > 1 ? 's' : ''}`
}

const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })
const WHEN = new Intl.DateTimeFormat('fr-FR', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })

export function formatDay(value: Date | string): string {
  return DAY.format(new Date(value))
}

export function formatWhen(value: Date | string): string {
  return WHEN.format(new Date(value))
}

/** Normalise une saisie pour la comparer : espaces repliés, casse et accents ignorés. */
function normalize(value: string): string {
  return value.normalize('NFC').replace(/\s+/g, ' ').trim().toLocaleLowerCase('fr-FR')
}

/** La confirmation est bonne quand la saisie redit le nom du sujet (non vide). */
export function confirmMatches(typed: string, subjectName: string): boolean {
  const expected = normalize(subjectName)

  return expected !== '' && normalize(typed) === expected
}

/** Total des lignes supprimées ou vidées et des fichiers d'un effacement exécuté ; null s'il n'a rien à dire. */
export function erasureTotals(summary: Record<string, unknown>): { rows: number; files: number } | null {
  const counts = summary.counts
  if (typeof counts !== 'object' || counts === null) return null

  let rows = 0
  for (const value of Object.values(counts)) if (typeof value === 'number') rows += value

  const files = typeof summary.files === 'number' ? summary.files : 0

  return { rows, files }
}
