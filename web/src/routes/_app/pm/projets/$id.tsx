import {
  ArrowLeft02Icon,
  CheckmarkSquare02Icon,
  Delete02Icon,
  Exchange01Icon,
  KanbanIcon,
  Link04Icon,
  ListViewIcon,
  Mail01Icon,
  MoreHorizontalIcon,
  Settings02Icon,
  StarIcon,
  Ticket02Icon,
  File01Icon,
  Attachment02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, notFound, Outlet, useMatchRoute } from '@tanstack/react-router'
import { motion } from 'framer-motion'
import { useRef, type ReactNode } from 'react'
import { toast } from 'sonner'
import { z } from 'zod'

import { PageFrame, type Crumb } from '@/components/layout/page-frame'
import { TabBar, type Tab } from '@/components/layout/tab-bar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  projectDetailQuery,
  fileUrl,
  useDeleteProjectFile,
  useToggleFavorite,
  useUpdateProject,
  useUploadProjectFile,
} from '@/features/projects/api'
import { PROJECT_STATUS, PROJECT_STATUS_ORDER, parseApiDate } from '@/features/projects/format'
import { Avatars, PriorityTag, ProgressBar, StatusPill } from '@/features/projects/ui'
import { NewTaskDialog } from '@/features/tasks/new-task-dialog'
import { ServicePills } from '@/features/services/tag'
import { NewTicketDialog } from '@/features/tickets/new-ticket-dialog'
import { HttpError } from '@/lib/api'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'
import type { ProjectDetail, ProjectStatus } from '@/types/api'

import { InviteDialog } from './-invite'

const LONG_DATE = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

/**
 * La facon de regarder vit dans l'adresse, et sur le chassis plutot que sur
 * chaque onglet : on passe des taches aux tickets sans changer de lunettes, et
 * « le kanban de ce projet » se partage par lien.
 */
//
// Optionnelle et sans valeur de repli inscrite : le tableau est ce qu'on voit
// sans rien demander, et `?vue=table` n'apprendrait rien a une adresse. La
// declarer requise obligerait par ailleurs chaque lien vers un projet — il y en
// a dans les listes, les tableaux et les cartes — a porter une vue.
const searchSchema = z.object({
  vue: z.enum(['table', 'kanban']).optional().catch(undefined),
})

export const Route = createFileRoute('/_app/pm/projets/$id')({
  validateSearch: searchSchema,
  // Le projet est precharge ici et relu par `useQuery` dans le composant : le
  // loader supprime le clignotement a l'arrivee, la requete laisse les
  // compteurs se rafraichir quand une tache change de colonne.
  loader: async ({ context, params }) => {
    try {
      await context.queryClient.query({
        ...projectDetailQuery(params.id),
        staleTime: 'static',
      })
    } catch (error) {
      // Un identifiant inconnu n'est pas une panne : la route se declare
      // introuvable plutot que d'afficher un ecran d'erreur technique.
      if (error instanceof HttpError && error.status === 404) throw notFound()

      throw error
    }
  },
  notFoundComponent: () => (
    <PageFrame title="Projet introuvable">
      <div className="flex flex-col items-start gap-3 p-4">
        <p className="text-[13px] text-[#777]">Ce projet n’existe pas, ou il a été archivé.</p>
        <Link
          to="/pm/projets"
          search={{ page: 1, sort: 'due', dir: 'asc' }}
          className="text-[13px] font-medium text-[#111] underline underline-offset-4"
        >
          Retour à la liste des projets
        </Link>
      </div>
    </PageFrame>
  ),
  component: ProjectLayout,
})

/**
 * Retour a la liste, maillon commun aux ecrans d'un projet.
 *
 * Les parametres de recherche sont ceux de la liste au repos : sans eux, le
 * lien atterrirait sur une adresse incomplete que la route completerait par
 * ses valeurs de repli — le meme resultat, par un detour.
 */
const TOUS_LES_PROJETS: Crumb = {
  label: 'Tous les projets',
  to: '/pm/projets',
  search: { page: 1, sort: 'due', dir: 'asc' },
}

/**
 * Les onglets disent QUOI, la bascule dit COMMENT.
 *
 * Deux axes et non un seul : « Liste » et « Kanban » etaient deux vues des
 * seules taches, et poser « Tickets » a cote aurait mis une entite en face de
 * deux facons de lire. Chaque entite qui arrive ajoute un onglet, pas une
 * entree par vue.
 */
