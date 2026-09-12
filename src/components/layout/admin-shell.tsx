import { ArrowDown01Icon, Search01Icon, StarIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { AnimatePresence, motion, type Transition } from 'framer-motion'
import { LogOut } from 'lucide-react'
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'

import accountMarkUrl from '@/assets/sidebar/rail-bottom.png'
import coolifyUrl from '@/assets/sidebar/app-coolify.svg'
import googleDriveUrl from '@/assets/sidebar/app-google-drive.svg'
import proxmoxUrl from '@/assets/sidebar/app-proxmox.svg'
import uptimeKumaUrl from '@/assets/sidebar/app-uptime-kuma.svg'
import logoUrl from '@/assets/sidebar/logo.svg'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  MODULES,
  activeDestination,
  destinationsOf,
  useActiveModule,
  type MenuItem,
} from '@/components/layout/modules'
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet'
import { favoriteProjectsQuery } from '@/features/projects/api'
import { logout } from '@/lib/auth'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'
import type { User } from '@/types/api'

/**
 * Un element du rail : soit un glyphe hugeicons, soit une marque exportee de
 * Figma. Les deux coexistent parce qu'aucune bibliotheque d'icones ne dessine
 * les logos Notion, Slack ou Drive — ils viennent du fichier de design.
 */
type RailMark =
  | { icon: IconSvgElement; label: string; to: string }
  | { src: string; alt: string; width: number; height: number; href?: string }

interface RailGroup {
  label: string
  marks: RailMark[]
  /** Groupe de navigation : ses marques sont des liens, l'URL dit lequel est actif. */
  navigable?: boolean
}
/* --------------------------------------------------------------------------
   Contenu du rail.

   Le menu du panneau, lui, est propre a chaque module : il vit dans
   `modules.ts`, aux cotes de la liste des modules.
   -------------------------------------------------------------------------- */

/**
 * Outils externes joints depuis le rail.
 *
 * Les trois derniers sont auto-heberges : ces adresses sont des suppositions,
 * a remplacer par celles des instances de l'agence.
 */
const TOOLS = {
  drive: 'https://drive.google.com',
  coolify: 'https://cool.plugiit.com',
  uptimeKuma: 'https://uptime.plugiit.com',
  proxmox: 'https://prox.plugiit.com',
} as const

const RAIL: RailGroup[] = [
  {
    label: 'Menu',
    navigable: true,
    marks: [...MODULES],
  },
  {
    label: 'App',
    marks: [
      { src: googleDriveUrl, alt: 'Google Drive', width: 18, height: 16.1, href: TOOLS.drive },
      { src: coolifyUrl, alt: 'Coolify', width: 18, height: 18, href: TOOLS.coolify },
      { src: uptimeKumaUrl, alt: 'Uptime Kuma', width: 18, height: 15.9, href: TOOLS.uptimeKuma },
      { src: proxmoxUrl, alt: 'Proxmox', width: 18, height: 15.6, href: TOOLS.proxmox },
    ],
  },
]

/** Intitule de section : DM Sans, casse haute, meme discretion dans les deux colonnes. */
function GroupLabel({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <p className={cn('font-heading font-medium text-[#73757c] opacity-70 uppercase', className)}>
      {children}
    </p>
  )
}

function RailMarkView({ mark, className }: { mark: RailMark; className?: string }) {
  if ('icon' in mark) {
    return <HugeiconsIcon icon={mark.icon} size={18} strokeWidth={1.6} className={className} />
  }

  // Chaque marque est detourée dans son fichier — son `viewBox` epouse le
  // dessin — puis posee a 18px de large. La hauteur suit le rapport propre au
  // logo : une taille unique en ecraserait certains, et une boite carree les
  // laisserait remplir des largeurs differentes.
  return (
    <img
      src={mark.src}
      alt={mark.alt}
      width={mark.width}
      height={mark.height}
      style={{ width: mark.width, height: mark.height }}
    />
  )
}

/**
 * Rail d'icones, colonne de gauche.
 *
 * Chaque groupe est pose dans un creux gris ; l'element actif est la seule
 * surface blanche du rail, ce qui le designe sans avoir besoin d'une couleur
 * d'accent.
 */
