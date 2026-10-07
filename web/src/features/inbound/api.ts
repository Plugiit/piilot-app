import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, unwrap } from '@/lib/api'
import type { InboundSettings } from '@/types/api'

export const inboundKeys = {
  settings: ['inbound', 'settings'] as const,
  held: (page: number) => ['inbound', 'held', page] as const,
  allHeld: ['inbound', 'held'] as const,
  detail: (id: string) => ['inbound', 'detail', id] as const,
}

/**
 * Reglages et etat de la releve. Releve toutes les trois secondes tant
 * qu'une releve demandee n'a pas eu lieu, toutes les minutes sinon.
 */
export const inboundSettingsQuery = queryOptions({
  queryKey: inboundKeys.settings,
  queryFn: async () => unwrap(await api.GET('/api/v1/admin/integrations/mail')),
  refetchInterval: (query) => (query.state.data?.status.poll_pending === true ? 3_000 : 60_000),
})

export interface InboundSettingsValues {
  address: string
  imap_enabled: boolean
  imap_host: string
  imap_port: number
  imap_security: 'tls' | 'none'
  imap_username: string
  /** Absent : garde le mot de passe en place. */
  imap_password?: string
  imap_folder: string
}

function useSetSettings() {
  const queryClient = useQueryClient()
  return (settings: InboundSettings) => queryClient.setQueryData(inboundKeys.settings, settings)
}

export function useUpdateInboundSettings() {
  const set = useSetSettings()
  return useMutation({
    mutationFn: async (body: InboundSettingsValues) => unwrap(await api.PUT('/api/v1/admin/integrations/mail', { body })),
    onSuccess: set,
  })
}

export function useRotateInboundSecret() {
  const set = useSetSettings()
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/integrations/mail/rotate')),
    onSuccess: set,
  })
}

export function useRequestPoll() {
  const set = useSetSettings()
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/api/v1/admin/integrations/mail/poll')),
    onSuccess: set,
  })
}

/** Les e-mails a trier. Relus chaque minute : ils arrivent d'eux-memes. */
export function heldEmailsQuery(page: number) {
  return queryOptions({
    queryKey: inboundKeys.held(page),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/inbound-emails', { params: { query: { page } } })),
    placeholderData: keepPreviousData,
    refetchInterval: 60_000,
  })
}

export function heldEmailQuery(id: string) {
  return queryOptions({
    queryKey: inboundKeys.detail(id),
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/inbound-emails/{id}', { params: { path: { id } } })),
  })
}

/** Apres un tri, la liste et les tickets bougent. */
function useInvalidate() {
  const queryClient = useQueryClient()
  return () => {
    void queryClient.invalidateQueries({ queryKey: inboundKeys.allHeld })
    void queryClient.invalidateQueries({ queryKey: ['tickets'] })
  }
}

export function useOpenTicketFromEmail() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: async ({ id, projectId }: { id: string; projectId: string }) =>
      unwrap(await api.POST('/api/v1/admin/inbound-emails/{id}/ticket', { params: { path: { id } }, body: { project_id: projectId } })),
    onSuccess: invalidate,
  })
}

export function useAttachEmail() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: async ({ id, numero }: { id: string; numero: number }) =>
      unwrap(await api.POST('/api/v1/admin/inbound-emails/{id}/attach', { params: { path: { id } }, body: { numero } })),
    onSuccess: invalidate,
  })
}

export function useDismissEmail() {
  const invalidate = useInvalidate()
  return useMutation({
    mutationFn: async (id: string) => {
      const { error } = await api.POST('/api/v1/admin/inbound-emails/{id}/ignore', { params: { path: { id } } })
      if (error !== undefined) throw error
    },
    onSuccess: invalidate,
  })
}

/** Pourquoi un e-mail attend, dans les mots de l'equipe. */
export const HELD_REASON: Record<string, string> = {
  unknown_sender: 'Expéditeur inconnu',
  ambiguous_sender: 'Adresse partagée par plusieurs clients',
  project_to_choose: 'Projet à choisir',
  sender_mismatch: 'Réponse d’une autre adresse que celle du client',
  from_team: 'Transféré par l’équipe',
  rate_limited: 'Trop de demandes en une heure',
  ticket_unmatched: 'Ticket introuvable',
}
