import type { StatusPill } from '@/features/projects/format'
import type { ClientStatus } from '@/types/api'

/**
 * Etapes du pipeline commercial.
 *
 * L'ordre vit ici et non en base : la base valide les valeurs par un CHECK,
 * elle n'a pas a savoir que « devis » vient apres « lead ». C'est cet ordre que
 * le kanban suit, colonne par colonne.
 *
 * Reserve heritee du schema : « devis » ne pointe vers aucun devis — la
 * facturation reste sur l'ancienne plateforme. L'etape ne vaut que ce que
 * l'equipe y met a la main.
 */
export const CLIENT_STATUS_ORDER: ClientStatus[] = ['lead', 'devis', 'actif', 'veille', 'perdu']

/**
 * Libelles et teintes, sur le meme gabarit que les statuts de projet : une
 * couleur pleine pour les pastilles, un triplet pour les etiquettes.
 */
export const CLIENT_STATUS: Record<
  ClientStatus,
  { label: string; color: string; pill: StatusPill }
> = {
  lead: {
    label: 'Piste',
    color: '#73757c',
    pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#5b5d63' },
  },
  devis: {
    label: 'Devis',
    color: '#4956f4',
    pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' },
  },
  actif: {
    label: 'Actif',
    color: '#0db471',
    pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' },
  },
  veille: {
    label: 'En veille',
    color: '#eab308',
    pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' },
  },
  perdu: {
    label: 'Perdu',
    color: '#d4183d',
    pill: { bg: '#ffe8ec', border: '#ffd0d8', text: '#a30f2c' },
  },
}
