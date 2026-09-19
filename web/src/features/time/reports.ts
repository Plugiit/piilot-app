import { keepPreviousData, queryOptions, useMutation } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'

/** Filtres communs au rapport, au detail et a l'export. */
export interface ReportFilters {
  from: string
  to: string
  project_id?: string
  user_id?: string
  service_id?: string
  client_id?: string
  /** true : projets clients ; false : projets internes ; absent : tout. */
  billable?: boolean
}

export type ReportGroupBy = 'project' | 'user' | 'service' | 'client'

export const reportKeys = {
  all: ['time-reports'] as const,
  report: (f: ReportFilters, by: ReportGroupBy, page: number) =>
    [...reportKeys.all, 'report', f, by, page] as const,
  entries: (f: ReportFilters, page: number) => [...reportKeys.all, 'entries', f, page] as const,
}

/** Tableau regroupe : dix postes par page, les plus gros en tete. */
export const GROUP_PAGE_SIZE = 10
/** Detail des saisies. */
export const ENTRY_PAGE_SIZE = 25

/**
 * Totaux, evolution et tableau regroupe.
 *
 * `keepPreviousData` : changer de filtre ou de page garde l'ecran affiche
 * pendant le chargement, au lieu de le vider puis de le remplir.
 */
export function timeReportQuery(f: ReportFilters, groupBy: ReportGroupBy, page: number) {
  return queryOptions({
    queryKey: reportKeys.report(f, groupBy, page),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/time-reports', {
          params: {
            query: {
              ...f,
              group_by: groupBy,
              page,
              page_size: GROUP_PAGE_SIZE,
            },
          },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

export function timeReportEntriesQuery(f: ReportFilters, page: number) {
  return queryOptions({
    queryKey: reportKeys.entries(f, page),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/time-reports/entries', {
          params: { query: { ...f, page, page_size: ENTRY_PAGE_SIZE } },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/**
 * Export CSV.
 *
 * Telecharge par le client HTTP plutot que par un lien : une session expiree
 * remonte alors comme toute autre erreur (et propose de se reconnecter), au
 * lieu d'ouvrir un onglet sur un message JSON.
 */
export function useExportTimeReport() {
  return useMutation({
    mutationFn: async (f: ReportFilters) => {
      const blob = unwrap(
        await api.GET('/api/v1/admin/time-reports/export', {
          params: { query: f },
          parseAs: 'blob',
        }),
      ) as unknown as Blob

      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `piilot-temps_${f.from}_${f.to}.csv`
      link.click()
      URL.revokeObjectURL(url)
    },
  })
}
