import { queryOptions } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'

/** Resultats de la palette pour une saisie. Courte duree de vie : on retape. */
export function searchQuery(q: string) {
  return queryOptions({
    queryKey: ['search', q] as const,
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/search', { params: { query: { q } } })),
    staleTime: 15_000,
    placeholderData: (previous) => previous,
  })
}
