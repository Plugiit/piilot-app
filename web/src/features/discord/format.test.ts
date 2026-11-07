import { describe, expect, it } from 'vitest'

import type { DiscordStatus } from '@/types/api'

import { discordCard, linkedSince, type PendingCode } from './format'

const NOW = new Date('2026-11-07T14:00:00Z')

const unlinked: DiscordStatus = { available: true, linked: false, discord_user_id: null, linked_at: null }
const linked: DiscordStatus = {
  available: true,
  linked: true,
  discord_user_id: '100000000000000001',
  linked_at: '2026-11-01T09:00:00Z',
}

function pending(before: DiscordStatus, expiresAt = '2026-11-07T14:10:00Z'): PendingCode {
  return {
    code: 'K7QM-4XZD',
    expiresAt: new Date(expiresAt),
    before: { discord_user_id: before.discord_user_id, linked_at: before.linked_at },
  }
}

describe('discordCard', () => {
  it('attend la lecture, puis signale son échec', () => {
    expect(discordCard(undefined, false, null, NOW)).toEqual({ kind: 'loading' })
    expect(discordCard(undefined, true, null, NOW)).toEqual({ kind: 'error' })
  })

  it('propose de se lier quand le bot est branché', () => {
    expect(discordCard(unlinked, false, null, NOW)).toEqual({ kind: 'unlinked' })
  })

  it('dit que le bot manque, sans proposer de code', () => {
    expect(discordCard({ ...unlinked, available: false }, false, null, NOW)).toEqual({ kind: 'unavailable' })
  })

  it('montre la liaison, et ne propose d’en changer que si le bot est branché', () => {
    expect(discordCard(linked, false, null, NOW)).toEqual({
      kind: 'linked',
      discordUserId: '100000000000000001',
      linkedAt: new Date('2026-11-01T09:00:00Z'),
      canRelink: true,
    })
    expect(discordCard({ ...linked, available: false }, false, null, NOW)).toMatchObject({ kind: 'linked', canRelink: false })
  })

  it('montre le code tant qu’il est valable, puis son expiration', () => {
    expect(discordCard(unlinked, false, pending(unlinked), NOW)).toEqual({
      kind: 'waiting',
      code: 'K7QM-4XZD',
      expiresAt: new Date('2026-11-07T14:10:00Z'),
    })
    expect(discordCard(unlinked, false, pending(unlinked, '2026-11-07T14:00:00Z'), NOW)).toEqual({ kind: 'expired' })
  })

  it('garde le code affiché quand un compte déjà lié en change', () => {
    expect(discordCard(linked, false, pending(linked), NOW)).toMatchObject({ kind: 'waiting' })
  })

  it('repasse à la liaison dès que le bot a racheté le code', () => {
    const after: DiscordStatus = { ...linked, discord_user_id: '100000000000000002', linked_at: '2026-11-07T14:02:00Z' }
    expect(discordCard(after, false, pending(linked), NOW)).toMatchObject({ kind: 'linked', discordUserId: '100000000000000002' })
  })
})

describe('linkedSince', () => {
  it('voit une première liaison', () => {
    expect(linkedSince(linked, pending(unlinked))).toBe(true)
  })

  it('voit un changement de compte Discord', () => {
    const after: DiscordStatus = { ...linked, discord_user_id: '100000000000000002', linked_at: '2026-11-07T14:02:00Z' }
    expect(linkedSince(after, pending(linked))).toBe(true)
  })

  it('ne voit rien tant que la liaison est la même, ou absente', () => {
    expect(linkedSince(linked, pending(linked))).toBe(false)
    expect(linkedSince(unlinked, pending(unlinked))).toBe(false)
    expect(linkedSince(undefined, pending(unlinked))).toBe(false)
  })
})
