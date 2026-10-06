import { PROJECT_STATUS, type StatusPill } from '@/features/projects/format'
import type { PortalDeliverable, PortalProject, PortalTicket } from '@/types/api'

/**
 * Les mots du portail sont ceux du client, pas ceux de l'agence : un livrable
 * « en attente » chez nous est un livrable « a valider » chez lui.
 */
export const PORTAL_PROJECT_STATUS: Record<PortalProject['status'], { label: string; color: string; pill: StatusPill }> = {
  cadrage: { ...PROJECT_STATUS.cadrage, label: 'Cadrage' },
  production: { ...PROJECT_STATUS.production, label: 'En production' },
  attente: { ...PROJECT_STATUS.attente, label: 'En attente de votre retour' },
  livre: { ...PROJECT_STATUS.livre, label: 'Livré' },
  hebergement: { ...PROJECT_STATUS.hebergement, label: 'Hébergé par l’agence' },
}

export const PORTAL_DELIVERABLE_STATUS: Record<
  PortalDeliverable['status'],
  { label: string; color: string; pill: StatusPill }
> = {
  en_attente: {
    label: 'À valider',
    color: '#ff782b',
    pill: { bg: '#fff2ea', border: '#ffd9c2', text: '#b84a0c' },
  },
  retours: {
    label: 'Retours envoyés',
    color: '#4956f4',
    pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' },
  },
  valide: {
    label: 'Validé',
    color: '#0db471',
    pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' },
  },
}

const RELATIVE = new Intl.RelativeTimeFormat('fr-FR', { numeric: 'auto' })
const SHORT = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long' })

/** Une date passee, en mots : « aujourd’hui », « il y a 3 jours », « 12 mars ». */
export function since(iso: string, now: Date = new Date()): string {
  const days = Math.floor((now.getTime() - new Date(iso).getTime()) / 86_400_000)
  if (days <= 0) return 'aujourd’hui'
  if (days < 7) return RELATIVE.format(-days, 'day')

  return `le ${SHORT.format(new Date(iso))}`
}

/** Taille d'un fichier, lisible. */
export function fileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} Ko`

  return `${(bytes / (1024 * 1024)).toLocaleString('fr-FR', { maximumFractionDigits: 1 })} Mo`
}

/**
 * Statut d'une demande, dans les mots du client. Les etapes internes de
 * l'agence s'y regroupent : le meme decoupage que les e-mails.
 */
export function portalTicketStatus(status: PortalTicket['status']): { label: string; color: string; pill: StatusPill } {
  switch (status) {
    case 'backlog':
    case 'todo':
      return { label: 'Reçue', color: '#8a8c91', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#4b4b4f' } }
    case 'in_progress':
    case 'in_review':
      return { label: 'En cours de traitement', color: '#4956f4', pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' } }
    case 'ready_to_deploy':
      return { label: 'Prête à être mise en ligne', color: '#eab308', pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' } }
    case 'done':
      return { label: 'Résolue', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } }
    case 'annule':
      return { label: 'Fermée', color: '#c4c4c4', pill: { bg: '#f3f4f4', border: '#e8e8e9', text: '#73757c' } }
  }
}

/** Nature d'une demande : ce que le client choisit au depot. */
export const PORTAL_TRACKER: Record<PortalTicket['tracker'], { label: string; hint: string }> = {
  anomalie: { label: 'Un problème', hint: 'Quelque chose ne fonctionne pas comme prévu' },
  evolution: { label: 'Une évolution', hint: 'Ajouter ou changer quelque chose' },
  assistance: { label: 'Une question', hint: 'Besoin d’aide ou d’un renseignement' },
}

/** Priorites que le client peut choisir. */
export const PORTAL_PRIORITY: Record<'low' | 'normal' | 'high', { label: string; hint: string }> = {
  low: { label: 'Basse', hint: 'Quand vous aurez le temps' },
  normal: { label: 'Normale', hint: 'Dans les prochains jours' },
  high: { label: 'Haute', hint: 'Ça nous bloque' },
}
