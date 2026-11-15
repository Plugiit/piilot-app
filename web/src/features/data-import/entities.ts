import type { ImportItem, ImportRow } from '@/types/api'

export type DataEntity = ImportItem['entity']
export type ImportStatus = ImportItem['status']
export type RowOutcome = ImportRow['outcome']

/**
 * Les entites importables, dans l'ordre recommande : un contact rattache un
 * client, un projet aussi, une saisie de temps un projet. Importer dans un
 * autre ordre produit des lignes « client introuvable ».
 */
export const ENTITIES: { value: DataEntity; label: string; plural: string }[] = [
  { value: 'clients', label: 'Client', plural: 'Clients' },
  { value: 'contacts', label: 'Contact', plural: 'Contacts' },
  { value: 'projects', label: 'Projet', plural: 'Projets' },
  { value: 'time_entries', label: 'Temps', plural: 'Temps passé' },
]

export const IMPORT_ORDER = 'clients → contacts → projets → temps'

export function entityLabel(entity: string): string {
  return ENTITIES.find((e) => e.value === entity)?.plural ?? entity
}

export const STATUS_LABEL: Record<ImportStatus, string> = {
  uploaded: 'Fichier déposé',
  queued: 'En attente',
  running: 'En cours',
  done: 'Terminé',
  failed: 'Échoué',
  cancelled: 'Annulé',
}

/** Vrai tant que le serveur traite l'import : l'écran interroge alors l'API en boucle. */
export function isActive(status: ImportStatus): boolean {
  return status === 'queued' || status === 'running'
}

export const OUTCOME_LABEL: Record<RowOutcome, string> = {
  created: 'Créées',
  skipped: 'Ignorées',
  error: 'En erreur',
}

/** Une ligne rapportée, au singulier : « créée », « ignorée », « en erreur ». */
export const OUTCOME_ROW_LABEL: Record<RowOutcome, string> = {
  created: 'Créée',
  skipped: 'Ignorée',
  error: 'En erreur',
}

/** Motifs d'une ligne ignorée ou refusée, dans les mots de l'écran. */
export const REASON_LABEL: Record<string, string> = {
  invalid: 'Valeur invalide',
  already_exists: 'Existe déjà',
  already_imported: 'Déjà importée lors d’un import précédent',
  duplicate_siret: 'SIRET déjà connu',
  duplicate_name: 'Nom déjà connu',
  duplicate_email: 'Adresse e-mail déjà connue',
  unknown_client: 'Client introuvable',
  unknown_project: 'Projet introuvable',
  unknown_user: 'Compte introuvable',
}

export function reasonLabel(reason: string): string {
  if (reason === '') return '—'

  return REASON_LABEL[reason] ?? reason
}

/** Codes d'échec d'un import (`error` de l'import, pas d'une ligne). */
export const FAILURE_LABEL: Record<string, string> = {
  interrupted: 'Le serveur a redémarré pendant l’import. Relancez-le : les lignes déjà importées seront ignorées.',
  file_unreadable: 'Le fichier n’a pas pu être relu.',
  internal: 'Une erreur interne a interrompu l’import.',
}

export function failureLabel(code: string): string {
  return FAILURE_LABEL[code] ?? 'L’import a échoué.'
}

export const STEPS = ['apercu', 'colonnes', 'simulation', 'import', 'rapport'] as const
export type Step = (typeof STEPS)[number]

export const STEP_LABEL: Record<Step, string> = {
  apercu: 'Aperçu',
  colonnes: 'Colonnes',
  simulation: 'Simulation',
  import: 'Import',
  rapport: 'Rapport',
}
