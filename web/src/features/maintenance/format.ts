import type { StatusPill } from '@/features/projects/format'
import type { InterventionKind, MaintenancePlan } from '@/types/api'

/** Les natures d'intervention, dans les vignettes de l'application. */
export const INTERVENTION_KIND: Record<InterventionKind, { label: string; color: string; pill: StatusPill }> = {
  contenu: { label: 'Contenu', color: '#4956f4', pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' } },
  correctif: { label: 'Correctif', color: '#e5484d', pill: { bg: '#fdf3f3', border: '#f2d5d6', text: '#c4323a' } },
  mise_a_jour: { label: 'Mise à jour', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } },
  support: { label: 'Support', color: '#eab308', pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' } },
  autre: { label: 'Autre', color: '#8a8c91', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#4b4b4f' } },
}

export const INTERVENTION_KINDS = Object.keys(INTERVENTION_KIND) as InterventionKind[]

type Section = MaintenancePlan['sections'][number]

/** Les sections du rapport ; celles de la 1.6 n'ont pas encore de donnees. */
export const REPORT_SECTIONS: { key: Section; label: string; soon?: boolean }[] = [
  { key: 'availability', label: 'Disponibilité' },
  { key: 'audience', label: 'Audience' },
  { key: 'interventions', label: 'Interventions' },
  { key: 'deployments', label: 'Mises en ligne' },
  { key: 'updates', label: 'Mises à jour', soon: true },
  { key: 'backups', label: 'Sauvegardes', soon: true },
  { key: 'domain', label: 'Domaine', soon: true },
]

const MONTH = new Intl.DateTimeFormat('fr-FR', { month: 'long', year: 'numeric' })

/** « octobre 2026 », depuis « 2026-10 ». */
export function monthLabel(month: string): string {
  const [y, m] = month.split('-').map(Number)
  const label = MONTH.format(new Date(y ?? 1970, (m ?? 1) - 1, 1))
  return label.charAt(0).toUpperCase() + label.slice(1)
}

/** Le mois d'une date locale, « 2026-10 ». */
export function monthOf(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`
}

/** Le mois voisin. */
export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number)
  return monthOf(new Date(y ?? 1970, (m ?? 1) - 1 + delta, 1))
}
