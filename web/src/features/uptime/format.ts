import type { StatusPill } from '@/features/projects/format'
import type { UptimeMonitor } from '@/types/api'

/** L'etat d'un site, dans les vignettes de l'application. */
export const MONITOR_STATUS: Record<UptimeMonitor['status'], { label: string; color: string; pill: StatusPill }> = {
  up: { label: 'En ligne', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } },
  down: { label: 'Hors ligne', color: '#e5484d', pill: { bg: '#fdf3f3', border: '#f2d5d6', text: '#e5484d' } },
  pending: { label: 'En attente', color: '#eab308', pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' } },
  maintenance: { label: 'Maintenance', color: '#4956f4', pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' } },
}

export const ENVIRONMENT = {
  prod: 'Production',
  preprod: 'Préproduction',
} as const

const PERCENT = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 2 })

/** « 99,95 % », ou un tiret sans controle concluant. */
export function formatUptime(value: number | null): string {
  return value === null ? '—' : `${PERCENT.format(value)} %`
}

/**
 * Couleur d'une case de la frise : vert franc a 100 %, plus terne en
 * dessous de 99,9, ambre sous 99, rouge sous 95. Gris sans controle.
 */
export function uptimeColor(value: number | null): string {
  if (value === null) return '#ebebeb'
  if (value >= 99.9) return '#0db471'
  if (value >= 99) return '#7fd3a8'
  if (value >= 95) return '#eab308'
  return '#e5484d'
}

/** Une duree en secondes, en mots courts : « 4 min », « 2 h 10 », « 3 j 4 h ». */
export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds} s`
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `${minutes} min`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} h${minutes % 60 > 0 ? ` ${String(minutes % 60).padStart(2, '0')}` : ''}`
  const days = Math.floor(hours / 24)
  return `${days} j${hours % 24 > 0 ? ` ${hours % 24} h` : ''}`
}
