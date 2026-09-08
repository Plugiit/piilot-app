import { UserGroupIcon } from '@hugeicons/core-free-icons'

import { DashboardCard } from '@/components/dashboard-card'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { cn } from '@/lib/utils'

/**
 * Etat de presence. Le vert dit « saisit du temps en ce moment », pas
 * « connecte » : c'est ce qu'un tableau de bord de suivi a besoin de montrer.
 *
 * Les teintes sont celles du graphique de charge et de l'agenda — deux verts
 * voisins sur la meme ligne de grille se remarquent aussitot.
 */
const PRESENCE = {
  online: { label: 'En ligne', color: '#0db471' },
  away: { label: 'Absent', color: '#eab308' },
  offline: { label: 'Hors ligne', color: '#c4c4c4' },
} as const

type Presence = keyof typeof PRESENCE

/** Bleu de « facturable », le meme que dans la carte du temps. */
const BILLABLE_COLOR = '#4956f4'

/** Equipe figee : le module n'a pas encore de saisie de temps a lire. */
const MEMBERS: {
  name: string
  initials: string
  tint: string
  task: string
  /** Temps saisi aujourd'hui, en secondes. */
  seconds: number
  billable: number | null
  presence: Presence
  /** Sa ligne se detache : on se repere d'abord soi-meme dans une liste. */
  self?: boolean
}[] = [
  {
    name: 'Maxence M.',
    initials: 'MM',
    tint: '#e9dcb3',
    task: 'API Go · authentification',
    seconds: 3 * 3600 + 10 * 60 + 12,
    billable: 92,
    presence: 'online',
    self: true,
  },
  {
    name: 'Ugo L.',
    initials: 'UL',
    tint: '#cfd4c7',
    task: 'Branding · Eric HDK',
    seconds: 2 * 3600 + 15 * 60 + 45,
    billable: 88,
    presence: 'online',
  },
  {
    name: 'Camille R.',
    initials: 'CR',
    tint: '#e0dad5',
    task: 'Intégration maquettes',
    seconds: 1 * 3600 + 45 * 60 + 30,
    billable: 76,
    presence: 'online',
  },
  {
    name: 'Adrien B.',
    initials: 'AB',
    tint: '#dcd7e0',
    task: 'Recette — Portail client',
    seconds: 52 * 60 + 8,
    billable: 41,
    presence: 'away',
  },
  {
    name: 'Théo B.',
    initials: 'TB',
    tint: '#e0e0e0',
    task: 'Aucune saisie aujourd’hui',
    seconds: 0,
    billable: null,
    presence: 'offline',
  },
]

/** Poids de tri : qui travaille en ce moment se lit en premier. */
const PRESENCE_RANK: Record<Presence, number> = { online: 0, away: 1, offline: 2 }

function formatDuration(seconds: number) {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  return [hours, minutes, seconds % 60].map((part) => String(part).padStart(2, '0')).join(':')
}

