import { Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import type { ReactNode } from 'react'

import { DashboardCard } from '@/components/dashboard-card'
import {
  DONE_COLOR,
  PRIORITY_TONE,
  PROGRESS_COLOR,
  tintOf,
  type StatusPill as StatusPillTokens,
} from '@/features/projects/format'
import { cn } from '@/lib/utils'
import type { Person, ProjectPriority } from '@/types/api'

/**
 * Vocabulaire visuel partage par les ecrans d'un projet.
 *
 * La jauge et la tuile vivaient dans la page de detail, ou elles etaient
 * seules a servir. Elles servent maintenant a l'en-tete du projet comme a ses
 * onglets : les tenir ici evite que la meme jauge ne prenne deux epaisseurs
 * selon l'ecran qui la dessine.
 */

/** Jauge fine : une part, jamais un axe gradue. */
export function Meter({
  ratio,
  color,
  className,
}: {
  ratio: number
  color: string
  className?: string
}) {
  return (
    <div
      aria-hidden
      className={cn('h-1 w-full overflow-hidden rounded-full bg-[#ebebeb]', className)}
    >
      <div
        className="h-full rounded-full"
        style={{ width: `${Math.min(100, Math.max(0, ratio))}%`, backgroundColor: color }}
      />
    </div>
  )
}

/**
 * Chiffre-cle de l'en-tete d'un projet.
 *
 * Elle se dessinait a part : un cadre gris, un intitule en bas de casse, pas
 * d'icone, pas de carte blanche. Posee au-dessus des blocs JALONS et
 * LIVRABLES, elle ne ressemblait a rien d'autre dans l'application.
 *
 * Elle prend donc le chassis commun — creux gris, icone, intitule en
 * capitales, carte blanche filetee posee dedans — et n'ajoute que ce qui la
 * distingue : un chiffre, sa jauge, sa mention.
 */
export function Tile({
  icon,
  label,
  value,
  hint,
  tone,
  children,
}: {
  icon: IconSvgElement
  label: string
  value: string
  hint?: string
  tone?: 'alert'
  children?: ReactNode
}) {
  return (
    <DashboardCard icon={icon} title={label}>
      <div className="flex flex-1 flex-col gap-1.5">
        <p
          className={cn(
            'text-[20px] leading-[1.3] font-semibold tabular-nums',
            tone === 'alert' ? 'text-[#e5484d]' : 'text-[#111]',
          )}
        >
          {value}
        </p>

        {/* Le pied est colle en bas et ses deux lignes ont une hauteur fixe,
            occupees ou non : c'est ce qui aligne les jauges entre elles et les
            mentions entre elles d'une tuile a l'autre. Sans cette reserve, une
            tuile sans jauge remonte sa mention d'un cran et la rangee ondule. */}
        <div className="mt-auto flex flex-col gap-1.5 pt-1">
          <div className="flex h-1 items-center">{children}</div>
          <p className="h-[18px] text-[12px] leading-[1.5] text-[#8d8d8d]">{hint}</p>
        </div>
      </div>
    </DashboardCard>
  )
}

/**
 * Vignette d'etat : fond clair, bordure a peine plus soutenue, point plein.
 *
 * La meme forme sert aux statuts de projet et de tache — seule la palette
 * change, et elle vient de la table du statut concerne.
 */
export function StatusPill({
  label,
  color,
  pill,
  className,
}: {
  label: string
  color: string
  pill: StatusPillTokens
  className?: string
}) {
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[12px] whitespace-nowrap',
        className,
      )}
      style={{ backgroundColor: pill.bg, borderColor: pill.border, color: pill.text }}
    >
      <span aria-hidden className="size-1.5 rounded-full" style={{ backgroundColor: color }} />
      {label}
    </span>
  )
}

/**
 * Vignette de priorite : aplat sature, texte blanc.
 *
 * `self-start` est dans le composant et non chez l'appelant : pose dans une
 * colonne flex, un enfant est etire sur toute la largeur par defaut, et la
 * vignette devenait une banniere. Chaque appelant aurait eu a s'en souvenir.
 */
export function PriorityTag({
  priority,
  className,
}: {
  priority: ProjectPriority
  className?: string
}) {
  const { label, bg } = PRIORITY_TONE[priority]

  return (
    <span
      className={cn(
        'inline-flex shrink-0 self-start items-center rounded-full px-2 py-0.5 text-[12px] text-white',
        className,
      )}
      style={{ backgroundColor: bg }}
    >
      {label}
    </span>
  )
}

/**
 * Pile de visages.
 *
 * La photo du compte quand il en a une, ses initiales sinon, sur la pastille
 * de couleur deduite de son identifiant. Les deux font le meme disque : la
 * pile ne se deforme pas selon que les comptes ont depose une photo ou non.
 */
