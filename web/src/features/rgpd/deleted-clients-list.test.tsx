import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { rgpdKeys } from './api'
import { DeletedClientsList } from './deleted-clients-list'

const CLIENT = { id: 'c1', name: 'Atelier Martin', deleted_at: '2026-11-01T09:00:00Z' }

function renderList(seed: (client: QueryClient) => void) {
  const client = new QueryClient({ defaultOptions: { queries: { staleTime: Infinity, retry: false } } })
  seed(client)

  return render(
    <QueryClientProvider client={client}>
      <DeletedClientsList page={1} onPage={() => {}} />
    </QueryClientProvider>,
  )
}

const withClients = (client: QueryClient) => client.setQueryData(rgpdKeys.deletedClients(1), { items: [CLIENT], total: 1, page: 1, page_size: 25 })

describe('DeletedClientsList', () => {
  it('affiche « Chargement… » tant que la liste arrive', () => {
    renderList(() => {})
    expect(screen.getByText('Chargement…')).toBeInTheDocument()
  })

  it('propose la suppression définitive quand rien n’est planifié', () => {
    renderList((client) => {
      withClients(client)
      client.setQueryData(rgpdKeys.scheduled('client', 'c1'), { items: [] })
    })
    expect(screen.getByRole('button', { name: 'Supprimer définitivement' })).toBeInTheDocument()
    expect(screen.queryByText(/Suppression prévue le/)).not.toBeInTheDocument()
  })

  it('montre la date prévue et Annuler à la place quand un effacement est planifié', () => {
    renderList((client) => {
      withClients(client)
      client.setQueryData(rgpdKeys.scheduled('client', 'c1'), {
        items: [{ id: 'e1', subject_type: 'client', subject_id: 'c1', status: 'scheduled', scheduled_for: '2026-12-01T09:00:00Z' }],
      })
    })
    expect(screen.getByText(/Suppression prévue le/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Annuler' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Supprimer définitivement' })).not.toBeInTheDocument()
  })
})
