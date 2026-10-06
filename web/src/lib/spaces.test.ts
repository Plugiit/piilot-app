import { describe, expect, it } from 'vitest'

import { crossSpaceUrl, currentSpace, readSpaces, spaceOfPath, type SpaceUrls } from '@/lib/spaces'

const SPACES: SpaceUrls = {
  auth: 'https://auth.agence.fr',
  team: 'https://team.agence.fr',
  admin: 'https://admin.agence.fr',
  client: 'https://client.agence.fr',
}

describe('spaceOfPath', () => {
  it('suit la même répartition que le serveur', () => {
    expect(spaceOfPath('/')).toBeNull()
    expect(spaceOfPath('/compte/securite')).toBe('team')
    expect(spaceOfPath('/login')).toBe('auth')
    expect(spaceOfPath('/invitation/abc')).toBe('auth')
    expect(spaceOfPath('/client/tickets')).toBe('client')
    expect(spaceOfPath('/clientele')).toBe('team')
    expect(spaceOfPath('/pm')).toBe('admin')
    expect(spaceOfPath('/parametres/comptes')).toBe('admin')
    expect(spaceOfPath('/pm/projets')).toBe('team')
    expect(spaceOfPath('/crm/clients')).toBe('team')
  })
})

describe('crossSpaceUrl', () => {
  it('ne bouge pas en mode mono-domaine', () => {
    expect(crossSpaceUrl('/client', '', undefined, null, 'https://piilot.fr')).toBeNull()
  })

  it('part vers le domaine de l’espace, avec la recherche', () => {
    expect(crossSpaceUrl('/parametres/comptes', '?x=1', undefined, SPACES, SPACES.team)).toBe(
      'https://admin.agence.fr/parametres/comptes?x=1',
    )
    expect(crossSpaceUrl('/login', '', undefined, SPACES, SPACES.client)).toBe('https://auth.agence.fr/login')
    expect(crossSpaceUrl('/compte', '', undefined, SPACES, SPACES.client)).toBe('https://team.agence.fr/compte')
  })

  it('sert le back-office sur team comme sur admin tant que le rôle est inconnu', () => {
    expect(crossSpaceUrl('/pm/projets', '', undefined, SPACES, SPACES.team)).toBeNull()
    expect(crossSpaceUrl('/pm/projets', '', undefined, SPACES, SPACES.admin)).toBeNull()
  })

  it('garde un administrateur sur son domaine, et l’équipe sur le sien', () => {
    expect(crossSpaceUrl('/pm/projets', '', 'admin', SPACES, SPACES.admin)).toBeNull()
    expect(crossSpaceUrl('/pm/projets', '', 'admin', SPACES, SPACES.team)).toBe('https://admin.agence.fr/pm/projets')
    expect(crossSpaceUrl('/pm/projets', '', 'admin', SPACES, SPACES.auth)).toBe('https://admin.agence.fr/pm/projets')
    expect(crossSpaceUrl('/pm/projets', '', 'team', SPACES, SPACES.admin)).toBe('https://team.agence.fr/pm/projets')
    expect(crossSpaceUrl('/parametres', '', 'team', SPACES, SPACES.team)).toBeNull()
    expect(crossSpaceUrl('/pm/projets', '', 'client', SPACES, SPACES.client)).toBeNull()
  })

  it('laisse tranquille un domaine inconnu', () => {
    expect(crossSpaceUrl('/client', '', undefined, SPACES, 'http://localhost:5173')).toBeNull()
    expect(currentSpace(SPACES, 'http://localhost:5173')).toBeNull()
  })
})

describe('readSpaces', () => {
  it('lit la balise posée par le serveur, et ignore une balise incomplète', () => {
    const doc = document.implementation.createHTMLDocument()
    expect(readSpaces(doc)).toBeNull()

    const meta = doc.createElement('meta')
    meta.name = 'piilot-spaces'
    meta.content = JSON.stringify(SPACES)
    doc.head.append(meta)
    expect(readSpaces(doc)).toEqual(SPACES)

    meta.content = JSON.stringify({ auth: SPACES.auth })
    expect(readSpaces(doc)).toBeNull()
  })
})
