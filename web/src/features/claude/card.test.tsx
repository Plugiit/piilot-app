import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import type { ClaudeAccount } from '@/types/api'

import { ClaudeCard, ClaudeConnections } from './card'

const URL = 'https://auth.agence.test/mcp'

function renderWith(account: ClaudeAccount) {
  const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity, retry: false } } })
  client.setQueryData(['claude', 'me'], account)
  return render(
    <QueryClientProvider client={client}>
      <ClaudeCard />
      <ClaudeConnections />
    </QueryClientProvider>,
  )
}

describe('ClaudeCard', () => {
  it('donne l’adresse et la commande de Claude Code quand rien n’est connecté', () => {
    renderWith({ mcp_url: URL, connections: [] })
    expect(screen.getAllByText(URL).length).toBeGreaterThan(0)
    expect(screen.getByText(`claude mcp add --transport http piilot ${URL}`)).toBeInTheDocument()
    expect(screen.queryByText('Connecté')).not.toBeInTheDocument()
    expect(screen.queryByText('Connexions en cours')).not.toBeInTheDocument()
  })

  it('liste les connexions et demande confirmation avant de couper', () => {
    renderWith({
      mcp_url: URL,
      connections: [
        {
          id: '00000000-0000-4000-8000-000000000001',
          client: 'Claude Code',
          surface: 'Claude Code',
          scopes: ['piilot:read', 'piilot:write'],
          created_at: '2026-11-01T09:00:00Z',
          last_used_at: null,
        },
      ],
    })
    expect(screen.getByText('Connecté')).toBeInTheDocument()
    expect(screen.getByText(/Lecture et modifications/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Déconnecter' }))
    const dialog = screen.getByRole('alertdialog', { name: 'Déconnecter Claude Code' })
    expect(dialog).toHaveFocus()

    fireEvent.click(screen.getByRole('button', { name: 'Annuler' }))
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Déconnecter' })).toHaveFocus()
  })
})
