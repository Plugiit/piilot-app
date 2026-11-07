import type { DiscordStatus } from '@/types/api'

/**
 * Un code tire depuis la carte, en attente d'etre tape dans Discord.
 *
 * `before` garde la liaison telle qu'elle etait au tirage : c'est en la
 * comparant que la carte voit que le bot a lie le compte, sans confronter
 * l'horloge du navigateur a celle du serveur.
 */
export interface PendingCode {
  code: string
  expiresAt: Date
  before: Pick<DiscordStatus, 'discord_user_id' | 'linked_at'>
}

/** Ce que la carte montre. */
export type DiscordCard =
  | { kind: 'loading' }
  | { kind: 'error' }
  | { kind: 'unavailable' }
  | { kind: 'unlinked' }
  | { kind: 'waiting'; code: string; expiresAt: Date }
  | { kind: 'expired' }
  | { kind: 'linked'; discordUserId: string; linkedAt: Date; canRelink: boolean }

/**
 * Vrai quand la liaison a change depuis le tirage du code : le bot l'a
 * rachete. Une liaison cree toujours une nouvelle date ; seul un rachat par le
 * compte Discord deja lie ne change rien, et il n'y a alors rien a annoncer.
 */
export function linkedSince(status: DiscordStatus | undefined, pending: PendingCode): boolean {
  if (status?.linked !== true) return false
  return status.linked_at !== pending.before.linked_at || status.discord_user_id !== pending.before.discord_user_id
}

export function discordCard(
  status: DiscordStatus | undefined,
  failed: boolean,
  pending: PendingCode | null,
  now: Date,
): DiscordCard {
  if (status === undefined) return failed ? { kind: 'error' } : { kind: 'loading' }

  if (pending !== null && !linkedSince(status, pending)) {
    return now < pending.expiresAt ? { kind: 'waiting', code: pending.code, expiresAt: pending.expiresAt } : { kind: 'expired' }
  }

  if (status.linked && status.discord_user_id !== null && status.linked_at !== null) {
    // Sans bot branche, un nouveau code ne servirait a rien ; delier reste
    // possible, c'est une donnee du compte.
    return { kind: 'linked', discordUserId: status.discord_user_id, linkedAt: new Date(status.linked_at), canRelink: status.available }
  }

  return status.available ? { kind: 'unlinked' } : { kind: 'unavailable' }
}

const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })
const TIME = new Intl.DateTimeFormat('fr-FR', { hour: '2-digit', minute: '2-digit' })

/** « 7 novembre 2026 ». */
export const formatDay = (date: Date) => DAY.format(date)

/** « 14:32 ». */
export const formatTime = (date: Date) => TIME.format(date)
