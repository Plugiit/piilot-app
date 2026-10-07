import { describe, expect, it } from 'vitest'

import { fillTemplate } from '@/features/tickets/replies'
import type { ReplyTemplate, TicketDetail } from '@/types/api'

const template: ReplyTemplate = {
  id: 't',
  title: 'Accès',
  body: 'Bonjour {prenom},\n\nPour la demande {numero} « {sujet} » sur {projet} : c’est fait.',
  updated_at: '2026-10-07T10:00:00Z',
}

function ticket(patch: Partial<TicketDetail>): TicketDetail {
  return {
    numero: 47,
    subject: 'Formulaire',
    project: { id: 'p', name: 'Vitrine' },
    reporter: null,
    requester: null,
    ...patch,
  } as TicketDetail
}

describe('fillTemplate', () => {
  it('remplit les variables avec le compte du portail', () => {
    const t = ticket({ reporter: { id: 'u', firstname: 'Inès', lastname: 'Client', initials: 'IC', avatar_url: null, role: 'client' } })
    expect(fillTemplate(template, t)).toBe('Bonjour Inès,\n\nPour la demande #47 « Formulaire » sur Vitrine : c’est fait.')
  })

  it('prend le prenom de qui a ecrit par e-mail', () => {
    expect(fillTemplate(template, ticket({ requester: { name: 'Paul Contact', email: 'p@c.fr' } }))).toMatch(/^Bonjour Paul,/)
  })

  it('ne laisse pas d’espace orpheline sans prenom', () => {
    expect(fillTemplate(template, ticket({}))).toMatch(/^Bonjour,/)
  })
})
