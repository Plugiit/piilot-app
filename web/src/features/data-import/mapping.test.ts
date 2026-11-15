import { describe, expect, it } from 'vitest'

import type { DataField } from '@/types/api'

import {
  ENTITIES,
  FAILURE_LABEL,
  IMPORT_ORDER,
  OUTCOME_LABEL,
  REASON_LABEL,
  STATUS_LABEL,
  failureLabel,
  isActive,
  reasonLabel,
} from './entities'
import { cleanMapping, missingRequired, sharedHeaders, unusedHeaders } from './mapping'

const fields: DataField[] = [
  { key: 'name', header: 'Nom', aliases: [], required: true },
  { key: 'siret', header: 'SIRET', aliases: [], required: false },
  { key: 'email', header: 'E-mail', aliases: [], required: true },
]

describe('cleanMapping', () => {
  it('retire les champs vides et les colonnes absentes du fichier', () => {
    expect(cleanMapping({ name: 'Société', siret: '', email: 'Ancienne colonne' }, ['Société', 'Ville'])).toEqual({
      name: 'Société',
    })
  })
})

describe('missingRequired', () => {
  it('liste les champs obligatoires sans colonne', () => {
    expect(missingRequired(fields, { name: 'Société', siret: '' }).map((f) => f.key)).toEqual(['email'])
  })

  it('ne réclame rien quand tout est relié', () => {
    expect(missingRequired(fields, { name: 'a', email: 'b' })).toEqual([])
  })
})

describe('sharedHeaders et unusedHeaders', () => {
  it('repère une colonne lue par deux champs', () => {
    expect(sharedHeaders({ name: 'A', siret: 'A', email: 'B', other: '' })).toEqual(['A'])
  })

  it('repère les colonnes ignorées', () => {
    expect(unusedHeaders(['A', 'B', 'C'], { name: 'A', email: 'C' })).toEqual(['B'])
  })
})

describe('libellés', () => {
  it('traduit tous les codes de motif renvoyés par l’API', () => {
    const codes = [
      'invalid', 'already_exists', 'already_imported', 'duplicate_siret', 'duplicate_name',
      'duplicate_email', 'unknown_client', 'unknown_project', 'unknown_user',
    ]
    for (const code of codes) expect(REASON_LABEL[code], code).toBeTruthy()
  })

  it('rend un motif inconnu tel quel et une ligne créée par un tiret', () => {
    expect(reasonLabel('nouveau_code')).toBe('nouveau_code')
    expect(reasonLabel('')).toBe('—')
  })

  it('traduit chaque statut, chaque issue et chaque échec', () => {
    expect(Object.keys(STATUS_LABEL)).toHaveLength(6)
    expect(Object.keys(OUTCOME_LABEL)).toEqual(['created', 'skipped', 'error'])
    for (const code of ['interrupted', 'file_unreadable', 'internal']) expect(FAILURE_LABEL[code]).toBeTruthy()
    expect(failureLabel('inconnu')).toBe('L’import a échoué.')
  })

  it('range les entités dans l’ordre d’import recommandé', () => {
    expect(ENTITIES.map((e) => e.value)).toEqual(['clients', 'contacts', 'projects', 'time_entries'])
    expect(IMPORT_ORDER).toBe('clients → contacts → projets → temps')
  })

  it('ne sonde l’API que pendant le traitement', () => {
    expect(isActive('queued')).toBe(true)
    expect(isActive('running')).toBe(true)
    expect(isActive('done')).toBe(false)
    expect(isActive('uploaded')).toBe(false)
  })
})
