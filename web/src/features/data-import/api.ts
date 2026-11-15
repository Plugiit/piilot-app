import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, apiUrl, postFile, unwrap } from '@/lib/api'
import type { ImportDetail, ImportItem } from '@/types/api'

import { isActive, type DataEntity, type RowOutcome } from './entities'

/** Intervalle de sondage tant que le serveur traite l'import. */
const POLL_MS = 1500

export const importKeys = {
  all: ['imports'] as const,
  list: (page: number) => ['imports', 'list', page] as const,
  one: (id: string) => ['imports', 'one', id] as const,
  rows: (id: string, outcome: RowOutcome | undefined, page: number) => ['imports', 'rows', id, outcome ?? 'all', page] as const,
}

export function importsQuery(page: number) {
  return queryOptions({
    queryKey: importKeys.list(page),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/imports', { params: { query: { page } } })),
    placeholderData: keepPreviousData,
    // Un import lancé depuis un autre onglet doit finir par passer à « Terminé ».
    refetchInterval: (query) => (query.state.data?.items.some((i) => isActive(i.status)) ? POLL_MS : false),
  })
}

/** Un import. Se réinterroge seul tant qu'il est en file ou en cours. */
export function importQuery(id: string) {
  return queryOptions({
    queryKey: importKeys.one(id),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/imports/{id}', { params: { path: { id } } })),
    refetchInterval: (query) => {
      const status = query.state.data?.status

      return status !== undefined && isActive(status) ? POLL_MS : false
    },
  })
}

export function importRowsQuery(id: string, outcome: RowOutcome | undefined, page: number) {
  return queryOptions({
    queryKey: importKeys.rows(id, outcome, page),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/imports/{id}/rows', { params: { path: { id }, query: { outcome, page } } })),
    placeholderData: keepPreviousData,
  })
}

export function useUploadImport() {
  const queryClient = useQueryClient()

  return useMutation({
    // Multipart : le client typé sérialise en JSON, d'où `postFile`.
    mutationFn: ({ entity, file }: { entity: DataEntity; file: File }) =>
      postFile<ImportDetail>('/api/v1/admin/imports', 'file', file, { entity }),
    onSuccess: (detail) => {
      queryClient.setQueryData(importKeys.one(detail.id), detail)
      void queryClient.invalidateQueries({ queryKey: ['imports', 'list'] })
    },
  })
}

export function useRunImport(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (input: { mapping: Record<string, string>; dryRun: boolean }) =>
      unwrap(
        await api.POST('/api/v1/admin/imports/{id}/run', {
          params: { path: { id } },
          body: { mapping: input.mapping, dry_run: input.dryRun },
        }),
      ),
    onSuccess: (item: ImportItem) => {
      // Le détail garde ses champs et son aperçu : on ne remplace que l'état.
      queryClient.setQueryData<ImportDetail>(importKeys.one(id), (old) => (old === undefined ? old : { ...old, ...item }))
      void queryClient.invalidateQueries({ queryKey: ['imports', 'rows', id] })
      void queryClient.invalidateQueries({ queryKey: ['imports', 'list'] })
    },
  })
}

export function useDeleteImport() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) =>
      unwrap(await api.DELETE('/api/v1/admin/imports/{id}', { params: { path: { id } } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: importKeys.all })
    },
  })
}

/** Adresse du CSV d'une entité : un lien, le navigateur télécharge. */
export function exportUrl(entity: DataEntity): string {
  return apiUrl(`/api/v1/admin/exports/${entity}`)
}
