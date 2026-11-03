import type { StatusPill } from '@/features/projects/format'
import type { ReportStatus } from '@/features/reports/api'

/** Ce qui retient l'envoi automatique d'un brouillon, en toutes lettres. */
export const HOLD_REASON_LABEL: Record<string, string> = {
  no_recipients: 'Aucun destinataire pour ce rapport',
  availability_missing: 'Disponibilité incomplète : un site de production n’a pas de mesure sur tout le mois, ou aucun site n’est surveillé',
  incident: 'Un incident a eu lieu ce mois-ci : le rapport mérite une relecture',
  backup_missing: 'Sauvegardes non surveillées sur ce projet',
  backup_stale: 'Sauvegardes en défaut à la fin du mois : la dernière a échoué ou date trop',
  domain_missing: 'Aucune échéance connue pour un domaine du projet',
  domain_expiring: 'Un domaine expire dans moins de 30 jours',
  updates_missing: 'Aucune mise à jour ce mois-ci : le robot ou la CI est peut-être en panne',
  update_failed: 'Le dernier lancement de mise à jour du mois a échoué',
}

export function holdReasonLabel(code: string): string {
  return HOLD_REASON_LABEL[code] ?? code
}

export const REPORT_STATUS: Record<ReportStatus, { label: string; color: string; pill: StatusPill }> = {
  draft: { label: 'Brouillon', color: '#8a8c91', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#4b4b4f' } },
  validated: { label: 'Validé', color: '#4956f4', pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' } },
  sent: { label: 'Envoyé', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } },
}
