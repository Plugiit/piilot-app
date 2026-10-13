import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { Report, ReportRecipients } from '@/types/api'

export const reportKeys = {
  all: ['reports'] as const,
  list: (month: string, status: string | null, page: number) => ['reports', 'list', month, status, page] as const,
  detail: (id: string) => ['reports', 'detail', id] as const,
  settings: ['reports', 'settings'] as const,
  recipients: (projectId: string) => ['reports', 'recipients', projectId] as const,
  portalList: (projectId: string) => ['reports', 'portal', projectId] as const,
  portal: (id: string) => ['reports', 'portal-report', id] as const,
}

export type ReportStatus = 'draft' | 'validated' | 'sent'

export function reportListQuery(month: string, status: ReportStatus | null, page: number) {
  return queryOptions({
    queryKey: reportKeys.list(month, status, page),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/reports', { params: { query: { month, page, ...(status !== null ? { status } : {}) } } })),
    placeholderData: keepPreviousData,
  })
}

export function reportQuery(id: string) {
  return queryOptions({
    queryKey: reportKeys.detail(id),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/reports/{id}', { params: { path: { id } } })),
  })
}

export const reportSettingsQuery = queryOptions({
  queryKey: reportKeys.settings,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/reports/settings')),
})

export function useUpdateReportSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: { auto_send: boolean; send_day: number; wait_days: number }) =>
      unwrap(await api.PUT('/api/v1/admin/reports/settings', { body })),
    onSuccess: (settings) => queryClient.setQueryData(reportKeys.settings, settings),
  })
}

export function useGenerateReports() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (month: string) => unwrap(await api.POST('/api/v1/admin/reports/generate', { body: { month } })),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: reportKeys.all }),
  })
}

/** Les gestes sur un rapport : chacun rend le rapport a jour. */
export function useReportAction(id: string) {
  const queryClient = useQueryClient()
  const set = (report: Report) => {
    queryClient.setQueryData(reportKeys.detail(id), report)
    void queryClient.invalidateQueries({ queryKey: ['reports', 'list'] })
  }
  const path = { params: { path: { id } } }
  return {
    editorial: useMutation({
      mutationFn: async (editorial: string) => unwrap(await api.PUT('/api/v1/admin/reports/{id}/editorial', { ...path, body: { editorial } })),
      onSuccess: set,
    }),
    regenerate: useMutation({ mutationFn: async () => unwrap(await api.POST('/api/v1/admin/reports/{id}/regenerate', path)), onSuccess: set }),
    validate: useMutation({ mutationFn: async () => unwrap(await api.POST('/api/v1/admin/reports/{id}/validate', path)), onSuccess: set }),
    unvalidate: useMutation({ mutationFn: async () => unwrap(await api.POST('/api/v1/admin/reports/{id}/unvalidate', path)), onSuccess: set }),
    send: useMutation({ mutationFn: async () => unwrap(await api.POST('/api/v1/admin/reports/{id}/send', path)), onSuccess: set }),
  }
}

export function reportRecipientsQuery(projectId: string) {
  return queryOptions({
    queryKey: reportKeys.recipients(projectId),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/projects/{id}/report-recipients', { params: { path: { id: projectId } } })),
  })
}

export function useSetReportRecipients(projectId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: { excluded: string[]; extra: string[] }) =>
      unwrap(await api.PUT('/api/v1/admin/projects/{id}/report-recipients', { params: { path: { id: projectId } }, body })),
    onSuccess: (view: ReportRecipients) => {
      queryClient.setQueryData(reportKeys.recipients(projectId), view)
      void queryClient.invalidateQueries({ queryKey: ['reports', 'detail'] })
    },
  })
}

export function portalReportsQuery(projectId: string) {
  return queryOptions({
    queryKey: reportKeys.portalList(projectId),
    queryFn: async () => unwrap(await api.GET('/api/v1/client/projects/{id}/reports', { params: { path: { id: projectId } } })),
  })
}

export function portalReportQuery(id: string) {
  return queryOptions({
    queryKey: reportKeys.portal(id),
    queryFn: async () => unwrap(await api.GET('/api/v1/client/reports/{id}', { params: { path: { id } } })),
  })
}
