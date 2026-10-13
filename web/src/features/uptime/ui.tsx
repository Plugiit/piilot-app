import { StatusPill } from '@/features/projects/ui'
import { formatDuration, MONITOR_STATUS, uptimeColor } from '@/features/uptime/format'
import type { UptimeDay, UptimeIncident, UptimeMonitor } from '@/types/api'

const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long' })
const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

export function MonitorStatus({ status }: { status: UptimeMonitor['status'] }) {
  const s = MONITOR_STATUS[status]
  return <StatusPill label={s.label} color={s.color} pill={s.pill} />
}

/**
 * La frise de trente jours : une case par jour, coloree par sa
 * disponibilite. Le libelle de chaque case la nomme pour les lecteurs
 * d'ecran ; a l'oeil, la couleur suffit.
 */
export function UptimeStrip({ days }: { days: UptimeDay[] }) {
  return (
    <div className="flex h-6 w-full items-stretch gap-[2px]" role="list" aria-label="Disponibilité des trente derniers jours">
      {days.map((d) => (
        <span
          key={d.day}
          role="listitem"
          aria-label={`${DAY.format(new Date(d.day))} : ${d.uptime === null ? 'pas de contrôle' : `${d.uptime} %`}`}
          title={`${DAY.format(new Date(d.day))} · ${d.uptime === null ? 'pas de contrôle' : `${d.uptime} %`}`}
          className="min-w-px flex-1 rounded-[2px]"
          style={{ backgroundColor: uptimeColor(d.uptime) }}
        />
      ))}
    </div>
  )
}

/** Une liste d'incidents, les plus recents d'abord. */
export function IncidentList({ items, showSite = true }: { items: UptimeIncident[]; showSite?: boolean }) {
  if (items.length === 0) return <p className="py-4 text-center text-[13px] text-[#73757c]">Aucun incident.</p>

  return (
    <ul className="flex flex-col">
      {items.map((i) => {
        const open = i.ended_at === null
        return (
          <li key={i.id} className="flex items-center gap-3 border-b border-[#f3f4f4] py-2 last:border-b-0">
            <span aria-hidden className="size-2 shrink-0 rounded-full" style={{ backgroundColor: open ? '#e5484d' : '#c4c4c4' }} />
            <span className="flex min-w-0 flex-1 flex-col gap-0.5">
              <span className="truncate text-[14px] text-[#1b1b1b]">
                {showSite ? i.monitor_name : open ? 'Panne en cours' : 'Panne'}
                {i.message !== '' && <span className="text-[#73757c]"> · {i.message}</span>}
              </span>
              <span className="text-[12px] text-[#8d8d8d]">
                {WHEN.format(new Date(i.started_at))}
                {i.ended_at !== null && ` → ${WHEN.format(new Date(i.ended_at))}`}
                {i.source === 'poll' && ' · déduit de la relève'}
              </span>
            </span>
            <span className={`shrink-0 text-[12px] tabular-nums ${open ? 'font-medium text-[#e5484d]' : 'text-[#73757c]'}`}>
              {open ? 'En cours' : i.duration_seconds === null ? '' : formatDuration(i.duration_seconds)}
            </span>
          </li>
        )
      })}
    </ul>
  )
}
