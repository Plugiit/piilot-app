import { UserGroupIcon } from '@hugeicons/core-free-icons'
import { useQuery } from '@tanstack/react-query'

import { PanelCard } from '@/components/panel-card'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { dashboardQuery } from '@/features/projects/api'
import { tintOf } from '@/features/projects/format'
import { formatDuration } from '@/features/time/format'
import { sessionQuery } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { TeamDay } from '@/types/api'

/**
 * Pastille d'activite. Le vert dit « a saisi du temps aujourd'hui », pas
 * « connecte » : Piilot ne suit pas les presences, et c'est la saisie qui dit
 * qui a travaille sur quoi.
 */
const ACTIVE_COLOR = '#0db471'
const IDLE_COLOR = '#c4c4c4'

/** Bleu de « facturable », le meme que dans la carte du temps. */
const BILLABLE_COLOR = '#4956f4'

/** « Camille L. » : le prenom suffit a se reconnaitre, l'initiale a departager. */
function shortName({ person }: TeamDay) {
  const initial = person.lastname.trim().charAt(0)

  return initial === '' ? person.firstname : `${person.firstname} ${initial}.`
}

/** Ce sur quoi la personne a pointe en dernier aujourd'hui. */
function lastWork(member: TeamDay) {
  if (member.today_minutes === 0) return 'Aucune saisie aujourd’hui'
  if (member.last_task === '') return member.last_project

  return `${member.last_project} · ${member.last_task}`
}

function Row({ member, self }: { member: TeamDay; self: boolean }) {
  const active = member.today_minutes > 0
  const billable = active ? Math.round((member.today_billable_minutes / member.today_minutes) * 100) : null

  return (
    <li
      className={cn(
        'relative flex items-center justify-between gap-2 rounded-[8px] px-2 py-2.5 transition-colors hover:bg-[#f8f8f8]',
        // Un filet plutot qu'un simple fond gris : sur une carte blanche, le
        // #f8f8f8 seul ne se voyait pas.
        self &&
          'bg-[#f8f8f8] before:absolute before:inset-y-2 before:left-0 before:w-[3px] before:rounded-full before:bg-[#ff782b] before:content-[""]',
      )}
    >
      <div className="flex min-w-0 flex-1 items-center gap-2.5">
        {/* Une journee sans saisie recule d'un cran : la liste se lit alors
            de haut en bas comme de l'actif vers l'inactif. */}
        <div className={cn('relative shrink-0', !active && 'opacity-55')}>
          <Avatar className="size-9 rounded-[6px] after:rounded-[6px]">
            {member.person.avatar_url != null && member.person.avatar_url !== '' && (
              <AvatarImage src={member.person.avatar_url} alt="" className="rounded-[6px]" />
            )}
            <AvatarFallback
              className="rounded-[6px] text-[12px] font-semibold text-[#0f172a]"
              style={{ backgroundColor: tintOf(member.person.id) }}
            >
              {member.person.initials}
            </AvatarFallback>
          </Avatar>

          <span
            aria-label={active ? 'A saisi aujourd’hui' : 'Aucune saisie aujourd’hui'}
            className="absolute -right-0.5 -bottom-0.5 size-2.5 rounded-full ring-2 ring-white"
            style={{ backgroundColor: active ? ACTIVE_COLOR : IDLE_COLOR }}
          />
        </div>

        <div className="flex min-w-0 flex-col">
          <p className="truncate text-[12px] font-semibold text-[#0f172a]">
            {shortName(member)}
            {self && <span className="font-normal text-[#64748b]"> · vous</span>}
          </p>
          <p className="truncate text-[12px] text-[#64748b]">{lastWork(member)}</p>
        </div>
      </div>

      <div className="flex shrink-0 flex-col items-end gap-1.5">
        <p
          className={cn(
            'text-[12px] leading-none font-semibold tabular-nums',
            active ? 'text-[#0f172a]' : 'text-[#a8a7ab]',
          )}
        >
          {formatDuration(member.today_minutes)}
          <span className="font-normal text-[#a8a7ab]">
            {' '}
            · {formatDuration(member.week_minutes)} sem.
          </span>
        </p>

        {/* Une journee sans saisie n'a pas de part facturable a montrer. La
            jauge se lit d'un coup d'oeil sur la colonne entiere, la ou le
            chiffre nu se comparait mal d'une ligne a l'autre. */}
        {billable !== null && (
          <div className="flex items-center gap-1.5" title="Part facturable du jour">
            <div aria-hidden className="h-1 w-10 overflow-hidden rounded-full bg-[#ebebeb]">
              <div
                className="h-full rounded-full"
                style={{ width: `${billable}%`, backgroundColor: BILLABLE_COLOR }}
              />
            </div>

            <p className="w-10 text-right text-[12px] leading-none font-medium whitespace-nowrap text-[#64748b] tabular-nums">
              {billable} %
            </p>
          </div>
        )}
      </div>
    </li>
  )
}

/**
 * Equipe du jour : qui a saisi du temps, sur quoi, et sa charge de la semaine.
 *
 * Ni total d'heures ni part facturable d'ensemble : c'est le propos de la
 * carte « Temps facturable ». Celle-ci repond a « qui travaille sur quoi », et
 * le seul chiffre qu'elle avance de son cote est un decompte.
 */
export function TeamActivity() {
  const { data, isPending } = useQuery(dashboardQuery)
  const { data: session } = useQuery(sessionQuery)

  // Soi-meme en tete, puis l'ordre de l'API : ceux qui ont le plus saisi
  // aujourd'hui, puis sur la semaine.
  const members = [...(data?.team ?? [])].sort(
    (a, b) => Number(b.person.id === session?.id) - Number(a.person.id === session?.id),
  )
  const active = members.filter((member) => member.today_minutes > 0).length

  return (
    <PanelCard
      icon={UserGroupIcon}
      title="ÉQUIPE AUJOURD'HUI"
      action={
        !isPending && (
          <div className="flex items-center gap-1.5">
            <div aria-hidden className="size-1.5 shrink-0 rounded-full" style={{ backgroundColor: ACTIVE_COLOR }} />
            <p className="text-[12px] whitespace-nowrap text-[#64748b]">
              {active} / {members.length} ont saisi
            </p>
          </div>
        )
      }
    >
      {isPending && <div className="h-[260px] animate-pulse rounded-[4px] bg-[#f2f2f2]" />}

      {!isPending && members.length === 0 && (
        <p className="py-6 text-center text-[13px] text-[#8d8d8d]">Aucun membre dans l’équipe.</p>
      )}

      {/* Bornee en hauteur : l'equipe entiere tient dans la liste, mais la
          carte partage sa ligne avec le graphique de charge et ne doit pas
          l'etirer. */}
      {!isPending && members.length > 0 && (
        <ul className="flex max-h-[300px] flex-1 flex-col gap-1 overflow-y-auto">
          {members.map((member) => (
            <Row key={member.person.id} member={member} self={member.person.id === session?.id} />
          ))}
        </ul>
      )}
    </PanelCard>
  )
}
