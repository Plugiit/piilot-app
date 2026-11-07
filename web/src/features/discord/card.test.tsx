import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import type { DiscordStatus } from '@/types/api'

import { DiscordCard } from './card'

/** La carte sur une liaison deja en cache : aucun appel ne part. */
function renderWith(status: DiscordStatus) {
  const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity, retry: false } } })
  client.setQueryData(['discord', 'me'], status)
  return render(
    <QueryClientProvider client={client}>
      <DiscordCard />
    </QueryClientProvider>,
  )
}

describe('DiscordCard', () => {
  it('propose de lier le compte quand le bot est branché', () => {
    renderWith({ available: true, linked: false, discord_user_id: null, linked_at: null })
    expect(screen.getByRole('button', { name: 'Lier mon compte' })).toBeEnabled()
  })

  it('ne propose rien tant que le bot n’est pas branché', () => {
    renderWith({ available: false, linked: false, discord_user_id: null, linked_at: null })
    expect(screen.getByText('Bot non branché')).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('demande confirmation avant de délier, et y met le focus', () => {
    renderWith({ available: true, linked: true, discord_user_id: '100000000000000001', linked_at: '2026-11-01T09:00:00Z' })
    expect(screen.getByText('Lié')).toBeInTheDocument()
    expect(screen.getByText('100000000000000001')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Délier' }))
    const dialog = screen.getByRole('alertdialog', { name: 'Confirmer la déliaison' })
    expect(dialog).toHaveFocus()

    fireEvent.click(screen.getByRole('button', { name: 'Annuler' }))
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Changer' })).toHaveFocus()
  })

  it('laisse délier sans proposer de changer quand le bot est débranché', () => {
    renderWith({ available: false, linked: true, discord_user_id: '100000000000000001', linked_at: '2026-11-01T09:00:00Z' })
    expect(screen.getByRole('button', { name: 'Délier' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Changer' })).not.toBeInTheDocument()
  })
})