export function Avatars({
  people,
  max = 3,
  size = 20,
}: {
  people: Person[]
  max?: number
  size?: number
}) {
  const shown = people.slice(0, max)
  const rest = people.length - shown.length
  const box = { width: size, height: size }
  const overlap = { marginLeft: -Math.round(size * 0.3) }

  return (
    <div className="flex items-center">
      {shown.map((member, index) => {
        const name = `${member.firstname} ${member.lastname}`.trim() || 'Sans nom'

        return (
          <div
            key={member.id}
            title={name}
            className="relative shrink-0 overflow-hidden rounded-full border border-white"
            style={{ ...box, ...(index === 0 ? {} : overlap), backgroundColor: tintOf(member.id) }}
          >
            {member.avatar_url == null || member.avatar_url === '' ? (
              <span
                className="flex size-full items-center justify-center font-medium text-[#1b1b1b]"
                style={{ fontSize: Math.max(9, Math.round(size * 0.42)) }}
              >
                {member.initials}
              </span>
            ) : (
              <img src={member.avatar_url} alt="" loading="lazy" className="size-full object-cover" />
            )}
          </div>
        )
      })}

      {rest > 0 && (
        <div
          title={`${rest} autre${rest > 1 ? 's' : ''}`}
          className="flex shrink-0 items-center justify-center rounded-full border border-white bg-[#e8e8e9] font-medium text-[#1b1b1b]"
          style={{ ...box, ...overlap, fontSize: Math.max(10, Math.round(size * 0.44)) }}
        >
          {rest}+
        </div>
      )}
    </div>
  )
}

/**
 * Choix des comptes internes affectes a un projet.
 *
 * Partage par les parametres du projet et par la modale « Inviter » : les deux
 * repondent a la meme question, et l'affectation decide de l'acces — une liste
 * qui se coche d'un cote et se presente autrement de l'autre finirait par
 * diverger sur ce qui compte, l'etat coche.
 *
 * Le composant ne detient pas la selection : il l'affiche et signale les
 * bascules. C'est l'ecran appelant qui decide quand ecrire — d'un coup avec le
 * reste du formulaire dans les parametres, a la validation de la modale
 * ailleurs.
 */
export function TeamPicker({
  people,
  selected,
  onToggle,
}: {
  people: Person[]
  selected: ReadonlySet<string>
  onToggle: (personID: string) => void
}) {
  if (people.length === 0) {
    return <p className="text-[14px] text-[#73757c]">Aucun compte interne à affecter.</p>
  }

  return (
    <ul className="flex flex-col gap-1">
      {people.map((person) => {
        const actif = selected.has(person.id)
        const nom = `${person.firstname} ${person.lastname}`.trim() || 'Sans nom'

        return (
          <li key={person.id}>
            <button
              type="button"
              role="checkbox"
              aria-checked={actif}
              onClick={() => onToggle(person.id)}
              className={cn(
                'flex w-full cursor-pointer items-center gap-3 rounded-[10px] border px-3 py-2 text-left transition-colors',
                actif
                  ? 'border-brand bg-[#fff7f2]'
                  : 'border-[#e8e8e9] bg-white hover:bg-[#f8f8f8]',
              )}
            >
              <span
                className={cn(
                  'flex size-5 shrink-0 items-center justify-center rounded-[6px] border',
                  actif ? 'border-brand bg-brand text-white' : 'border-[#e8e8e9] bg-white',
                )}
              >
                {actif && <HugeiconsIcon icon={Tick02Icon} size={14} strokeWidth={2.4} />}
              </span>

              <Avatars people={[person]} max={1} size={24} />
              <span className="min-w-0 flex-1 truncate text-[16px] text-[#1b1b1b]">{nom}</span>
            </button>
          </li>
        )
      })}
    </ul>
  )
}

/**
 * Anneau d'avancement.
 *
 * Trace a partir de la circonference plutot qu'anime : la valeur est connue au
 * rendu, et un anneau qui se remplit a chaque changement de page ferait
 * clignoter la grille pour ne rien apprendre.
 */
export function ProgressRing({ value }: { value: number }) {
  const ratio = Math.max(0, Math.min(100, value)) / 100
  const circumference = 2 * Math.PI * 8

  return (
    <svg viewBox="0 0 20 20" className="size-5 shrink-0 -rotate-90" aria-hidden>
      <circle cx="10" cy="10" r="8" fill="none" stroke="#e8e8e9" strokeWidth="2.5" />
      {ratio > 0 && (
        <circle
          cx="10"
          cy="10"
          r="8"
          fill="none"
          stroke={value >= 100 ? DONE_COLOR : PROGRESS_COLOR}
          strokeWidth="2.5"
          strokeLinecap="round"
          strokeDasharray={`${ratio * circumference} ${circumference}`}
        />
      )}
    </svg>
  )
}

/**
 * Barre d'avancement de la fiche projet : le pourcentage se lit dans la barre.
 *
 * Le chiffre bascule dehors sous les vingt pour cent — pose sur un remplissage
 * de quelques pixels, il deborderait sur le fond gris et deviendrait illisible.
 */
export function ProgressBar({ value }: { value: number }) {
  const ratio = Math.max(0, Math.min(100, value))
  const inside = ratio >= 20

  return (
    <div className="relative h-6 w-[200px] shrink-0 overflow-hidden rounded-full bg-[#e8e8e9]">
      <div
        className="flex h-full items-center rounded-full transition-[width]"
        style={{
          width: `${ratio}%`,
          backgroundColor: value >= 100 ? DONE_COLOR : PROGRESS_COLOR,
        }}
      >
        {inside && (
          <span className="pl-2 text-[12px] font-medium text-white tabular-nums">{ratio} %</span>
        )}
      </div>

      {!inside && (
        <span className="absolute inset-y-0 right-2 flex items-center text-[12px] font-medium text-[#73757c] tabular-nums">
          {ratio} %
        </span>
      )}
    </div>
  )
}