function Rail({ footer, scope }: { footer: ReactNode; scope: string }) {
  // L'element actif se lit dans l'URL, pas dans un etat local : un rechargement
  // ou un lien colle designent le bon module sans que rien n'ait a le retenir.
  const activeModule = useActiveModule()

  const transition = useSlideTransition()

  return (
    <div
      className={cn(
        'bg-surface flex h-full w-[60px] shrink-0 flex-col items-center justify-between border-[#d8d8d8] px-2.5 py-4',
        // Le filet ne separe le rail que du panneau : il n'apparait donc que la
        // ou les deux sont cote a cote — a partir de lg en colonne fixe, et
        // toujours dans le tiroir, ou ils voyagent ensemble. Entre md et lg le
        // rail borde la zone de contenu, qui a deja la sienne.
        scope === 'drawer' ? 'border-r' : 'lg:border-r',
      )}
    >
      <div className="flex flex-col items-center gap-5">
        <img src={logoUrl} alt="Plugiit" width={40} height={40} className="size-10" />

        <div className="flex flex-col items-center">
          {RAIL.map((group, index) => (
            <div
              key={group.label}
              className={cn(
                'flex flex-col items-center justify-center gap-2 pb-4',
                // Le dernier groupe ne porte pas de filet : rien ne le suit.
                index < RAIL.length - 1 && 'mb-4 border-b border-[#d8d8d8]',
              )}
            >
              <GroupLabel className="text-[10px] leading-[1.5] tracking-[0.4px]">
                {group.label}
              </GroupLabel>

              {/* La pastille active reprend les 12px de la maquette, comme la
                  pilule qui l'entoure. Geometriquement, un enfant en retrait de
                  2px demanderait 10px pour que les deux arrondis restent
                  concentriques : c'est le choix du dessin, assume tel quel. */}
              {/* Le groupe selectionnable n'a pas de rembourrage vertical : le
                  retrait de 2px qui detache la pastille du bord est porte par
                  la pastille elle-meme. Chaque case garde ainsi 40px quel que
                  soit son etat, donc rien ne bouge quand la selection change,
                  et la colonne conserve la hauteur de la maquette. */}
              <div
                className={cn(
                  'bg-surface-sunken flex flex-col items-center justify-center gap-1 rounded-[12px]',
                  !group.navigable && 'py-0.5',
                )}
              >
                {group.marks.map((mark, position) => {
                  const active = 'to' in mark && mark.to === activeModule?.to

                  const view = (
                    <RailMarkView mark={mark} className={active ? 'text-[#111]' : 'text-[#999]'} />
                  )
                  const box = 'relative flex size-10 items-center justify-center rounded-[12px]'

                  // Une marque qui porte un lien sort de l'application : nouvel
                  // onglet, et `noopener` pour que la page ouverte n'obtienne
                  // pas de reference sur celle-ci.
                  if ('href' in mark && mark.href) {
                    return (
                      <a
                        key={position}
                        href={mark.href}
                        target="_blank"
                        rel="noopener noreferrer"
                        title={mark.alt}
                        className={box}
                      >
                        {view}
                      </a>
                    )
                  }

                  // Marque decorative : ni route ni lien, rien a activer.
                  if (!('to' in mark)) {
                    return (
                      <div key={position} className={box}>
                        {view}
                      </div>
                    )
                  }

                  return (
                    <Link
                      key={position}
                      to={mark.to}
                      aria-label={mark.label}
                      title={mark.label}
                      // TanStack Router pose `aria-current="page"` des qu'un
                      // chemin prefixe l'URL, et ne se laisse pas surcharger :
                      // sans `exact`, le module serait annonce comme la page
                      // ouverte sur chacune de ses sous-routes. La pastille,
                      // elle, vient de `useActiveModule` et reste allumee.
                      activeOptions={{ exact: true }}
                      className={box}
                    >
                      {active && (
                        // Un seul et meme noeud pour toute la colonne : partage
                        // le `layoutId`, donc Framer Motion l'interpole de sa
                        // position precedente vers la nouvelle au lieu de le
                        // faire disparaitre ici et reapparaitre la.
                        <motion.span
                          layoutId={`${scope}-rail-highlight`}
                          transition={transition}
                          className="absolute inset-[2px] rounded-[12px] border border-[#e6e6e6] bg-white"
                        />
                      )}
                      <span className="relative z-10">{view}</span>
                    </Link>
                  )
                })}
              </div>
            </div>
          ))}
        </div>
      </div>

      {footer}
    </div>
  )
}

/**
 * Pied du rail : le carre d'identite de la maquette, qui porte le menu du
 * compte. La deconnexion doit rester joignable depuis le back-office ; c'est le
 * seul point du rail qui n'est pas decoratif.
 */
