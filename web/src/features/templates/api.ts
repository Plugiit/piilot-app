import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { TemplateMilestone, TemplateTask } from '@/types/api'

export const templateKeys = {
  all: ['project-templates'] as const,
  list: (page: number) => [...templateKeys.all, 'list', page] as const,
  detail: (id: string) => [...templateKeys.all, 'detail', id] as const,
}

export const TEMPLATES_PAGE_SIZE = 50

/** Les modeles, par ordre alphabetique. */
export function templateListQuery(page = 1) {
  return queryOptions({
    queryKey: templateKeys.list(page),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/project-templates', {
          params: { query: { page, page_size: TEMPLATES_PAGE_SIZE } },
        }),
      ),
    placeholderData: keepPreviousData,
  })
}

/** Un modele et son contenu. */
export function templateQuery(id: string) {
  return queryOptions({
    queryKey: templateKeys.detail(id),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/project-templates/{id}', { params: { path: { id } } })),
  })
}

/** Un modele entier, tel que l'editeur l'enregistre. */
export interface TemplateValues {
  name: string
  description: string
  service_ids: string[]
  milestones: TemplateMilestone[]
  tasks: TemplateTask[]
}

export function useSaveTemplate(id: string | null) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: TemplateValues) =>
      id === null
        ? unwrap(await api.POST('/api/v1/admin/project-templates', { body: values }))
        : unwrap(
            await api.PUT('/api/v1/admin/project-templates/{id}', { params: { path: { id } }, body: values }),
          ),
    onSuccess: (template) => {
      queryClient.setQueryData(templateKeys.detail(template.id), template)
      void queryClient.invalidateQueries({ queryKey: [...templateKeys.all, 'list'] })
    },
  })
}

export function useDeleteTemplate(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () =>
      unwrap(await api.DELETE('/api/v1/admin/project-templates/{id}', { params: { path: { id } } })),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: templateKeys.detail(id) })
      void queryClient.invalidateQueries({ queryKey: [...templateKeys.all, 'list'] })
    },
  })
}
