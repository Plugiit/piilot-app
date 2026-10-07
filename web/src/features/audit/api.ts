import { keepPreviousData, queryOptions } from '@tanstack/react-query'

import { api, apiUrl, unwrap } from '@/lib/api'

export interface AuditFilters {
  action?: string
  search?: string
  from?: string
  to?: string
}

export function auditQuery(f: AuditFilters, page: number) {
  return queryOptions({
    queryKey: ['audit', f, page],
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/audit', { params: { query: { ...f, page } } })),
    placeholderData: keepPreviousData,
  })
}

/** Adresse de l'export CSV des memes filtres. */
export function auditExportUrl(f: AuditFilters): string {
  const params = new URLSearchParams()
  for (const [k, v] of Object.entries(f)) {
    if (v !== undefined && v !== '') params.set(k, v)
  }
  return apiUrl(`/api/v1/admin/audit/export?${params.toString()}`)
}

/** Les familles d'actions que le filtre propose. */
export const AUDIT_FAMILIES: { value: string; label: string }[] = [
  { value: 'auth.', label: 'Connexions et mots de passe' },
  { value: 'account.', label: 'Comptes' },
  { value: 'invitation.', label: 'Invitations' },
  { value: 'role.', label: 'Rôles et permissions' },
  { value: '*.deleted', label: 'Suppressions' },
  { value: 'export.', label: 'Exports' },
  { value: 'portal.', label: 'Portail client' },
  { value: 'secret.', label: 'Secrets' },
  { value: 'settings.', label: 'Réglages' },
  { value: 'inbound.', label: 'Tri des e-mails' },
  { value: 'system.', label: 'Mises à jour' },
]

/** Une action, dans les mots de l'ecran. */
export const AUDIT_ACTION: Record<string, string> = {
  'auth.login': 'Connexion',
  'auth.login_failed': 'Connexion refusée',
  'auth.login_rate_limited': 'Connexion bloquée (trop d’essais)',
  'auth.password_changed': 'Mot de passe changé',
  'auth.password_reset': 'Mot de passe réinitialisé',
  'account.email_changed': 'Adresse e-mail changée',
  'account.role_changed': 'Rôle changé',
  'account.disabled': 'Compte désactivé',
  'account.enabled': 'Compte réactivé',
  'account.password_reset_link': 'Lien de réinitialisation créé',
  'invitation.created': 'Invitation envoyée',
  'invitation.resent': 'Invitation renvoyée',
  'invitation.revoked': 'Invitation annulée',
  'invitation.accepted': 'Invitation acceptée',
  'role.permissions_changed': 'Permissions d’un rôle changées',
  'system.update_requested': 'Mise à jour lancée',
  'client.deleted': 'Client supprimé',
  'contact.deleted': 'Contact supprimé',
  'interaction.deleted': 'Interaction supprimée',
  'project.deleted': 'Projet supprimé',
  'file.deleted': 'Fichier supprimé',
  'milestone.deleted': 'Jalon supprimé',
  'project_template.deleted': 'Modèle de projet supprimé',
  'task.deleted': 'Tâche supprimée',
  'service.deleted': 'Service supprimé',
  'time_entry.deleted': 'Saisie de temps supprimée',
  'reply_template.deleted': 'Réponse type supprimée',
  'secret.git_rotated': 'Secret Git changé',
  'secret.inbound_webhook_rotated': 'Secret des webhooks e-mail changé',
  'settings.inbound_mail_updated': 'Réglages des e-mails entrants modifiés',
  'inbound.ticket_created': 'Ticket ouvert depuis un e-mail',
  'inbound.attached': 'E-mail ajouté à un ticket',
  'inbound.dismissed': 'E-mail écarté',
  'export.time_report': 'Export des temps',
  'export.audit': 'Export du journal d’audit',
  'portal.file_downloaded': 'Fichier téléchargé (portail)',
  'portal.deliverable_decided': 'Réponse à un livrable (portail)',
  'portal.deliverable_decided_by_link': 'Réponse à un livrable (lien e-mail)',
}