function AccountButton({ user }: { user: User }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  async function handleLogout() {
    await logout()
    // Le cache porte des donnees du compte qui se deconnecte : le vider evite
    // qu'une connexion suivante voie brievement les ecrans du precedent.
    queryClient.clear()
    void navigate({ to: '/login' })
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="Menu du compte"
        className="size-8.75 overflow-hidden rounded-[8px] bg-[#02474f] p-[7.955px] outline-none"
      >
        <img src={accountMarkUrl} alt="" className="size-full object-contain" />
      </DropdownMenuTrigger>

      <DropdownMenuContent align="start" side="top" className="w-52">
        <DropdownMenuLabel className="font-normal">
          <span className="block truncate text-sm font-medium">
            {user.firstname} {user.lastname}
          </span>
          <span className="text-muted-foreground block truncate text-xs">{user.email}</span>
        </DropdownMenuLabel>

        <DropdownMenuSeparator />

        <DropdownMenuItem onSelect={() => void handleLogout()}>
          <LogOut />
          Déconnexion
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** Champ de recherche de la maquette : decoratif tant qu'il n'y a rien a chercher. */
function SearchField() {
  return (
    <div className="bg-surface-sunken border-surface-sunken flex h-[39px] w-full items-center gap-1.5 rounded-[10px] border py-2.5 pr-[7px] pl-3.5">
      <HugeiconsIcon
        icon={Search01Icon}
        size={17}
        strokeWidth={1.6}
        className="shrink-0 text-[#73757c]"
      />
      <input
        type="search"
        placeholder="Rechercher"
        className="min-w-0 flex-1 bg-transparent text-sm text-[#111] outline-none placeholder:text-[#73757c]"
      />
      <kbd className="flex size-[26px] shrink-0 items-center justify-center rounded-md border border-[#e6e6e6] bg-white text-xs font-medium text-[#333]">
        /
      </kbd>
    </div>
  )
}

interface MenuEntryProps {
  item: MenuItem
  /** L'entree, ou l'une de ses sous-entrees, correspond a l'URL courante. */
  active: boolean
  /** Destination retenue par le menu, pour designer la sous-entree qui la porte. */
  activeTo: string | undefined
  expanded: boolean
  /** Identite du calque anime, commune aux entrees d'un meme groupe. */
  layoutId: string
  /** Identite du repere de bord, commune aux entrees d'un meme groupe. */
  markLayoutId: string
  /** Identite du repere de sous-entree, commune aux feuilles d'un meme menu. */
  leafLayoutId: string
  transition: Transition
  /** Replie ou deplie l'entree. Absent pour une entree sans sous-entrees. */
  onToggle: () => void
}

function MenuEntry({
  item,
  active,
  activeTo,
  expanded,
  layoutId,
  markLayoutId,
  leafLayoutId,
  transition,
  onToggle,
}: MenuEntryProps) {
  const { icon, label, children } = item

  // Une entree mene quelque part ou deplie, jamais les deux : le lien navigue,
  // le bouton ne fait que replier.
  const Box = children ? 'button' : Link
  const boxProps = children
    ? ({ type: 'button', onClick: onToggle, 'aria-expanded': expanded } as const)
    : // `exact` est indispensable : sans lui TanStack Router tient pour
      // courant tout lien dont le chemin prefixe l'URL, et « Tableau de bord »
      // (/pm) serait annonce comme la page ouverte alors qu'on lit
      // /pm/temps/saisie. L'attribut aria-current en decoule, on ne le pose
      // donc pas a la main.
      ({ to: item.to, activeOptions: { exact: true } } as const)

  return (
    <>
      <Box
        {...boxProps}
        // Le rembourrage ne depend pas de l'etat actif, seulement de la presence
        // d'un chevron : sinon l'entree decalerait son contenu en devenant
        // active, comme le faisait le rail avant d'etre corrige.
        className={cn(
          'relative flex h-[38px] w-full items-center gap-2 rounded-[8px] py-1.5 pl-3',
          children && 'pr-3',
        )}
      >
        {active && (
          // Repere de bord de la maquette : un trait de 6px a bouts ronds, pose
          // a cheval sur le bord gauche du panneau, donc seule sa moitie droite
          // se voit. Le panneau a 16px de rembourrage, d'ou le recul de l'entree
          // jusqu'au bord. Le centrage vertical est calcule (38 - 34) / 2 plutot
          // que fait par `translate` : Framer Motion pilote la transformation
          // pour animer le glissement et ecraserait la classe.
          <motion.span
            layoutId={markLayoutId}
            transition={transition}
            aria-hidden
            className="absolute top-[2px] left-[-16px] h-[34px] w-[3px] rounded-r-[3px] bg-[#ff782b]"
          />
        )}

        {active && (
          // La pastille vit dans l'entree d'arrivee. En vol elle survole les
          // entrees intermediaires ; sans cote explicite, l'ordre du document
          // decide qui passe devant, et elle masquait leur texte en descendant
          // alors qu'elle passait dessous en montant. Le contenu monte donc
          // d'un cran : la pastille reste derriere, quel que soit le sens.
          <motion.span
            layoutId={layoutId}
            transition={transition}
            className="absolute inset-0 rounded-[8px] border border-[#efefef] bg-white"
          />
        )}
        <HugeiconsIcon
          icon={icon}
          size={18}
          strokeWidth={1.6}
          className="relative z-10 shrink-0 text-[#111]"
        />
        <p className="relative z-10 min-w-0 flex-1 truncate text-left text-sm font-medium text-[#111]">
          {label}
        </p>
        {children && (
          // Un seul glyphe qui pivote, plutot que deux qui se remplacent : la
          // rotation dit dans quel sens va le repli, un echange ne dirait rien.
          <motion.span
            animate={{ rotate: expanded ? 180 : 0 }}
            transition={transition}
            className="relative z-10 flex shrink-0"
          >
            <HugeiconsIcon
              icon={ArrowDown01Icon}
              size={16}
              strokeWidth={1.6}
              className="text-[#111]"
            />
          </motion.span>
        )}
      </Box>

      {/* `initial={false}` empeche l'entree deja depliee au premier rendu de
          s'ouvrir toute seule sous les yeux de l'utilisateur. Le debordement est
          masque : c'est lui qui donne l'effet de rideau pendant que la hauteur
          se resorbe. */}
      <AnimatePresence initial={false}>
        {children && expanded && (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={transition}
            className="flex w-full flex-col overflow-hidden"
          >
            {children.map((leaf) => {
              const leafActive = leaf.to === activeTo

              return (
                <Link
                  key={leaf.to}
                  to={leaf.to}
                  activeOptions={{ exact: true }}
                  className="relative flex h-8 w-full items-center py-1.5 pl-10"
                >
                  {/* Le filet gris est porte par chaque feuille : mis bout a bout
                      il forme la ligne continue du menu. */}
                  <span className="absolute top-0 left-[22px] h-full w-px bg-[#d8d8d8]" />

                  {leafActive && (
                    // Le repere noir est un noeud unique par menu : il glisse
                    // d'une feuille a l'autre au lieu de se rallumer ailleurs.
                    // `z-10` le tient au-dessus des filets gris qu'il survole,
                    // sans quoi il passerait derriere en remontant.
                    <motion.span
                      layoutId={leafLayoutId}
                      transition={transition}
                      className="absolute top-0 left-[22px] z-10 h-full w-px bg-[#111]"
                    />
                  )}

                  <p
                    className={cn(
                      'min-w-0 flex-1 truncate text-left text-sm',
                      leafActive ? 'text-[#111]' : 'text-[#111]/70',
                    )}
                  >
                    {leaf.label}
                  </p>
                </Link>
              )
            })}
          </motion.div>
        )}
      </AnimatePresence>
    </>
  )
}

/**
 * Passe-plat entre le chassis, qui detient le tiroir, et l'en-tete de page qui
 * porte son bouton. Les deux sont trop eloignes dans l'arbre pour une prop.
 */
const SidebarContext = createContext<{ openDrawer: () => void } | null>(null)

export function useSidebar() {
  const context = useContext(SidebarContext)

  if (!context) {
    throw new Error('useSidebar doit etre utilise dans AdminShell')
  }

  return context
}

/** Panneau de navigation, colonne de droite du duo. */
/**
 * Projets etoiles, en bas du panneau du module.
 *
 * Les entrees au-dessus sont les memes pour tout le monde et vivent dans
 * `MODULES` ; celles-ci dependent du compte et changent en cours de session.
 * Elles ne peuvent donc pas rejoindre la liste statique — d'ou un bloc a part,
 * qui lit l'API plutot qu'une constante.
 *
 * Le bloc disparait quand il est vide : un intitule « Raccourcis » surmontant
 * du vide occuperait la place sans rien apprendre. Il n'y a pas d'etat de
 * chargement pour la meme raison — la barre est deja peinte, et une ligne
 * grise qui apparait puis se remplace saute aux yeux pour rien.
 */
function Shortcuts() {
  const { data } = useQuery(favoriteProjectsQuery)
  const items = data?.items ?? []

  if (items.length === 0) return null

  return (
    <div className="mt-auto flex w-full flex-col items-start gap-2 border-t border-[#d8d8d8] pt-3">
      <GroupLabel className="text-xs leading-[15.378px] tracking-[0.48px]">Raccourcis</GroupLabel>

      <div className="flex w-full flex-col">
        {items.map((project) => (
          <Link
            key={project.id}
            to="/pm/projets/$id"
            params={{ id: project.id }}
            title={project.name}
            className="flex w-full items-center gap-2 rounded-[8px] px-2 py-1.5 text-[13px] text-[#111] transition-colors hover:bg-[#ededed]"
            activeProps={{ className: 'bg-[#ededed] font-medium' }}
          >
            <HugeiconsIcon
              icon={StarIcon}
              size={16}
              strokeWidth={1.6}
              className="fill-brand text-brand shrink-0"
            />
            <span className="min-w-0 flex-1 truncate">{project.name}</span>
          </Link>
        ))}
      </div>
    </div>
  )
}

function Panel({ fallbackTitle, scope }: { fallbackTitle: string; scope: string }) {
  const activeModule = useActiveModule()
  const transition = useSlideTransition()

  const pathname = useRouterState({ select: (state) => state.location.pathname })

  // Une seule destination retenue pour tout le panneau : entree et sous-entree
  // se peignent a partir d'elle, donc elles ne peuvent pas se contredire.
  const current = activeDestination(activeModule, pathname)

  // Entrees depliees a la main. Une entree qui contient l'ecran courant est
  // toujours ouverte, meme absente de cet ensemble : arriver sur une
  // sous-entree par un lien direct doit la montrer, et replier ce qui indique
  // ou l'on se trouve ne rend service a personne.
  const [unfolded, setUnfolded] = useState<ReadonlySet<string>>(() => new Set())

  function toggle(key: string) {
    setUnfolded((prev) => {
      const next = new Set(prev)

      if (next.has(key)) {
        next.delete(key)
      } else {
        next.add(key)
      }

      return next
    })
  }

  // Hors module — l'accueil, a la racine — le panneau garde son titre mais
  // n'a aucun menu a proposer.
  const menu = activeModule?.menu ?? []

  return (
    <div className="bg-surface flex h-full w-[248px] shrink-0 flex-col overflow-y-auto p-4">
      <div className="flex w-full flex-col gap-4">
        <div className="flex w-full flex-col gap-1">
          <div className="flex w-full items-center pb-3">
            <h2 className="font-heading truncate text-xl font-medium text-[#1f1f1f]">
              {activeModule?.label ?? fallbackTitle}
            </h2>
          </div>
          <SearchField />
        </div>

        {menu.map((group, groupIndex) => (
          <div
            key={group.label}
            className="flex w-full flex-col items-start gap-2 border-b border-[#d8d8d8] pb-3"
          >
            <GroupLabel className="text-xs leading-[15.378px] tracking-[0.48px]">
              {group.label}
            </GroupLabel>

            <div className="flex w-full flex-col">
              {group.items.map((item, itemIndex) => {
                const key = `${groupIndex}:${itemIndex}`
                // Une entree est active quand l'ecran courant est le sien ou
                // celui de l'une de ses sous-entrees : selectionner une feuille
                // allume son menu avec elle.
                // L'entree s'allume quand la destination retenue est l'une
                // des siennes : celle de l'ecran ouvert, ou celle dont il est
                // une sous-page.
                const active = current !== undefined && destinationsOf(item).includes(current)

                return (
                  <MenuEntry
                    key={item.label}
                    item={item}
                    active={active}
                    activeTo={current}
                    expanded={unfolded.has(key) || active}
                    // Une identite par groupe : Framer Motion ne relie que deux
                    // noeuds de meme `layoutId`. La pastille glisse donc entre
                    // deux entrees d'un meme groupe, et se contente de disparaitre
                    // puis reapparaitre quand la selection change de groupe.
                    layoutId={`${scope}-${activeModule?.to}-highlight-${groupIndex}`}
                    markLayoutId={`${scope}-${activeModule?.to}-mark-${groupIndex}`}
                    // Un repere par menu deroulant : la barre glisse entre les
                    // feuilles d'un meme menu, et se contente d'apparaitre quand
                    // on passe d'un menu a l'autre.
                    leafLayoutId={`${scope}-${activeModule?.to}-leaf-${groupIndex}-${itemIndex}`}
                    transition={transition}
                    onToggle={() => toggle(key)}
                  />
                )
              })}
            </div>
          </div>
        ))}
      </div>

      {/* Les favoris sont des projets : ils n'ont rien a faire sous le CRM.
          `mt-auto` les plaque au bas du panneau — ils ne prolongent pas le
          menu, ils occupent le pied de la colonne. Quand le menu remplit deja
          la hauteur, la marge se reduit a rien et le bloc reprend sa place a
          la suite plutot que de deborder. */}
      {activeModule?.to === '/pm' && <Shortcuts />}
    </div>
  )
}

/**
 * Coquille du back-office : double navigation — rail d'icones puis panneau
 * detaille — et zone de contenu.
 *
 * Le rail porte le niveau haut (espace de travail, application) et le panneau
 * le niveau bas (les ecrans de l'espace courant). Les deux colonnes sont
 * statiques a ce stade : la maquette est posee, le cablage sur les routes
 * viendra avec les modules qu'elle doit desservir.
 *
 * Les couleurs sont ecrites en dur ici, contrairement au reste de
 * l'application : ce sont celles du fichier Figma, pas celles de l'echelle
 * neutre shadcn, et les faire passer par des tokens laisserait croire qu'elles
 * sont partagees avec le portail client.
 *
 * Comme dans `AppShell`, la hauteur est bornee a la fenetre et chaque colonne
 * gere son propre defilement — sinon un tableau long ferait defiler la
 * navigation avec lui.
 */
export function AdminShell({
  children,
  title,
  user,
}: {
  children: ReactNode
  /** Titre du panneau pour un ecran qui n'appartient a aucun module. */
  title: string
  user: User
}) {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const pathname = useRouterState({ select: (state) => state.location.pathname })

  // Naviguer depuis le tiroir doit le refermer : sinon il masque l'ecran qu'on
  // vient d'ouvrir. On suit le chemin plutot que d'intercepter chaque lien.
  useEffect(() => setDrawerOpen(false), [pathname])

  const rail = <Rail scope="fixed" footer={<AccountButton user={user} />} />

  return (
    <SidebarContext.Provider value={{ openDrawer: () => setDrawerOpen(true) }}>
      <div className="bg-surface flex h-screen w-full overflow-hidden">
        {/* Les deux colonnes pesent 308px : les laisser en place sous 1024px
            ne laisserait pas de quoi afficher un tableau. Le rail seul, a
            60px, reste tenable des la tablette ; le panneau attend le grand
            ecran et passe par le tiroir en attendant. */}
        <div className="hidden md:flex">{rail}</div>
        <div className="hidden lg:flex">
          <Panel scope="fixed" fallbackTitle={title} />
        </div>

        <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
          <SheetContent
            side="left"
            // La ou le rail est deja a l'ecran, le tiroir se range a cote
            // plutot que par-dessus : recouvrir le rail ferait perdre le
            // module ouvert au moment meme ou l'on cherche a en changer.
            // `!` est necessaire, le composant pose `left-0` via un
            // selecteur d'attribut, plus specifique qu'une classe.
            className="bg-surface w-auto max-w-none gap-0 border-r-[#d8d8d8] p-0 md:left-[60px]!"
          >
            <SheetTitle className="sr-only">Navigation</SheetTitle>
            <div className="flex h-full">
              {/* Le rail n'entre dans le tiroir que la ou il n'est pas deja
                  a l'ecran. */}
              <div className="flex md:hidden">
                <Rail scope="drawer" footer={<AccountButton user={user} />} />
              </div>
              <Panel scope="drawer" fallbackTitle={title} />
            </div>
          </SheetContent>
        </Sheet>

        {/* Colonne flex : le decollement de 16px vient de `PageFrame`, et la
            frame occupe tout le reste jusqu'au bas de la fenetre. */}
        <main className="bg-surface flex min-w-0 flex-1 flex-col overflow-hidden">{children}</main>
      </div>
    </SidebarContext.Provider>
  )
}
