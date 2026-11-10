import { describe, expect, it } from 'vitest'

import type { ClaudeAccount } from '@/types/api'

import { claudeCard, claudeCodeCommand, consentErrorMessage, decisionScopes, formatLastUse, scopesLabel } from './format'

const URL = 'https://auth.agence.test/mcp'

function account(connections: ClaudeAccount['connections']): ClaudeAccount {
  return { mcp_url: URL, connections }
}

function connection(id: string, lastUsed: string | null): ClaudeAccount['connections'][number] {
  return {
    id,
    client: 'Claude',
    surface: 'claude.ai, Claude Desktop et mobile',
    scopes: ['piilot:read'],
    created_at: '2026-11-01T09:00:00Z',
    last_used_at: lastUsed,
  }
}

describe('claudeCard', () => {
  it('attend, puis dit l’échec, tant que rien n’est lu', () => {
    expect(claudeCard(undefined, false)).toEqual({ kind: 'loading' })
    expect(claudeCard(undefined, true)).toEqual({ kind: 'error' })
  })

  it('donne l’adresse quand rien n’est connecté', () => {
    expect(claudeCard(account([]), false)).toEqual({ kind: 'idle', mcpUrl: URL })
  })

  it('compte les connexions et garde le dernier appel', () => {
    const card = claudeCard(
      account([connection('a', '2026-11-03T10:00:00Z'), connection('b', null), connection('c', '2026-11-05T08:00:00Z')]),
      false,
    )
    expect(card).toEqual({ kind: 'connected', mcpUrl: URL, count: 3, lastUsedAt: new Date('2026-11-05T08:00:00Z') })
  })

  it('n’invente pas d’usage', () => {
    expect(claudeCard(account([connection('a', null)]), false)).toMatchObject({ kind: 'connected', lastUsedAt: null })
  })
})

describe('portées', () => {
  it('lecture toujours, écriture si cochée', () => {
    expect(decisionScopes(false)).toEqual(['piilot:read'])
    expect(decisionScopes(true)).toEqual(['piilot:read', 'piilot:write'])
    expect(scopesLabel(['piilot:read'])).toBe('Lecture seule')
    expect(scopesLabel(['piilot:read', 'piilot:write'])).toBe('Lecture et modifications')
  })
})

describe('textes', () => {
  it('dit pourquoi une demande est refusée', () => {
    expect(consentErrorMessage('invalid_client')).toMatch(/Seul Claude/)
    expect(consentErrorMessage('invalid_request')).toMatch(/adresse de retour/)
    expect(consentErrorMessage('autre')).toMatch(/Relancez-la/)
  })

  it('date le dernier usage', () => {
    expect(formatLastUse(null)).toBe('Jamais utilisée')
    expect(formatLastUse('2026-11-07T13:32:00Z')).toMatch(/^Utilisée le 7 novembre 2026 à \d\d:32$/)
  })

  it('donne la commande de Claude Code', () => {
    expect(claudeCodeCommand(URL)).toBe(`claude mcp add --transport http piilot ${URL}`)
  })
})
