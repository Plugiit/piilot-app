import type { ClaudeAccount, ClaudeConnection } from '@/types/api'

export type Scope = 'piilot:read' | 'piilot:write'

/** Ce que chaque portee laisse faire a Claude, dans les mots de l'ecran. */
export const SCOPE_TEXT: Record<Scope, { label: string; detail: string }> = {
  'piilot:read': {
    label: 'Lire',
    detail: 'Projets, tâches, jalons, tickets, livrables et votre temps, avec les droits de votre compte.',
  },
  'piilot:write': {
    label: 'Modifier',
    detail: 'Créer, modifier et supprimer projets, tâches, jalons, saisies de temps et pièces jointes.',
  },
}

/** Ce que Claude ne fait jamais, quelles que soient les portees. */
export const NEVER = [
  'Écrire ou envoyer un e-mail à un client',
  'Publier sur le portail client : livrable, réponse à un ticket, fichier partagé',
  'Lancer une mise à jour de site',
  'Gérer les comptes, les rôles ou les réglages',
]

export function scopesLabel(scopes: readonly string[]): string {
  return scopes.includes('piilot:write') ? 'Lecture et modifications' : 'Lecture seule'
}

/** Les portees envoyees a la decision : la lecture toujours, l'ecriture si cochee. */
export function decisionScopes(write: boolean): Scope[] {
  return write ? ['piilot:read', 'piilot:write'] : ['piilot:read']
}

export type ClaudeCard =
  | { kind: 'loading' }
  | { kind: 'error' }
  | { kind: 'idle'; mcpUrl: string }
  | { kind: 'connected'; mcpUrl: string; count: number; lastUsedAt: Date | null }

export function claudeCard(account: ClaudeAccount | undefined, failed: boolean): ClaudeCard {
  if (account === undefined) return failed ? { kind: 'error' } : { kind: 'loading' }
  if (account.connections.length === 0) return { kind: 'idle', mcpUrl: account.mcp_url }
  return {
    kind: 'connected',
    mcpUrl: account.mcp_url,
    count: account.connections.length,
    lastUsedAt: latestUse(account.connections),
  }
}

function latestUse(connections: readonly ClaudeConnection[]): Date | null {
  let latest: Date | null = null
  for (const c of connections) {
    if (c.last_used_at === null) continue
    const at = new Date(c.last_used_at)
    if (latest === null || at > latest) latest = at
  }
  return latest
}

const day = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })
const time = new Intl.DateTimeFormat('fr-FR', { hour: '2-digit', minute: '2-digit' })

export function formatDay(date: Date): string {
  return day.format(date)
}

/** « Jamais utilisée », « Utilisée le 7 novembre 2026 à 14:32 ». */
export function formatLastUse(at: string | null): string {
  if (at === null) return 'Jamais utilisée'
  const d = new Date(at)
  return `Utilisée le ${day.format(d)} à ${time.format(d)}`
}

/** Le message d'un refus avant consentement, selon le code renvoye par /oauth/authorize. */
export function consentErrorMessage(code: string): string {
  switch (code) {
    case 'invalid_client':
      return 'Seul Claude peut se connecter à Piilot. Ce lien vient d’une autre application.'
    case 'invalid_request':
      return 'La demande de connexion est incomplète, ou son adresse de retour n’est pas celle de Claude.'
    default:
      return 'Cette demande de connexion ne peut pas aboutir. Relancez-la depuis Claude.'
  }
}

/** Les instructions de connexion, par surface. */
export function claudeCodeCommand(mcpUrl: string): string {
  return `claude mcp add --transport http piilot ${mcpUrl}`
}
