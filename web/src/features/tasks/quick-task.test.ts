import { describe, expect, it } from 'vitest'

import { parseQuickTask } from './quick-task'

// Un mardi.
const today = new Date(2026, 9, 6)

describe('parseQuickTask', () => {
  it('garde un libelle ordinaire tel quel', () => {
    expect(parseQuickTask('Relancer le client', today)).toEqual({ title: 'Relancer le client' })
  })

  it('lit la priorite et la retire du libelle', () => {
    expect(parseQuickTask('Corriger le menu !haute', today)).toEqual({ title: 'Corriger le menu', priority: 'high' })
    expect(parseQuickTask('!basse Ranger les maquettes', today)).toEqual({ title: 'Ranger les maquettes', priority: 'low' })
  })

  it('lit les dates relatives', () => {
    expect(parseQuickTask('Appeler demain', today).dueOn).toBe('2026-10-07')
    expect(parseQuickTask('Point lundi', today).dueOn).toBe('2026-10-12')
    // Le jour meme vise la semaine suivante.
    expect(parseQuickTask('Point mardi', today).dueOn).toBe('2026-10-13')
  })

  it('lit une date jj/mm, et bascule a l’annee suivante si elle est passee', () => {
    expect(parseQuickTask('Livrer 12/10', today).dueOn).toBe('2026-10-12')
    expect(parseQuickTask('Livrer 03/02', today).dueOn).toBe('2027-02-03')
    expect(parseQuickTask('Livrer 03/02/2026', today).dueOn).toBe('2026-02-03')
    expect(parseQuickTask('Livrer 31/02', today)).toEqual({ title: 'Livrer 31/02' })
  })

  it('combine tout', () => {
    expect(parseQuickTask('Recette vendredi !haute', today)).toEqual({
      title: 'Recette',
      priority: 'high',
      dueOn: '2026-10-09',
    })
  })
})