const TABS: Tab[] = [
  { to: '/pm/projets/$id/taches', label: 'Tâches', icon: CheckmarkSquare02Icon },
  { to: '/pm/projets/$id/tickets', label: 'Tickets', icon: Ticket02Icon },
]

// Deux tailles pour un meme rendu : les tracés n'occupent pas la meme part de
// leur boite — 20/24 pour les barres de la liste, 18/24 pour le carre du
// kanban. A taille egale, la liste paraissait un dixieme plus grosse que le
// kanban pose a cote d'elle ; ces deux valeurs leur donnent la meme emprise.
const VUES = [
  { vue: 'table', label: 'Tableau', icon: ListViewIcon, size: 15 },
  { vue: 'kanban', label: 'Kanban', icon: KanbanIcon, size: 17 },
] as const

/**
 * Bascule entre les deux facons de lire l'onglet ouvert.
 *
 * Meme dessin que le rail des modules : un creux qui porte deux cases, et une
 * pastille blanche qui glisse de l'une a l'autre. Le geste est le meme — passer
 * d'une vue a l'autre sans quitter ce qu'on regarde — donc il se montre pareil.
 *
 * La pastille est un noeud unique porte par `layoutId` : Framer l'interpole
 * d'une case a la suivante au lieu de l'effacer ici pour la repeindre la.
 *
 * Des liens et non des boutons : la vue vit dans l'adresse, au meme titre que
 * l'onglet. Chacune s'ouvre donc dans un nouvel onglet et se met en favori.
 */
function ViewSwitch() {
  const { vue } = Route.useSearch()
  const transition = useSlideTransition()

  return (
    <div className="bg-surface-sunken flex items-center gap-1 rounded-[12px]">
      {VUES.map((item) => {
        const active = (vue ?? 'table') === item.vue

        return (
          <Link
            key={item.vue}
            to="."
            search={(prev) => ({ ...prev, vue: item.vue })}
            aria-label={item.label}
            title={item.label}
            className="relative flex size-8 items-center justify-center rounded-[12px]"
          >
            {/* 10px et non 12 : un enfant en retrait de 2px doit retrancher
                ce retrait au rayon du parent pour que les deux arrondis
                restent concentriques. Le rail des modules garde 12px des deux
                cotes, par choix de dessin assume a cet endroit-la. */}
            {active && (
              <motion.span
                layoutId="project-view-highlight"
                transition={transition}
                className="absolute inset-[2px] rounded-[10px] border border-[#e6e6e6] bg-white"
              />
            )}
            <HugeiconsIcon
              icon={item.icon}
              size={item.size}
              strokeWidth={1.8}
              className={cn('relative z-10', active ? 'text-[#111]' : 'text-[#999]')}
            />
          </Link>
        )
      })}
    </div>
  )
}

/** Une ligne du bloc d'informations : intitule a gauche, valeur a droite. */
function MetaRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-4">
      <p className="w-[92px] shrink-0 text-[14px] text-[#73757c]">{label}</p>
      {children}
    </div>
  )
}

