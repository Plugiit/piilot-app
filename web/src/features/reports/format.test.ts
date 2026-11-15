import { describe, expect, it } from 'vitest'

import { deploymentsSourceLabel, HOLD_REASON_LABEL, holdReasonInMaintenance, holdReasonLabel } from './format'

describe('motifs de retenue', () => {
  it('dit en clair les motifs Coolify et Git', () => {
    expect(holdReasonLabel('deployments_stale')).toContain('Coolify')
    expect(holdReasonLabel('git_sync_stale')).toContain('GitHub')
    expect(holdReasonLabel('git_sync_stale')).toContain('webhook')
  })

  it('garde le code d’un motif inconnu plutôt que de le taire', () => {
    expect(holdReasonLabel('motif_futur')).toBe('motif_futur')
  })

  it('n’envoie vers l’onglet Maintenance que les motifs qui s’y règlent', () => {
    expect(holdReasonInMaintenance('deployments_stale')).toBe(true)
    expect(holdReasonInMaintenance('git_sync_stale')).toBe(true)
    expect(holdReasonInMaintenance('incident')).toBe(false)
    expect(holdReasonInMaintenance('no_recipients')).toBe(false)
  })

  it('a un libellé pour chaque motif que le serveur peut poser', () => {
    for (const code of ['no_recipients', 'availability_missing', 'incident', 'backup_missing', 'backup_stale', 'domain_missing', 'domain_expiring', 'updates_missing', 'update_failed', 'deployments_stale', 'git_sync_stale']) {
      expect(HOLD_REASON_LABEL[code]).toBeDefined()
    }
  })
})

describe('source des mises en ligne', () => {
  it('dit Coolify ou le dépôt, et se tait sans source', () => {
    expect(deploymentsSourceLabel('coolify')).toBe('d’après Coolify')
    expect(deploymentsSourceLabel('webhook')).toBe('d’après le dépôt')
    expect(deploymentsSourceLabel(undefined)).toBeNull()
    expect(deploymentsSourceLabel('')).toBeNull()
  })
})
