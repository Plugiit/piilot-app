import { Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'

import { Input } from '@/components/ui/input'
import { PROGRESS_COLOR } from '@/features/projects/format'
import { cn } from '@/lib/utils'
import type { PortalActivityItem } from '@/types/api'

/**
 * Les briques partagees du tableau de bord du portail : les tuiles chiffrees
 * de la premiere ligne, et l'historique d'un projet. Le reste de l'ecran est
 * dans journey.tsx.
 */

// --- Tuile chiffree ----------------------------------------------------------

/**
 * Barre segmentee : une suite de traits dont la part pleine dit le ratio. Se
 * lit d'un coup d'oeil, sans graduation.
 */
export function SegmentBar({ ratio, segments = 44, color = PROGRESS_COLOR }: { ratio: number; segments?: number; color?: string }) {
  const filled = Math.round(Math.max(0, Math.min(1, ratio)) * segments)
  return (
    <div aria-hidden className="flex h-[26px] w-full items-center justify-between gap-px">
      {Array.from({ length: segments }, (_, i) => (
        <span key={i} className="h-full w-[3px] rounded-full" style={{ backgroundColor: i < filled ? color : '#e8e8e9' }} />
      ))}
    </div>
  )
}

/**
 * Une tuile : l'intitule, le chiffre, la barre, et une ligne de contexte au
 * pied. Le chassis est celui des cartes du tableau de bord de l'agence.
 */
export function StatTile({
  label,
  value,
  ratio,
  color,
  footer,
  aside,
  chart,
}: {
  label: string
  value: ReactNode
  /** La barre segmentee sous le chiffre ; sans ratio, pas de barre. */
  ratio?: number
  color?: string
  footer: ReactNode
  aside?: ReactNode
  /** Remplace la barre : un graphique pose au ras du bas de la carte blanche. */
  chart?: ReactNode
}) {
  return (
    <section className="flex h-full min-w-0 flex-col gap-0.5 overflow-clip rounded-[12px] border border-surface-sunken bg-surface p-0.5">
      <div className={cn('flex w-full flex-1 flex-col gap-1 overflow-hidden rounded-[10px] bg-white p-2', chart !== undefined && 'pb-0')}>
        <p className="text-[14px] leading-[1.5] text-[#1b1b1b]">{label}</p>
        <p className="font-heading text-[32px] leading-[1.2] font-medium text-[#1b1b1b] tabular-nums">{value}</p>
        {/* Le graphique prend la hauteur de la barre plus le rembourrage du
            bas (26 + 8) : la tuile garde la hauteur de ses voisines, et le
            zero tombe sur le bord de la carte blanche. */}
        {chart !== undefined ? <div className="-mx-2 h-[34px]">{chart}</div> : ratio !== undefined && <SegmentBar ratio={ratio} color={color} />}
      </div>
      <div className="flex w-full items-center gap-1 px-2 py-1.5 text-[12px] leading-[1.5]">
        <span className="min-w-0 flex-1 truncate text-[#1b1b1b]">{footer}</span>
        {aside}
      </div>
    </section>
  )
}

// --- Historique ----------------------------------------------------------------

/**
 * Genres d'entree de l'historique : etiquette et teinte du filet, dans la
 * palette de l'agenda du tableau de bord de l'agence.
 */
const ACTIVITY: Record<PortalActivityItem['kind'], { label: string; color: string; title: (i: PortalActivityItem) => string }> = {
  deliverable_submitted: {
    label: 'LIVRABLE',
    color: '#6060d8',
    title: (i) => (i.number > 1 ? `Nouvelle version de « ${i.title} »` : `« ${i.title} » déposé`),
  },
  deliverable_validated: { label: 'VALIDÉ', color: '#00ac47', title: (i) => `« ${i.title} » validé` },
  milestone_reached: { label: 'ÉTAPE', color: '#cf029f', title: (i) => `${i.title} atteinte` },
  deployment: { label: 'MISE EN LIGNE', color: '#ff782b', title: (i) => (i.title !== '' ? `Mise en ligne ${i.title}` : 'Mise en ligne') },
  ticket_replied: { label: 'RÉPONSE', color: '#3b45c9', title: (i) => `Réponse à votre demande #${i.number}` },
}

const DAY = new Intl.DateTimeFormat('fr-FR', { weekday: 'long', day: 'numeric', month: 'long' })
const TIME = new Intl.DateTimeFormat('fr-FR', { hour: '2-digit', minute: '2-digit' })

function dayLabel(date: Date): string {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const day = new Date(date)
  day.setHours(0, 0, 0, 0)
  const diff = Math.round((today.getTime() - day.getTime()) / 86_400_000)
  if (diff === 0) return 'Aujourd’hui'
  if (diff === 1) return 'Hier'
  const label = DAY.format(date)
  return label.charAt(0).toUpperCase() + label.slice(1)
}

/** L'historique du projet suivi, cherchable, regroupe par jour. */
export function History({ items }: { items: PortalActivityItem[] }) {
  const [query, setQuery] = useState('')
  const q = query.trim().toLowerCase()
  const shown = items.filter(
    (i) => q === '' || ACTIVITY[i.kind].title(i).toLowerCase().includes(q) || i.project.name.toLowerCase().includes(q),
  )

  const days: { label: string; entries: PortalActivityItem[] }[] = []
  for (const item of shown) {
    const label = dayLabel(new Date(item.at))
    const last = days.at(-1)
    if (last?.label === label) last.entries.push(item)
    else days.push({ label, entries: [item] })
  }

  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-[16px] font-medium text-[#111]">Historique</h2>
      </div>
      <div className="relative">
        <HugeiconsIcon
          icon={Search01Icon}
          size={16}
          strokeWidth={1.6}
          className="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-[#8d8d8d]"
        />
        <Input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Rechercher"
          aria-label="Rechercher dans l’historique"
          className="h-9 pl-8 text-[13px]"
        />
      </div>

      {days.length === 0 ? (
        <p className="text-[14px] text-[#111]/50">{q === '' ? 'Rien encore : l’activité de vos projets s’affichera ici.' : 'Rien ne correspond.'}</p>
      ) : (
        days.map((day) => (
          <section key={day.label} className="flex flex-col gap-3">
            {/* Un filet sur toute la largeur, le jour pose au centre sur un
                fond blanc qui l'interrompt. */}
            <h3 className="relative flex items-center justify-center">
              <span aria-hidden className="absolute inset-x-0 top-1/2 h-px bg-[#ebebeb]" />
              <span className="relative bg-white px-3 text-[12px] leading-[1.4] font-semibold text-[#444]">{day.label}</span>
            </h3>
            <div className="flex flex-col gap-4">
              {day.entries.map((item) => (
                <HistoryEntry key={`${item.kind}-${item.ref_id}-${item.at}`} item={item} />
              ))}
            </div>
          </section>
        ))
      )}
    </section>
  )
}

/**
 * Une entree facon agenda : le filet et l'etiquette colores, le titre, le
 * detail. Partagee par l'historique et les actions requises, pour que le
 * panneau de droite se lise d'un seul geste.
 */
export function AgendaEntry({
  color,
  label,
  meta,
  title,
  detail,
}: {
  color: string
  label: string
  meta?: ReactNode
  title: string
  detail: ReactNode
}) {
  return (
    <>
      <span aria-hidden className="relative w-[10px] shrink-0 self-stretch">
        <span className="absolute top-0 left-0 h-3 w-[6px] rounded-l-full" style={{ backgroundColor: color }} />
        <span className="absolute top-[7px] bottom-0 left-[4px] w-[2px]" style={{ backgroundColor: color }} />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-1.5">
        <span className="flex items-center gap-2">
          <span className="text-[12px] font-medium tracking-[0.48px]" style={{ color }}>
            {label}
          </span>
          {meta !== undefined && <span className="ml-auto text-[11px] text-[#111]/50 tabular-nums">{meta}</span>}
        </span>
        <span className="flex flex-col gap-0.5">
          <span className="truncate text-[14px] font-semibold text-[#111]">{title}</span>
          <span className="truncate text-[13px] leading-[1.5] text-[#111]/70">{detail}</span>
        </span>
      </span>
    </>
  )
}

export const AGENDA_ENTRY = '-mx-2 flex w-[calc(100%+1rem)] gap-1 rounded-[8px] px-2 py-1 text-left transition-colors hover:bg-[#f8f8f8]'

/** Une entree de l'historique, liee a ce qu'elle concerne. */
function HistoryEntry({ item }: { item: PortalActivityItem }) {
  const { label, color, title } = ACTIVITY[item.kind]
  const body = (
    <AgendaEntry
      color={color}
      label={label}
      meta={TIME.format(new Date(item.at))}
      title={title(item)}
      detail={item.kind === 'ticket_replied' ? item.title : item.project.name}
    />
  )
  const className = AGENDA_ENTRY

  switch (item.kind) {
    case 'deliverable_submitted':
    case 'deliverable_validated':
      return (
        <Link to="/client/livrables/$id" params={{ id: item.ref_id }} className={className}>
          {body}
        </Link>
      )
    case 'ticket_replied':
      return (
        <Link to="/client/tickets/$id" params={{ id: item.ref_id }} className={className}>
          {body}
        </Link>
      )
    default:
      return (
        <Link to="/client/projets/$id" params={{ id: item.project.id }} className={className}>
          {body}
        </Link>
      )
  }
}