/** Taille lisible d'une piece jointe. */
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} Ko`

  return `${(bytes / (1024 * 1024)).toFixed(1).replace('.0', '')} Mo`
}

/**
 * Pieces jointes du projet.
 *
 * Le telechargement est un lien et non un `fetch` : l'API repond en
 * `Content-Disposition: attachment`, donc c'est au navigateur d'enregistrer le
 * fichier — le passer par du JavaScript obligerait a garder tout le contenu en
 * memoire pour le rendre ensuite.
 */
function Attachments({ project }: { project: ProjectDetail }) {
  const input = useRef<HTMLInputElement>(null)
  const upload = useUploadProjectFile(project.id)
  const remove = useDeleteProjectFile(project.id)

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
      {project.files.map((file) => (
        <span key={file.id} className="group/file flex items-center gap-1">
          <HugeiconsIcon icon={File01Icon} size={16} strokeWidth={1.6} className="text-[#73757c]" />
          <a
            href={fileUrl(file.id)}
            className="text-[14px] text-[#1b1b1b] underline underline-offset-2"
          >
            {file.filename}
          </a>
          <span aria-hidden className="size-1 rounded-full bg-[#d0d1d3]" />
          <span className="text-[14px] text-[#73757c]">{formatSize(file.size_bytes)}</span>
          <button
            type="button"
            aria-label={`Supprimer ${file.filename}`}
            onClick={() => remove.mutate(file.id)}
            className="ml-0.5 cursor-pointer text-[#a2a3a7] opacity-0 transition-opacity group-hover/file:opacity-100 hover:text-[#e5484d] focus-visible:opacity-100"
          >
            <HugeiconsIcon icon={Delete02Icon} size={14} strokeWidth={1.6} />
          </button>
        </span>
      ))}

      <input
        ref={input}
        type="file"
        hidden
        onChange={(event) => {
          const file = event.target.files?.[0]
          if (file !== undefined) upload.mutate(file)
          // Remis a zero pour que redeposer le meme fichier declenche bien un
          // nouvel evenement `change`.
          event.target.value = ''
        }}
      />

      <Button
        variant="ghost"
        size="xs"
        className="gap-1 text-[#73757c]"
        disabled={upload.isPending}
        onClick={() => input.current?.click()}
      >
        <HugeiconsIcon icon={Attachment02Icon} size={14} strokeWidth={1.6} />
        {upload.isPending ? 'Envoi…' : project.files.length === 0 ? 'Joindre un fichier' : 'Ajouter'}
      </Button>

      {upload.isError && (
        <span className="text-[13px] text-[#e5484d]">
          {upload.error instanceof HttpError ? upload.error.message : 'Envoi impossible'}
        </span>
      )}
    </div>
  )
}

/**
 * Le client du projet, et chez lui l'interlocuteur.
 *
 * Ces quatre champs etaient servis par l'endpoint et affiches nulle part : la
 * fiche la plus detaillee de l'application ne disait pas pour qui le projet
 * etait fait. Ils ne coutent donc aucune requete de plus.
 *
 * L'adresse est un `mailto:` et non un texte a recopier — ecrire au contact
 * est le seul geste qu'on fait avec elle.
 *
 * Le contact et son role peuvent manquer : un client se cree depuis le
 * formulaire de projet, ou son nom suffit. Les lignes vides ne s'affichent
 * pas plutot que d'annoncer « Non renseigne » trois fois.
 */
function ProjectClient({ project }: { project: ProjectDetail }) {
  const contact = project.client_contact_name.trim()
  const role = project.client_contact_role.trim()
  const email = project.client_contact_email ?? ''

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
      <span className="text-[14px] text-[#1b1b1b]">{project.client_name}</span>

      {contact !== '' && (
        <span className="flex items-center gap-3 text-[14px] text-[#73757c]">
          <span aria-hidden className="size-1 rounded-full bg-[#d0d1d3]" />
          {role === '' ? contact : `${contact}, ${role}`}
        </span>
      )}

      {email !== '' && (
        <a
          href={`mailto:${email}`}
          className="flex items-center gap-1 text-[14px] text-[#4770e4] underline underline-offset-2"
        >
          <HugeiconsIcon icon={Mail01Icon} size={16} strokeWidth={1.6} />
          {email}
        </a>
      )}
    </div>
  )
}

/**
 * Ce que disent les taches, a cote de ce que declare l'equipe.
 *
 * Les deux chiffres vivent sur la meme ligne parce que c'est leur ecart qui
 * informe : une jauge a 80 % au-dessus d'une tache sur dix faites dit quelque
 * chose qu'aucun des deux ne dit seul. Les compteurs sont tenus par
 * declencheur en base, donc les lire ici ne coute aucun COUNT.
 */
function TasksRatio({ project }: { project: ProjectDetail }) {
  if (project.tasks_total === 0) {
    return <p className="text-[14px] text-[#a2a3a7]">Aucune tâche</p>
  }

  // L'accord suit le nombre de taches faites : « 1 tache sur 3 terminee »,
  // « 12 taches sur 30 terminees ». Zero reste au singulier, comme en francais.
  const pluriel = project.tasks_done > 1 ? 's' : ''

  return (
    <p className="text-[14px] text-[#73757c]">
      {`${project.tasks_done} tâche${pluriel} sur ${project.tasks_total} terminée${pluriel}`}
    </p>
  )
}

/**
 * Les trois liens de travail du projet.
 *
 * Chacun porte son nom : « voici la preproduction » ne se devine pas d'une
 * adresse. Ceux qui ne sont pas renseignes ne s'affichent pas — une ligne
 * « aucun lien » par emplacement vide aurait fait trois lignes mortes.
 */
function ProjectLinks({ project }: { project: ProjectDetail }) {
  const liens = [
    { label: 'Figma', url: project.figma_url },
    { label: 'Production', url: project.prod_url },
    { label: 'Préproduction', url: project.preprod_url },
  ].filter((lien) => lien.url !== '')

  if (liens.length === 0) {
    return <p className="text-[14px] text-[#a2a3a7]">Aucun lien</p>
  }

  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
      {liens.map((lien) => (
        <a
          key={lien.label}
          href={lien.url}
          target="_blank"
          rel="noreferrer noopener"
          title={lien.url}
          className="flex items-center gap-1 text-[14px] text-[#4770e4] underline underline-offset-2"
        >
          <HugeiconsIcon icon={Link04Icon} size={16} strokeWidth={1.6} />
          {lien.label}
        </a>
      ))}
    </div>
  )
}

/**
 * Chassis d'un projet : ce qui ne change pas quand on passe d'un onglet a
 * l'autre.
 */
function ProjectLayout() {
  const { id } = Route.useParams()
  const { data: project } = useQuery(projectDetailQuery(id))
  const favorite = useToggleFavorite(id)
  const update = useUpdateProject(id)
  const matchRoute = useMatchRoute()
  const surTickets = matchRoute({ to: '/pm/projets/$id/tickets', params: { id } }) !== false

  // Le loader a deja rempli le cache : ce cas ne se produit qu'au tout premier
  // rendu d'une navigation sans prefetch.
  if (project === undefined) return null

  const status = PROJECT_STATUS[project.status]
  const start = parseApiDate(project.starts_on)
  const due = parseApiDate(project.due_on)

  return (
    <PageFrame
      title={project.name}
      trail={[TOUS_LES_PROJETS]}
    >
      <div className="flex min-h-full flex-col">
        <div className="flex flex-col gap-5 p-4">
          <div className="flex items-center justify-between gap-4">
            {/* Le resume se lit sur la carte de la liste mais disparaissait en
                ouvrant le projet. Il est attache au titre plutot que pose dans
                le bloc d'informations : ce n'est pas une valeur qu'on releve,
                c'est ce que le projet est. */}
            <div className="flex min-w-0 flex-col gap-1">
              <h1 className="truncate text-[24px] leading-[1.5] font-medium text-[#1b1b1b]">
                {project.name}
              </h1>

              {project.description !== '' && (
                <p className="text-[14px] leading-[1.5] text-[#73757c]">{project.description}</p>
              )}
            </div>

            <div className="flex shrink-0 items-center gap-4">
              <Avatars people={project.team} max={6} size={32} />

              <div className="flex items-center gap-2">
                <InviteDialog project={project} />

                <Button
                  variant="outline"
                  size="icon-lg"
                  aria-pressed={project.is_favorite}
                  aria-label={project.is_favorite ? 'Retirer des favoris' : 'Mettre en favori'}
                  onClick={() => favorite.mutate(!project.is_favorite)}
                >
                  <HugeiconsIcon
                    icon={StarIcon}
                    size={20}
                    strokeWidth={1.8}
                    className={cn(project.is_favorite && 'fill-brand text-brand')}
                  />
                </Button>

                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="icon-lg" aria-label="Actions du projet">
                      <HugeiconsIcon icon={MoreHorizontalIcon} size={20} strokeWidth={1.8} />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-56">
                    {/* Le statut se lit deux lignes plus bas mais ne s'y change
                        pas : c'est la seule valeur de l'en-tete qui bouge en
                        cours de projet, et aller la modifier dans les
                        parametres pour la voir revenir ici est un aller-retour
                        qu'on fait plusieurs fois par semaine.

                        L'etoile, l'invitation et la creation de tache ont deja
                        leur bouton a cote : les redire ici ne donnerait qu'un
                        second chemin vers le meme geste. */}
                    <DropdownMenuSub>
                      <DropdownMenuSubTrigger>
                        <HugeiconsIcon icon={Exchange01Icon} size={16} strokeWidth={1.6} />
                        Changer le statut
                      </DropdownMenuSubTrigger>
                      <DropdownMenuSubContent className="w-44">
                        {PROJECT_STATUS_ORDER.map((status: ProjectStatus) => (
                          <DropdownMenuItem
                            key={status}
                            disabled={status === project.status || update.isPending}
                            onSelect={() =>
                              update.mutate(
                                { status },
                                {
                                  onSuccess: () =>
                                    toast.success(`Statut : ${PROJECT_STATUS[status].label}`),
                                  onError: (error) =>
                                    toast.error(
                                      error instanceof HttpError
                                        ? error.message
                                        : 'Changement impossible',
                                    ),
                                },
                              )
                            }
                          >
                            <span
                              aria-hidden
                              className="size-1.5 shrink-0 rounded-full"
                              style={{ backgroundColor: PROJECT_STATUS[status].color }}
                            />
                            {PROJECT_STATUS[status].label}
                          </DropdownMenuItem>
                        ))}
                      </DropdownMenuSubContent>
                    </DropdownMenuSub>

                    <DropdownMenuItem asChild>
                      <Link to="/pm/projets/$id/parametres" params={{ id: project.id }}>
                        <HugeiconsIcon icon={Settings02Icon} size={16} strokeWidth={1.6} />
                        Paramètres du projet
                      </Link>
                    </DropdownMenuItem>
                    <DropdownMenuItem asChild>
                      <Link to="/pm/projets" search={{ page: 1, sort: 'due', dir: 'asc' }}>
                        <HugeiconsIcon icon={ArrowLeft02Icon} size={16} strokeWidth={1.6} />
                        Retour à la liste
                      </Link>
                    </DropdownMenuItem>

                    <DropdownMenuSeparator />

                    {/* Un lien vers la zone de danger, pas une suppression :
                        elle demande de recopier le nom du projet, et un menu
                        qui effacerait d'un clic viderait ce garde-fou de son
                        sens. L'entree dit ou aller, l'ecran fait le reste. */}
                    <DropdownMenuItem asChild variant="destructive">
                      <Link
                        to="/pm/projets/$id/parametres/zone-de-danger"
                        params={{ id: project.id }}
                      >
                        <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.6} />
                        Supprimer le projet
                      </Link>
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
          </div>

          <div className="flex flex-col gap-4">
            {/* En tete du bloc : un projet appartient d'abord a quelqu'un. */}
            <MetaRow label="Client">
              <ProjectClient project={project} />
            </MetaRow>

            <MetaRow label="Priorité">
              <PriorityTag priority={project.priority} />
            </MetaRow>

            {/* La ligne ne s'affiche que s'il y a des services : « Aucun » sur
                tous les projets internes n'apprendrait rien. */}
            {project.services.length > 0 && (
              <MetaRow label={project.services.length > 1 ? 'Services' : 'Service'}>
                <ServicePills services={project.services} />
              </MetaRow>
            )}

            <MetaRow label="Statut">
              <StatusPill label={status.label} color={status.color} pill={status.pill} />
            </MetaRow>

            <MetaRow label="Début">
              <p className="text-[14px] text-[#1b1b1b]">
                {start === null ? 'Sans date de début' : LONG_DATE.format(start)}
              </p>
            </MetaRow>

            <MetaRow label="Échéance">
              <p className="text-[14px] text-[#1b1b1b]">
                {due === null ? 'Sans échéance' : LONG_DATE.format(due)}
              </p>
            </MetaRow>

            <MetaRow label="Avancement">
              <ProgressBar value={project.progress} />
              <TasksRatio project={project} />
            </MetaRow>

            <MetaRow label="Document">
              <Attachments project={project} />
            </MetaRow>

            <MetaRow label="Liens">
              <ProjectLinks project={project} />
            </MetaRow>
          </div>
        </div>

        <div className="flex shrink-0 items-center justify-between gap-3 border-b border-[#e8e8e9] px-4">
          <TabBar tabs={TABS} params={{ id: project.id }} layoutId="project-tab" keepSearch />

          {/* L'action suit l'onglet : on depose un ticket depuis les tickets,
              on cree une tache depuis les taches. Les deux boutons cote a cote
              auraient demande de lire lequel des deux on visait.

              Rien ici ne doit depasser la hauteur d'un onglet : le filet actif
              est ancre au bas de son lien, et une rangee plus haute que les
              onglets les centrerait en decollant le filet du bord. */}
          <div className="flex items-center gap-2">
            <ViewSwitch />
            {surTickets ? (
              <NewTicketDialog projectId={project.id} />
            ) : (
              <NewTaskDialog projectId={project.id} />
            )}
          </div>
        </div>

        <Outlet />
      </div>
    </PageFrame>
  )
}
