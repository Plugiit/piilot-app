import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import type { OAuthConsent } from '@/types/api'

import { ConsentForm } from './consent'

const REQUEST: OAuthConsent = {
  id: '00000000-0000-4000-8000-000000000001',
  client: 'Claude',
  surface: 'claude.ai, Claude Desktop et mobile',
  host: 'claude.ai',
  redirect_host: 'claude.ai',
  local: false,
  scopes: ['piilot:read', 'piilot:write'],
  expires_at: '2026-11-10T10:10:00Z',
}

function renderForm(request: OAuthConsent = REQUEST) {
  const onDecide = vi.fn()
  render(<ConsentForm request={request} email="compte@agence.test" pending={false} onDecide={onDecide} />)
  return onDecide
}

describe('ConsentForm', () => {
  it('dit qui demande, pour quel compte, et où part le code', () => {
    renderForm()
    expect(screen.getByText('compte@agence.test')).toBeInTheDocument()
    expect(screen.getAllByText('claude.ai')).toHaveLength(2)
    expect(screen.getByRole('note')).toHaveTextContent('vous venez de lancer cette connexion vous-même')
    expect(screen.getByText('Écrire ou envoyer un e-mail à un client')).toBeInTheDocument()
  })

  it('accorde l’écriture par défaut, et la retire si on la décoche', () => {
    const onDecide = renderForm()
    expect(screen.getByRole('checkbox', { name: /Lire/ })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Autoriser' }))
    expect(onDecide).toHaveBeenLastCalledWith(true, true)

    fireEvent.click(screen.getByRole('checkbox', { name: /Modifier/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Autoriser' }))
    expect(onDecide).toHaveBeenLastCalledWith(true, false)
  })

  it('refuse', () => {
    const onDecide = renderForm()
    fireEvent.click(screen.getByRole('button', { name: 'Refuser' }))
    expect(onDecide).toHaveBeenCalledWith(false, false)
  })

  it('n’offre pas l’écriture si elle n’est pas demandée', () => {
    const onDecide = renderForm({ ...REQUEST, scopes: ['piilot:read'] })
    expect(screen.queryByRole('checkbox', { name: /Modifier/ })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Autoriser' }))
    expect(onDecide).toHaveBeenCalledWith(true, false)
  })

  it('signale un retour vers la machine locale', () => {
    renderForm({ ...REQUEST, client: 'Claude Code', surface: 'Claude Code', redirect_host: 'localhost', local: true })
    expect(screen.getByRole('note')).toHaveTextContent('Retour vers cet ordinateur')
  })
})
