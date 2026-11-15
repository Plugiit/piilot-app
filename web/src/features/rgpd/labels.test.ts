import { describe, expect, it } from 'vitest'

import {
  confirmMatches,
  ERASURE_STATUS_LABEL,
  ERASURE_STATUSES,
  erasureTotals,
  EXPORT_STATUS_LABEL,
  exportFailureLabel,
  graceLabel,
  isExportActive,
  plannedDate,
  rgpdErrorMessage,
} from './labels'

describe('statuts', () => {
  it('libelle chaque statut d’export et d’effacement', () => {
    expect(Object.keys(EXPORT_STATUS_LABEL).sort()).toEqual(['expired', 'failed', 'queued', 'ready', 'running'])
    expect(ERASURE_STATUSES.sort()).toEqual(['cancelled', 'done', 'failed', 'running', 'scheduled'])
    for (const status of ERASURE_STATUSES) expect(ERASURE_STATUS_LABEL[status]).not.toBe('')
  })

  it('sonde tant que l’archive se construit, pas après', () => {
    expect(isExportActive('queued')).toBe(true)
    expect(isExportActive('running')).toBe(true)
    expect(isExportActive('ready')).toBe(false)
    expect(isExportActive('failed')).toBe(false)
  })
})

describe('codes d’erreur', () => {
  it('traduit les codes connus et garde le message du serveur sinon', () => {
    expect(rgpdErrorMessage('LAST_ADMIN', 'x')).toContain('dernier administrateur')
    expect(rgpdErrorMessage('CONFLICT', 'x')).toContain('déjà en cours')
    expect(rgpdErrorMessage('AUTRE', 'message serveur')).toBe('message serveur')
  })

  it('explique les échecs d’export', () => {
    expect(exportFailureLabel('too_large')).toContain('taille maximale')
    expect(exportFailureLabel('interrupted')).toContain('redémarré')
    expect(exportFailureLabel('???')).toBe('L’export a échoué.')
  })
})

describe('date d’effacement', () => {
  it('ajoute le délai à la demande', () => {
    const now = new Date('2026-11-15T10:00:00Z')
    expect(plannedDate(now, 720).toISOString()).toBe('2026-12-15T10:00:00.000Z')
  })

  it('dit le délai en jours quand il tombe juste', () => {
    expect(graceLabel(720)).toBe('30 jours')
    expect(graceLabel(24)).toBe('1 jour')
    expect(graceLabel(36)).toBe('36 heures')
    expect(graceLabel(1)).toBe('1 heure')
  })
})

describe('confirmMatches', () => {
  it('accepte le nom retapé, sans égard à la casse, aux espaces ni aux accents composés', () => {
    expect(confirmMatches('  acme   corp ', 'Acme Corp')).toBe(true)
    expect(confirmMatches('Éloïse Martin', 'Éloïse Martin')).toBe(true)
  })

  it('refuse une saisie différente ou vide', () => {
    expect(confirmMatches('Acme', 'Acme Corp')).toBe(false)
    expect(confirmMatches('', '')).toBe(false)
    expect(confirmMatches('   ', '   ')).toBe(false)
  })
})

describe('erasureTotals', () => {
  it('additionne les lignes et lit les fichiers', () => {
    expect(erasureTotals({ counts: { projects: 2, tasks: 10 }, files: 3 })).toEqual({ rows: 12, files: 3 })
  })

  it('rend null sans compteurs', () => {
    expect(erasureTotals({})).toBeNull()
  })
})
