import type { StatusPill } from '@/features/projects/format'
import type { Milestone } from '@/types/api'

/** Etats d'un jalon : libelle, couleur pleine, palette de vignette. */
export const MILESTONE_STATE: Record<Milestone['state'], { label: string; color: string; pill: StatusPill }> = {
  upcoming: {
    label: 'À venir',
    color: '#4956f4',
    pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' },
  },
  late: {
    label: 'En retard',
    color: '#e5484d',
    pill: { bg: '#ffe8ec', border: '#ffd0d8', text: '#a30f2c' },
  },
  done: {
    label: 'Atteint',
    color: '#0db471',
    pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' },
  },
}
