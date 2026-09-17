import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'

import { api, postFile, unwrap } from '@/lib/api'
import type { SidebarApp } from '@/types/api'

export const sidebarAppKeys = {
  all: ['sidebar-apps'] as const,
}

/**
 * Les apps du rail.
 *
 * Lue par le rail lui-meme, present sur chaque page, et par l'ecran qui le
 * regle. `staleTime` genereux : cette liste change quelques fois par an, et la
 * relire a chaque navigation ferait une requete de plus pour rien.
 */
export function sidebarAppListQuery() {
  return queryOptions({
    queryKey: sidebarAppKeys.all,
    queryFn: async () => unwrap(await api.GET('/api/v1/admin/sidebar-apps')),
    staleTime: 10 * 60 * 1000,
  })
}

/** Ce que le formulaire envoie. */
export interface SidebarAppValues {
  name: string
  url: string
  color: string
  position?: number
}

export function useCreateSidebarApp() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: SidebarAppValues) =>
      unwrap(await api.POST('/api/v1/admin/sidebar-apps', { body: values })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: sidebarAppKeys.all })
    },
  })
}

export function useUpdateSidebarApp(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (values: SidebarAppValues) =>
      unwrap(
        await api.PATCH('/api/v1/admin/sidebar-apps/{id}', {
          params: { path: { id } },
          body: values,
        }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: sidebarAppKeys.all })
    },
  })
}

export function useDeleteSidebarApp(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () =>
      unwrap(await api.DELETE('/api/v1/admin/sidebar-apps/{id}', { params: { path: { id } } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: sidebarAppKeys.all })
    },
  })
}

/**
 * Depot d'un logo.
 *
 * Passe par `postFile` et non par le client type : le navigateur doit poser
 * lui-meme la frontiere multipart, et l'ecraser casserait le decoupage cote
 * serveur.
 */
export function useUploadSidebarAppLogo(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (file: File) =>
      postFile<SidebarApp>(`/api/v1/admin/sidebar-apps/${id}/logo`, 'file', file),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: sidebarAppKeys.all })
    },
  })
}

/**
 * Repasse le logo en automatique.
 *
 * Le serveur repond sans logo : la recuperation sort vers le site et se fait en
 * tache de fond. D'ou les relectures differees — le logo arrive au passage
 * suivant du job, et l'ecran le montre sans qu'on ait a recharger.
 */
export function useRefetchSidebarAppLogo(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () =>
      unwrap(
        await api.POST('/api/v1/admin/sidebar-apps/{id}/logo/auto', {
          params: { path: { id } },
        }),
      ),
    onSuccess: () => {
      const relire = () => queryClient.invalidateQueries({ queryKey: sidebarAppKeys.all })

      void relire()
      // Le job passe toutes les minutes : ces deux relectures couvrent le cas
      // courant sans imposer de recharger la page. Au-dela, la prochaine
      // navigation s'en charge.
      setTimeout(relire, 5_000)
      setTimeout(relire, 20_000)
    },
  })
}

export function useDeleteSidebarAppLogo(id: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async () =>
      unwrap(
        await api.DELETE('/api/v1/admin/sidebar-apps/{id}/logo', { params: { path: { id } } }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: sidebarAppKeys.all })
    },
  })
}
