import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { deliverableKeys } from '@/features/deliverables/api'
import { api, unwrap } from '@/lib/api'

export const milestoneKeys = {
  all: ['milestones'] as const,
  project: (projectId: string) => [...milestoneKeys.all, 'project', projectId] as const,
  planning: (from: string, to: string, scope: 'all' | 'mine') =>
    [...milestoneKeys.all, 'planning', from, to, scope] as const,
}

/** Les jalons d'un projet, avec leurs livrables. */
export function milestoneListQuery(projectId: string) {
  return queryOptions({
    queryKey: milestoneKeys.project(projectId),
    queryFn: async () =>
      unwrap(
        await api.GET('/api/v1/admin/projects/{id}/milestones', { params: { path: { id: projectId } } }),
      ),
  })
}

/** Le calendrier d'une periode. */
export function planningQuery(from: string, to: string, scope: 'all' | 'mine') {
  return queryOptions({
    queryKey: milestoneKeys.planning(from, to, scope),
    queryFn: async () =>
      unwrap(await api.GET('/api/v1/admin/planning', { params: { query: { from, to, scope } } })),
  })
}

export interface MilestoneValues {
  title?: string
  description?: string
  due_on?: string | null
  done?: boolean
}

/**
 * Un jalon change le planning et l'en-tete de ses livrables : tout ce qui
 * porte la cle des jalons se relit, et les livrables avec.
 */
function useInvalidate() {
  const queryClient = useQueryClient()

  return () => {
    void queryClient.invalidateQueries({ queryKey: milestoneKeys.all })
    void queryClient.invalidateQueries({ queryKey: deliverableKeys.all })
  }
}

export function useCreateMilestone(projectId: string) {
  const invalidate = useInvalidate()

  return useMutation({
    mutationFn: async (values: MilestoneValues) =>
      unwrap(
        await api.POST('/api/v1/admin/projects/{id}/milestones', {
          params: { path: { id: projectId } },
          body: values,
        }),
      ),
    onSuccess: invalidate,
  })
}

export function useUpdateMilestone(id: string) {
  const invalidate = useInvalidate()

  return useMutation({
    mutationFn: async (values: MilestoneValues) =>
      unwrap(
        await api.PATCH('/api/v1/admin/milestones/{id}', { params: { path: { id } }, body: values }),
      ),
    onSuccess: invalidate,
  })
}

export function useDeleteMilestone(id: string) {
  const invalidate = useInvalidate()

  return useMutation({
    mutationFn: async () =>
      unwrap(await api.DELETE('/api/v1/admin/milestones/{id}', { params: { path: { id } } })),
    onSuccess: invalidate,
  })
}

/** Rattache un livrable a un jalon, ou l'en detache avec `null`. */
export function useAttachDeliverable() {
  const invalidate = useInvalidate()

  return useMutation({
    mutationFn: async ({ deliverableId, milestoneId }: { deliverableId: string; milestoneId: string | null }) =>
      unwrap(
        await api.PUT('/api/v1/admin/deliverables/{id}/milestone', {
          params: { path: { id: deliverableId } },
          body: { milestone_id: milestoneId },
        }),
      ),
    onSuccess: invalidate,
  })
}
