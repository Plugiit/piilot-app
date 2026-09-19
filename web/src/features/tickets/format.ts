import type { StatusPill } from '@/features/projects/format'
import type { TicketPriority, TicketStatus, TicketTracker } from '@/types/api'

/**
 * Libelles francais des trois nomenclatures des tickets.
 *
 * L'API renvoie des cles stables — `in_progress`, `critical` — et la traduction
 * vit ici. Traduire cote serveur figerait l'intitule dans le contrat : le
 * reformuler demanderait alors une migration ou une version d'endpoint, pour
 * un simple mot a l'ecran.
 *
 * Les teintes reprennent le gabarit des statuts de projet : un triplet par
 * etiquette, pour que les pastilles se ressemblent d'un module a l'autre.
 */

/** Ordre du cycle de vie, du depot a la cloture. */
export const TICKET_STATUS_ORDER: TicketStatus[] = [
  'backlog',
  'todo',
  'in_progress',
  'in_review',
  'ready_to_deploy',
  'done',
  'annule',
]

export const TICKET_STATUS: Record<TicketStatus, { label: string; pill: StatusPill }> = {
  backlog: { label: 'Backlog', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#5b5d63' } },
  todo: { label: 'À faire', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#3f4147' } },
  in_progress: { label: 'En cours', pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' } },
  in_review: { label: 'En relecture', pill: { bg: '#f6eefe', border: '#e6d5fd', text: '#7134c9' } },
  ready_to_deploy: {
    label: 'Prêt à déployer',
    pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' },
  },
  done: { label: 'Terminé', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } },
  annule: { label: 'Annulé', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#8a8c91' } },
}

/** Ordre d'affichage des natures, du plus subi au plus choisi. */
export const TICKET_TRACKER_ORDER: TicketTracker[] = ['anomalie', 'evolution', 'assistance']

export const TICKET_TRACKER: Record<TicketTracker, { label: string; pill: StatusPill }> = {
  anomalie: { label: 'Anomalie', pill: { bg: '#ffe8ec', border: '#ffd0d8', text: '#a30f2c' } },
  evolution: { label: 'Évolution', pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' } },
  assistance: { label: 'Assistance', pill: { bg: '#e7f6ff', border: '#c7e9fb', text: '#0a6c9a' } },
}

/** Ordre croissant de gravite : c'est aussi l'ordre de lecture d'un menu. */
export const TICKET_PRIORITY_ORDER: TicketPriority[] = [
  'low',
  'normal',
  'high',
  'urgent',
  'critical',
]

/**
 * Les priorites basse et normale restent grises.
 *
 * Colorer les cinq ferait un tableau ou tout crie, et ou plus rien ne ressort :
 * seules les trois qui demandent une action se distinguent.
 */
export const TICKET_PRIORITY: Record<TicketPriority, { label: string; pill: StatusPill }> = {
  low: { label: 'Basse', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#8a8c91' } },
  normal: { label: 'Normale', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#5b5d63' } },
  high: { label: 'Haute', pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' } },
  urgent: { label: 'Urgente', pill: { bg: '#ffeadd', border: '#ffd3b9', text: '#a2440a' } },
  critical: { label: 'Critique', pill: { bg: '#ffe8ec', border: '#ffd0d8', text: '#a30f2c' } },
}

/**
 * Date et heure, en un seul bloc.
 *
 * Le tableau montre des dates de creation et de mise a jour : sans l'heure,
 * deux tickets ouverts le meme jour sembleraient simultanes.
 */
const DATE_TIME = new Intl.DateTimeFormat('fr-FR', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})

export function formatDateTime(iso: string): string {
  return DATE_TIME.format(new Date(iso))
}
