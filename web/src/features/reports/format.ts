import type { StatusPill } from '@/features/projects/format'
import type { ReportStatus } from '@/features/reports/api'

export const REPORT_STATUS: Record<ReportStatus, { label: string; color: string; pill: StatusPill }> = {
  draft: { label: 'Brouillon', color: '#8a8c91', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#4b4b4f' } },
  validated: { label: 'Validé', color: '#4956f4', pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' } },
  sent: { label: 'Envoyé', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } },
}
