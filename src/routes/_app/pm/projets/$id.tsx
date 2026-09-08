import {
  Calendar03Icon,
  CheckmarkSquare02Icon,
  Clock01Icon,
  File01Icon,
  LayoutTable01Icon,
  Loading03Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { createFileRoute, Link, notFound, Outlet, useMatchRoute } from '@tanstack/react-router'
import { motion } from 'framer-motion'

import { PageFrame } from '@/components/layout/page-frame'
import { DELIVERABLE_STATE, DELIVERABLES, PROJECTS, STATUS } from '@/features/projects/fixtures'
import {
  ALERT_COLOR,
  BILLABLE_COLOR,
  DONE_COLOR,
  Meter,
  PROGRESS_COLOR,
  Tile,
  WARN_COLOR,
} from '@/features/projects/ui'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' })
const RELATIVE = new Intl.RelativeTimeFormat('fr-FR', { numeric: 'auto' })

export const Route = createFileRoute('/_app/pm/projets/$id')({
  // Un identifiant inconnu n'affiche pas une page vide : la route echoue, et le
  // `notFoundComponent` dit ce qui manque plutot que de laisser un ecran de
  // cartes a zero faire croire a un projet sans activite.
  loader: ({ params }) => {
    const project = PROJECTS.find((entry) => entry.id === params.id)

    if (project === undefined) throw notFound()

    return project
  },
  notFoundComponent: () => (
    <PageFrame title="Projet introuvable">
      <div className="flex flex-col items-start gap-3 p-4">
        <p className="text-[13px] text-[#64748b]">Ce projet n’existe pas, ou il a été archivé.</p>
        <Link
          to="/pm/projets"
          search={{ page: 1, sort: 'due', dir: 'asc' }}
          className="text-[13px] font-medium text-[#4956f4] hover:underline"
        >
          Retour à la liste des projets
        </Link>
      </div>
    </PageFrame>
  ),
  component: ProjectLayout,
})

function daysUntil(date: Date) {
  const today = new Date()
  today.setHours(0, 0, 0, 0)

  return Math.round((date.getTime() - today.getTime()) / 86_400_000)
}

const TABS: { to: string; label: string; icon: IconSvgElement }[] = [
  { to: '/pm/projets/$id', label: 'Vue d’ensemble', icon: LayoutTable01Icon },
  { to: '/pm/projets/$id/taches', label: 'Tâches', icon: CheckmarkSquare02Icon },
]

/**
 * Onglets du projet.
 *
 * Des liens et non un etat local : un onglet est une adresse. On partage
 * « les tâches du portail client », on y revient par le bouton Retour, on la
 * met en favori — trois choses qu'un `useState` ne sait pas faire.
 *
 * Le filet actif est un noeud unique porte par `layoutId` : Framer le glisse
 * d'un onglet a l'autre au lieu de l'effacer ici pour le repeindre la. C'est
 * le meme ressort que le rail de modules et que les onglets du tiroir — trois
 * mouvements voisins d'allures differentes se remarqueraient aussitot.
 */
function ProjectTabs({ id }: { id: string }) {
  const matchRoute = useMatchRoute()
  const transition = useSlideTransition()

  return (
    <nav className="flex shrink-0 items-center gap-6 border-b border-[#ebebeb] px-4">
      {TABS.map((tab) => {
        // `exact` sur la vue d'ensemble seulement : sans lui, elle resterait
        // allumee sur l'onglet des taches, qui est une de ses sous-routes.
        const active =
          matchRoute({ to: tab.to, params: { id }, fuzzy: tab.to !== '/pm/projets/$id' }) !==
          false

        return (
          <Link
            key={tab.to}
            to={tab.to}
            params={{ id }}
            className="relative flex shrink-0 items-center gap-1.5 py-3"
          >
            <HugeiconsIcon
              icon={tab.icon}
              size={16}
              strokeWidth={1.6}
              className={cn('shrink-0 transition-colors', active ? 'text-[#111]' : 'text-[#999]')}
            />
            <span
              className={cn(
                'text-[13px] font-medium whitespace-nowrap transition-colors',
                active ? 'text-[#111]' : 'text-[#777]',
              )}
            >
              {tab.label}
            </span>

            {active && (
              <motion.span
                aria-hidden
                layoutId="project-tab"
                transition={transition}
                className="absolute right-0 -bottom-px left-0 h-[2px] bg-[#ff782b]"
              />
            )}
          </Link>
        )
      })}
    </nav>
  )
}

/**
 * Chassis d'un projet : ce qui ne change pas quand on passe d'un onglet a
 * l'autre.
 *
 * Les quatre chiffres restent sous les yeux d'un onglet a l'autre, et c'est
 * voulu : on deplace une tache sans perdre de vue qu'il reste six jours et que
 * le budget est deja depasse. Les redessiner dans chaque onglet aurait fait
 * quatre chiffres qui sautent a chaque navigation.
 */
function ProjectLayout() {
  const project = Route.useLoaderData()

  const budgetRatio = (project.hoursSpent / project.hoursSold) * 100
  const overBudget = project.hoursSpent > project.hoursSold
  const days = daysUntil(project.due)
  const late = project.status !== 'livre' && days < 0

  const pending = DELIVERABLES.filter((item) => item.state === 'review').length

  // La jauge du budget ne se colore que quand il y a de quoi s'inquieter :
  // encre tant qu'on est au large, orange dans les dix derniers pour cent,
  // rouge au-dela du vendu.
  const budgetColor = overBudget
    ? ALERT_COLOR
    : budgetRatio >= 90
      ? WARN_COLOR
      : BILLABLE_COLOR

  return (
    <PageFrame
      title={project.name}
      description={`${project.client} · ${STATUS[project.status].label}`}
    >
      <div className="flex min-h-full flex-col">
        {/* Les quatre chiffres qui decident si l'on doit s'inquieter, avant
            tout detail : avancement, budget, echeance, validation client. */}
        <div className="grid shrink-0 grid-cols-2 gap-3 p-4 lg:grid-cols-4">
          {/* La jauge prenait la couleur du statut : un projet livre affichait
              donc 100 % en gris, la teinte de « Livré » — une barre pleine qui
              avait l'air eteinte. L'avancement a sa propre couleur, orange puis
              verte une fois complet. */}
          <Tile icon={Loading03Icon} label="AVANCEMENT" value={`${project.progress} %`}>
            <Meter
              ratio={project.progress}
              color={project.progress === 100 ? DONE_COLOR : PROGRESS_COLOR}
            />
          </Tile>

          <Tile
            icon={Clock01Icon}
            label="BUDGET CONSOMMÉ"
            value={`${project.hoursSpent} / ${project.hoursSold} h`}
            tone={overBudget ? 'alert' : undefined}
            hint={
              overBudget
                ? `${project.hoursSpent - project.hoursSold} h au-delà du vendu`
                : `${project.hoursSold - project.hoursSpent} h restantes`
            }
          >
            <Meter ratio={budgetRatio} color={budgetColor} />
          </Tile>

          {/* « il y a 52 jours » sous une echeance depassee dit la distance
              sans dire le probleme. Un retard s'annonce comme un retard. */}
          <Tile
            icon={Calendar03Icon}
            label="ÉCHÉANCE"
            value={DATE_FORMAT.format(project.due)}
            tone={late ? 'alert' : undefined}
            hint={late ? `${-days} jours de retard` : RELATIVE.format(days, 'day')}
          />

          <Tile
            icon={File01Icon}
            label="EN ATTENTE DU CLIENT"
            value={`${pending}`}
            hint={
              pending === 0
                ? 'Rien à valider'
                : `livrable${pending > 1 ? 's' : ''} sur ${DELIVERABLES.length} à valider`
            }
          >
            {/* La part de ce qui dort chez le client, dans le jaune que le
                module donne deja a l'etat « Chez le client ». */}
            {pending > 0 && (
              <Meter
                ratio={(pending / DELIVERABLES.length) * 100}
                color={DELIVERABLE_STATE.review.color}
              />
            )}
          </Tile>
        </div>

        <ProjectTabs id={project.id} />

        <Outlet />
      </div>
    </PageFrame>
  )
}
