import type { StatisticsTotals } from '@/types/api'

/**
 * Les mots et les nombres de la page « Statistiques ». Umami rend des codes
 * (FR, desktop, edge-chromium) : on les dit en francais.
 */

const NUMBER = new Intl.NumberFormat('fr-FR')
const PERCENT = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 1 })

export function formatNumber(n: number): string {
  return NUMBER.format(n)
}

/** « 2 min 05 s », « 48 s ». */
export function formatSeconds(seconds: number): string {
  const s = Math.round(seconds)
  if (s < 60) return `${s} s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m} min ${String(s % 60).padStart(2, '0')} s`
  return `${Math.floor(m / 60)} h ${String(m % 60).padStart(2, '0')}`
}

/** Les chiffres derives d'une periode : taux de rebond et duree moyenne. */
export function derived(t: StatisticsTotals) {
  return {
    bounceRate: t.visits === 0 ? null : (Math.min(t.bounces, t.visits) / t.visits) * 100,
    avgDuration: t.visits === 0 ? null : t.total_time / t.visits,
  }
}

/** Variation en %, nulle sans base de comparaison. */
export function variation(current: number | null, previous: number | null): number | null {
  if (current === null || previous === null || previous === 0) return null
  return ((current - previous) / previous) * 100
}

export function formatVariation(v: number): string {
  return `${v > 0 ? '+' : ''}${PERCENT.format(v)} %`
}

export function formatPercent(v: number): string {
  return `${PERCENT.format(v)} %`
}

const REGIONS = (() => {
  try {
    return new Intl.DisplayNames(['fr'], { type: 'region' })
  } catch {
    return null
  }
})()

/** « 🇫🇷 France », depuis le code ISO. */
export function countryLabel(code: string): { flag: string; name: string } {
  const upper = code.toUpperCase()
  if (!/^[A-Z]{2}$/.test(upper)) return { flag: '🌐', name: code === '' ? 'Inconnu' : code }
  const flag = String.fromCodePoint(...[...upper].map((c) => 0x1f1e6 + c.charCodeAt(0) - 65))
  let name = upper
  try {
    name = REGIONS?.of(upper) ?? upper
  } catch {
    // Un code que le navigateur ne connait pas reste un code.
  }
  return { flag, name }
}

const DEVICES: Record<string, string> = { desktop: 'Ordinateur', laptop: 'Ordinateur portable', mobile: 'Mobile', tablet: 'Tablette' }

export function deviceLabel(v: string): string {
  return DEVICES[v] ?? (v === '' ? 'Inconnu' : v)
}

const BROWSERS: Record<string, string> = {
  chrome: 'Chrome', crios: 'Chrome (iOS)', firefox: 'Firefox', fxios: 'Firefox (iOS)', safari: 'Safari', ios: 'Safari (iOS)',
  'ios-webview': 'Navigateur intégré (iOS)', 'edge-chromium': 'Edge', edge: 'Edge', 'edge-ios': 'Edge (iOS)', opera: 'Opera',
  samsung: 'Samsung Internet', 'chromium-webview': 'Navigateur intégré (Android)', brave: 'Brave', yandexbrowser: 'Yandex',
  facebook: 'Facebook', instagram: 'Instagram',
}

export function browserLabel(v: string): string {
  return BROWSERS[v] ?? (v === '' ? 'Inconnu' : v.charAt(0).toUpperCase() + v.slice(1))
}

export function referrerLabel(v: string): string {
  return v === '' ? 'Accès direct' : v.replace(/^www\./, '')
}

export function plainLabel(v: string): string {
  return v === '' ? 'Inconnu' : v
}

/** Un pas « 2026-10-12T14:00:00 », lu comme une heure locale. */
export function localDate(at: string): Date {
  const [d, t = '00:00:00'] = at.split('T')
  const [y, m, day] = (d ?? '').split('-').map(Number)
  const [h, min] = t.split(':').map(Number)
  return new Date(y ?? 1970, (m ?? 1) - 1, day ?? 1, h ?? 0, min ?? 0)
}