export function TeamActivity() {
  // Soi-meme en tete, puis les presents, puis le temps saisi : l'ordre de la
  // base ferait descendre au milieu ceux qui travaillent en ce moment.
  const members = [...MEMBERS].sort(
    (a, b) =>
      Number(b.self ?? false) - Number(a.self ?? false) ||
      PRESENCE_RANK[a.presence] - PRESENCE_RANK[b.presence] ||
      b.seconds - a.seconds,
  )

  const online = members.filter((member) => member.presence === 'online').length

  return (
    <DashboardCard
      icon={UserGroupIcon}
      title="ÉQUIPE AUJOURD'HUI"
      action={
        <div className="ml-auto flex items-center gap-1.5">
          <div
            aria-hidden
            className="size-1.5 shrink-0 rounded-full"
            style={{ backgroundColor: PRESENCE.online.color }}
          />
          <p className="text-[12px] whitespace-nowrap text-[#64748b]">
            {online} / {members.length} en ligne
          </p>
        </div>
      }
    >
      {/* Ni total d'heures ni part facturable d'ensemble : c'est le propos de
          la carte « Temps facturable », une ligne plus haut. Celle-ci repond a
          « qui travaille sur quoi en ce moment », et le seul chiffre qu'elle
          avance de son cote est un decompte de presences. */}
      <ul className="flex flex-1 flex-col gap-1">
        {members.map((member) => (
          <li
            key={member.name}
            className={cn(
              'relative flex items-center justify-between gap-2 rounded-[8px] px-2 py-2.5 transition-colors hover:bg-[#f8f8f8]',
              // Un filet plutot qu'un simple fond gris : sur une carte
              // blanche, le #f8f8f8 seul ne se voyait pas. La couleur est
              // celle du jour choisi dans l'agenda.
              member.self &&
                'bg-[#f8f8f8] before:absolute before:inset-y-2 before:left-0 before:w-[3px] before:rounded-full before:bg-[#ff782b] before:content-[""]',
            )}
          >
            <div className="flex min-w-0 flex-1 items-center gap-2.5">
              <div
                className={cn(
                  'relative shrink-0',
                  // Une journee sans saisie recule d'un cran : la liste se lit
                  // alors de haut en bas comme de l'actif vers l'inactif.
                  member.presence === 'offline' && 'opacity-55',
                )}
              >
                {/* Le dessin exporte des photos. Des initiales tiennent lieu de
                    portrait sans embarquer de visage ni dependre d'un fichier
                    distant, et le carre arrondi du dessin l'emporte sur le rond
                    de shadcn. */}
                <Avatar className="size-9 rounded-[6px] after:rounded-[6px]">
                  <AvatarFallback
                    className="rounded-[6px] text-[12px] font-semibold text-[#0f172a]"
                    style={{ backgroundColor: member.tint }}
                  >
                    {member.initials}
                  </AvatarFallback>
                </Avatar>

                <span
                  aria-label={PRESENCE[member.presence].label}
                  className="absolute -right-0.5 -bottom-0.5 size-2.5 rounded-full ring-2 ring-white"
                  style={{ backgroundColor: PRESENCE[member.presence].color }}
                >
                  {/* Le halo ne bat que pour un chronometre en marche : c'est
                      la seule pastille qui dise quelque chose du present. */}
                  {member.presence === 'online' && (
                    <span
                      aria-hidden
                      className="absolute inset-0 rounded-full opacity-50 motion-safe:animate-ping"
                      style={{ backgroundColor: PRESENCE.online.color }}
                    />
                  )}
                </span>
              </div>

              <div className="flex min-w-0 flex-col">
                <p className="truncate text-[12px] font-semibold text-[#0f172a]">{member.name}</p>
                <p className="truncate text-[12px] text-[#64748b]">{member.task}</p>
              </div>
            </div>

            <div className="flex shrink-0 flex-col items-end gap-1.5">
              <p
                className={cn(
                  'text-[12px] leading-none font-semibold tracking-[0.24px] tabular-nums',
                  member.presence === 'offline' ? 'text-[#a8a7ab]' : 'text-[#0f172a]',
                )}
              >
                {formatDuration(member.seconds)}
              </p>

              {/* Une journee sans saisie n'a pas de part facturable a montrer :
                  le dessin garde la ligne et la rend invisible, on la retire.
                  Le chiffre nu se comparait mal d'une ligne a l'autre ; la
                  jauge se lit d'un coup d'oeil sur la colonne entiere. */}
              {member.billable !== null && (
                <div className="flex items-center gap-1.5">
                  <div aria-hidden className="h-1 w-10 overflow-hidden rounded-full bg-[#ebebeb]">
                    <div
                      className="h-full rounded-full"
                      style={{ width: `${member.billable}%`, backgroundColor: BILLABLE_COLOR }}
                    />
                  </div>

                  <p className="w-8 text-right text-[12px] leading-none font-medium text-[#64748b] tabular-nums">
                    {member.billable} %
                  </p>
                </div>
              )}
            </div>
          </li>
        ))}
      </ul>
    </DashboardCard>
  )
}
