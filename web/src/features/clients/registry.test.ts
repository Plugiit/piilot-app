import { describe, expect, it } from 'vitest'

import { isValidSiret, legalFormLabel, normalizeSiret, sameLegalName, vatOfSiren } from './registry'

describe('registre', () => {
  it('verifie la cle d’un SIRET', () => {
    expect(isValidSiret('55203253400646')).toBe(true)
    expect(isValidSiret('55203253400647')).toBe(false)
    expect(isValidSiret('35600000049837')).toBe(true)
    expect(isValidSiret(normalizeSiret('552 032 534 00646'))).toBe(true)
  })

  it('deduit le numero de TVA du SIREN', () => {
    expect(vatOfSiren('552032534')).toBe('FR27552032534')
  })

  it('compare les raisons sociales sans la casse ni la ponctuation', () => {
    expect(sameLegalName('Maison-Aubert', 'MAISON AUBERT')).toBe(true)
    expect(sameLegalName('Éclat', 'ECLAT')).toBe(true)
    expect(sameLegalName('Aubert', 'Aubert & fils')).toBe(false)
    expect(sameLegalName('', '')).toBe(false)
  })

  it('nomme la forme juridique, ou sa famille', () => {
    expect(legalFormLabel('5710')).toBe('Société par actions simplifiée (SAS)')
    expect(legalFormLabel('5499')).toBe('Société à responsabilité limitée (SARL)')
    expect(legalFormLabel('5460')).toBe('Société à responsabilité limitée (SARL)')
    expect(legalFormLabel('')).toBe('')
  })
})
