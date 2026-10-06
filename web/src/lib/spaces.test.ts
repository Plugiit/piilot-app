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
    expect(spaceOfPath('/compte/securite')).toBeNull()
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
    expect(crossSpaceUrl('/client', '', null, 'https://piilot.fr')).toBeNull()
  })

  it('part vers le domaine de l’espace, avec la recherche', () => {
    expect(crossSpaceUrl('/parametres/comptes', '?x=1', SPACES, SPACES.team)).toBe(
      'https://admin.agence.fr/parametres/comptes?x=1',
    )
    expect(crossSpaceUrl('/login', '', SPACES, SPACES.client)).toBe('https://auth.agence.fr/login')
    expect(crossSpaceUrl('/pm/projets', '', SPACES, SPACES.team)).toBeNull()
    expect(crossSpaceUrl('/compte', '', SPACES, SPACES.admin)).toBeNull()
    expect(crossSpaceUrl('/compte', '', SPACES, SPACES.client)).toBe('https://team.agence.fr/compte')
  })

  it('laisse tranquille un domaine inconnu', () => {
    expect(crossSpaceUrl('/client', '', SPACES, 'http://localhost:5173')).toBeNull()
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
