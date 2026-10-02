import type { Account } from '@/types/api'

export type RoleCode = 'admin' | 'team' | 'client'

/**
 * Roles : libelle, couleur et ce qu'ils permettent, dit en une phrase. La
 * phrase sert au choix du role dans l'invitation : on choisit un usage, pas
 * un code.
 */
export const ROLE: Record<RoleCode, { label: string; color: string; hint: string }> = {
  admin: {
    label: 'Administrateur',
    color: '#ff782b',
    hint: 'Tout Piilot, y compris les comptes, les rôles et les mises à jour.',
  },
  team: {
    label: 'Équipe',
    color: '#4956f4',
    hint: 'Projets, tâches, temps, tickets et CRM, sans l’administration.',
  },
  client: {
    label: 'Client',
    color: '#0db471',
    hint: 'Le portail : les projets et tickets de son entreprise, rien d’autre.',
  },
}

export function roleOf(code: string): { label: string; color: string; hint: string } {
  return ROLE[code as RoleCode] ?? { label: code, color: '#73757c', hint: '' }
}

const RELATIVE = new Intl.RelativeTimeFormat('fr-FR', { numeric: 'auto' })

/**
 * Derniere connexion, en temps relatif : « il y a 3 jours » se lit plus vite
 * qu'une date, et c'est l'ordre de grandeur qui compte ici.
 */
export function lastSeen(account: Pick<Account, 'last_login_at'>, now = Date.now()): string {
  if (account.last_login_at === null) return 'Aucune connexion'

  const seconds = Math.round((new Date(account.last_login_at).getTime() - now) / 1000)
  const abs = Math.abs(seconds)

  if (abs < 60) return 'À l’instant'
  if (abs < 3600) return RELATIVE.format(Math.round(seconds / 60), 'minute')
  if (abs < 86_400) return RELATIVE.format(Math.round(seconds / 3600), 'hour')
  if (abs < 30 * 86_400) return RELATIVE.format(Math.round(seconds / 86_400), 'day')
  if (abs < 365 * 86_400) return RELATIVE.format(Math.round(seconds / (30 * 86_400)), 'month')
  return RELATIVE.format(Math.round(seconds / (365 * 86_400)), 'year')
}

/** Nom affichable, l'adresse a defaut : une invitation n'a pas toujours de nom. */
export function displayName(person: { firstname: string; lastname: string; email: string }): string {
  const name = `${person.firstname} ${person.lastname}`.trim()
  return name === '' ? person.email : name
}
