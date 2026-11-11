import { describe, expect, it } from 'vitest'

import { beforeSend, readTracker, sanitizeUrl, startTracker } from '@/lib/tracker'

const CONFIG = { src: 'https://umami.agence.fr/script.js', websiteId: '7b376ab6-cf03-4a00-9831-c5ea22aa570e' }

function docWithMeta(content: string | null): Document {
  const doc = document.implementation.createHTMLDocument()
  if (content !== null) {
    const meta = doc.createElement('meta')
    meta.name = 'piilot-tracker'
    meta.content = content
    doc.head.append(meta)
  }
  return doc
}

describe('sanitizeUrl', () => {
  it('retire les jetons du chemin', () => {
    expect(sanitizeUrl('https://piilot.agence.fr/reinitialiser/q3J9xK_2mZ-aPw8LrT5vYb1cN0dE4fG6hH7iI8jJ9kK')).toBe('https://piilot.agence.fr/reinitialiser/:token')
    expect(sanitizeUrl('https://piilot.agence.fr/invitation/eyJ1IjoiYSJ9.c2lnbmF0dXJl')).toBe('https://piilot.agence.fr/invitation/:token')
  })

  it('retire query et fragment, donc les e-mails et le jeton des livrables', () => {
    expect(sanitizeUrl('https://piilot.agence.fr/login?email=jane%40example.com&redirect=%2Fpm')).toBe('https://piilot.agence.fr/login')
    expect(sanitizeUrl('https://piilot.agence.fr/client/livrables/7b376ab6-cf03-4a00-9831-c5ea22aa570e/repondre?token=abc#x')).toBe(
      'https://piilot.agence.fr/client/livrables/:id/repondre',
    )
  })

  it('garde les routes fixes intactes', () => {
    expect(sanitizeUrl('https://piilot.agence.fr/mot-de-passe-oublie')).toBe('https://piilot.agence.fr/mot-de-passe-oublie')
    expect(sanitizeUrl('https://piilot.agence.fr/')).toBe('https://piilot.agence.fr/')
  })

  it("vide une adresse illisible plutot que de l'envoyer", () => {
    expect(sanitizeUrl('')).toBe('')
    expect(sanitizeUrl('pas une adresse')).toBe('')
  })
})

describe('beforeSend', () => {
  it("filtre l'adresse et la page precedente", () => {
    const sent = beforeSend('event', {
      website: CONFIG.websiteId,
      url: 'https://piilot.agence.fr/reinitialiser/q3J9xK_2mZ-aPw8LrT5vYb1cN0dE4fG6hH7iI8jJ9kK',
      referrer: 'https://piilot.agence.fr/invitation/q3J9xK_2mZ-aPw8LrT5vYb1cN0dE4fG6hH7iI8jJ9kK?x=1',
    })
    expect(sent).toEqual({
      website: CONFIG.websiteId,
      url: 'https://piilot.agence.fr/reinitialiser/:token',
      referrer: 'https://piilot.agence.fr/invitation/:token',
    })
  })

  it('annule un envoi sans adresse', () => {
    expect(beforeSend('event', { website: CONFIG.websiteId })).toBeNull()
  })
})

describe('readTracker', () => {
  it('lit la balise posee par le serveur', () => {
    expect(readTracker(docWithMeta(null))).toBeNull()
    expect(readTracker(docWithMeta(JSON.stringify(CONFIG)))).toEqual(CONFIG)
  })

  it('ignore une balise incomplete ou douteuse', () => {
    expect(readTracker(docWithMeta('{'))).toBeNull()
    expect(readTracker(docWithMeta(JSON.stringify({ src: CONFIG.src })))).toBeNull()
    expect(readTracker(docWithMeta(JSON.stringify({ ...CONFIG, src: 'http://umami.agence.fr/script.js' })))).toBeNull()
    expect(readTracker(docWithMeta(JSON.stringify({ ...CONFIG, websiteId: 'x' })))).toBeNull()
  })
})

describe('startTracker', () => {
  it('ne charge rien sans configuration', () => {
    const doc = docWithMeta(null)
    const win = {} as Window
    startTracker(doc, win)
    expect(doc.querySelector('script')).toBeNull()
    expect(win.piilotTrackerBeforeSend).toBeUndefined()
  })

  it('pose le filtre puis charge le script', () => {
    const doc = docWithMeta(JSON.stringify(CONFIG))
    const win = {} as Window
    startTracker(doc, win)

    expect(win.piilotTrackerBeforeSend).toBe(beforeSend)
    const script = doc.querySelector('script')
    expect(script?.getAttribute('src')).toBe(CONFIG.src)
    expect(script?.getAttribute('data-website-id')).toBe(CONFIG.websiteId)
    expect(script?.getAttribute('data-before-send')).toBe('piilotTrackerBeforeSend')
    expect(script?.getAttribute('data-exclude-search')).toBe('true')
    expect(script?.getAttribute('data-exclude-hash')).toBe('true')
  })
})
