import {
  AiMagicIcon,
  Analytics01Icon,
  ArrowDown01Icon,
  Copy01Icon,
  CodeXmlIcon,
  File01Icon,
  Home03Icon,
  Mail01Icon,
  Megaphone01Icon,
  PenTool03Icon,
  Search01Icon,
  SmartPhone01Icon,
  SparklesIcon,
  Wallet01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { AnimatePresence, motion, useReducedMotion, type Transition } from 'framer-motion'
import { LogOut } from 'lucide-react'
import { useState, type ReactNode } from 'react'

import accountMarkUrl from '@/assets/sidebar/rail-bottom.png'
import coolifyUrl from '@/assets/sidebar/app-coolify.svg'
import googleDriveUrl from '@/assets/sidebar/app-google-drive.svg'
import proxmoxUrl from '@/assets/sidebar/app-proxmox.svg'
import uptimeKumaUrl from '@/assets/sidebar/app-uptime-kuma.svg'
import logoUrl from '@/assets/sidebar/logo.svg'
import workspaceGoogleUrl from '@/assets/sidebar/rail-group-3.svg'
import workspaceLogomarkUrl from '@/assets/sidebar/rail-group-2.svg'
import workspaceTriangleUrl from '@/assets/sidebar/rail-group-1.svg'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { logout } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { User } from '@/types/api'

/**
 * Un element du rail : soit un glyphe hugeicons, soit une marque exportee de
 * Figma. Les deux coexistent parce qu'aucune bibliotheque d'icones ne dessine
 * les logos Notion, Slack ou Drive — ils viennent du fichier de design.
 */
type RailMark =
  | { icon: IconSvgElement; label: string }
  | { src: string; alt: string; width: number; height: number; href?: string }

interface RailGroup {
  label: string
  marks: RailMark[]
  /** Groupe selectionnable : ses marques sont des boutons, une seule est active. */
  selectable?: boolean
}

interface MenuLeaf {
  label: string
}

interface MenuItem {
  icon: IconSvgElement
  label: string
  /** Presence d'enfants : l'entree porte un chevron et peut se deplier. */
  children?: MenuLeaf[]
  /** Etat de depart du repli ; ensuite c'est le clic qui commande. */
  expanded?: boolean
}

interface MenuGroup {
  label: string
  items: MenuItem[]
}

/* --------------------------------------------------------------------------
   Contenu de la maquette.

   Statique et volontairement decorrele des routes : c'est le gabarit visuel de
   la navigation, pas encore la navigation. Les libelles seront remplaces quand
   les modules PM existeront ; la structure — rail, groupes, arborescence — est
   celle qui restera.
   -------------------------------------------------------------------------- */

/**
 * Outils externes joints depuis le rail.
 *
 * Les trois derniers sont auto-heberges : ces adresses sont des suppositions,
 * a remplacer par celles des instances de l'agence.
 */
const TOOLS = {
  drive: 'https://drive.google.com',
  coolify: 'https://coolify.plugiit.com',
  uptimeKuma: 'https://uptime.plugiit.com',
  proxmox: 'https://proxmox.plugiit.com',
} as const

const RAIL: RailGroup[] = [
  {
    label: 'Menu',
    selectable: true,
    marks: [
      { icon: Mail01Icon, label: 'Messages' },
      { icon: Analytics01Icon, label: 'Statistiques' },
      { icon: Wallet01Icon, label: 'Facturation' },
      { icon: SmartPhone01Icon, label: 'Mobile' },
      { icon: SparklesIcon, label: 'Assistant' },
    ],
  },
  {
    label: 'Group',
    marks: [
      { src: workspaceTriangleUrl, alt: '', width: 18, height: 16.4 },
      { src: workspaceLogomarkUrl, alt: '', width: 18, height: 18.4 },
      { src: workspaceGoogleUrl, alt: '', width: 18, height: 17.8 },
    ],
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

const MENU: MenuGroup[] = [
  {
    label: 'Général',
    items: [
      { icon: Home03Icon, label: 'Tableau de bord' },
      { icon: AiMagicIcon, label: 'Projets' },
      { icon: Analytics01Icon, label: 'Temps passé' },
      { icon: File01Icon, label: 'Livrables' },
    ],
  },
  {
    label: 'Production',
    items: [
      { icon: Copy01Icon, label: 'Jalons' },
      { icon: Megaphone01Icon, label: 'Tickets' },
    ],
  },
  {
    label: 'Ressources',
    items: [
      {
        icon: PenTool03Icon,
        label: 'Design UI/UX',
        expanded: true,
        children: [{ label: 'Wireframes' }, { label: 'Maquettes' }, { label: 'Prototype' }],
      },
      {
        icon: CodeXmlIcon,
        label: 'Intégration',
        children: [{ label: 'Front' }, { label: 'Back' }, { label: 'Recette' }],
      },
    ],
  },
]

/**
 * Ressort de la pastille active, partage par le rail et le panneau.
 *
 * Un ressort plutot qu'une duree : la pastille garde la meme allure quelle que
 * soit la distance parcourue, d'un voisin immediat aux deux bouts de la
 * colonne. `bounce: 0` lui evite de depasser sa cible.
 */
function useSlideTransition() {
  const reduced = useReducedMotion()

  return reduced ? { duration: 0 } : ({ type: 'spring', visualDuration: 0.25, bounce: 0 } as const)
}

/** Intitule de section : DM Sans, casse haute, meme discretion dans les deux colonnes. */
function GroupLabel({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <p className={cn('font-heading font-medium text-[#5b5b5b] opacity-70 uppercase', className)}>
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
function Rail({ footer }: { footer: ReactNode }) {
  // Selection locale : le rail n'est branche sur aucune route, mais il doit
  // deja repondre au clic pour que la maquette se manipule. L'etat partira
  // dans l'URL quand chaque marque desservira un espace.
  const [selected, setSelected] = useState(0)

  const transition = useSlideTransition()

  return (
    <div className="bg-surface flex h-full w-[60px] shrink-0 flex-col items-center justify-between border-r border-[#ebebeb] pt-3 pb-5">
      <div className="flex flex-col items-center gap-5">
        <img src={logoUrl} alt="Plugiit" width={40} height={40} className="size-9" />

        <div className="flex flex-col items-center">
          {RAIL.map((group, index) => (
            <div
              key={group.label}
              className={cn(
                'flex flex-col items-center justify-center gap-2 pb-4',
                // Le dernier groupe ne porte pas de filet : rien ne le suit.
                index < RAIL.length - 1 && 'mb-4 border-b border-[#ebebeb]',
              )}
            >
              <GroupLabel className="text-[10px] leading-[1.5] tracking-[0.4px]">
                {group.label}
              </GroupLabel>

              {/* Rayons concentriques : la pilule est a 12px, l'element actif
                  est en retrait de 2px (le py-0.5 du parent, et 2px de chaque
                  cote puisqu'il fait 36px dans une colonne large de 40), donc
                  10px. Les 12px que Figma pose aussi sur l'enfant laisseraient
                  son arrondi plus plat que celui qui l'entoure. */}
              {/* Le groupe selectionnable n'a pas de rembourrage vertical : le
                  retrait de 2px qui detache la pastille du bord est porte par
                  la pastille elle-meme. Chaque case garde ainsi 40px quel que
                  soit son etat, donc rien ne bouge quand la selection change,
                  et la colonne conserve la hauteur de la maquette. */}
              <div
                className={cn(
                  'flex flex-col items-center justify-center gap-1 rounded-[12px] bg-[#f0f0f0]',
                  !group.selectable && 'py-0.5',
                )}
              >
                {group.marks.map((mark, position) => {
                  const active = group.selectable && position === selected

                  const view = (
                    <RailMarkView mark={mark} className={active ? 'text-[#111]' : 'text-[#999]'} />
                  )
                  const box = 'relative flex size-10 items-center justify-center rounded-[10px]'

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

                  // Les groupes non selectionnables ne portent que des marques
                  // exportees de Figma : rien a activer, donc pas de bouton.
                  if (!group.selectable) {
                    return (
                      <div key={position} className={box}>
                        {view}
                      </div>
                    )
                  }

                  return (
                    <button
                      key={position}
                      type="button"
                      aria-pressed={active}
                      aria-label={'label' in mark ? mark.label : mark.alt}
                      onClick={() => setSelected(position)}
                      className={box}
                    >
                      {active && (
                        // Un seul et meme noeud pour toute la colonne : partage
                        // le `layoutId`, donc Framer Motion l'interpole de sa
                        // position precedente vers la nouvelle au lieu de le
                        // faire disparaitre ici et reapparaitre la.
                        <motion.span
                          layoutId="rail-highlight"
                          transition={transition}
                          className="absolute inset-[2px] rounded-[10px] border border-[#e6e6e6] bg-white"
                        />
                      )}
                      <span className="relative z-10">{view}</span>
                    </button>
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
    <div className="flex h-[39px] w-full items-center gap-1.5 rounded-[10px] bg-[#f0f0f0] py-2.5 pr-[7px] pl-3.5">
      <HugeiconsIcon
        icon={Search01Icon}
        size={17}
        strokeWidth={1.6}
        className="shrink-0 text-[#999]"
      />
      <input
        type="search"
        placeholder="Rechercher"
        className="min-w-0 flex-1 bg-transparent text-sm text-[#111] outline-none placeholder:text-[#999]"
      />
      <kbd className="flex size-[26px] shrink-0 items-center justify-center rounded-md border border-[#e6e6e6] bg-white text-xs font-medium text-[#333]">
        /
      </kbd>
    </div>
  )
}

interface MenuEntryProps {
  item: MenuItem
  active: boolean
  expanded: boolean
  /** Identite du calque anime, commune aux entrees d'un meme groupe. */
  layoutId: string
  /** Identite du repere de bord, commune aux entrees d'un meme groupe. */
  markLayoutId: string
  /** Identite du repere de sous-entree, commune aux feuilles d'un meme menu. */
  leafLayoutId: string
  /** Rang de la sous-entree active, ou `undefined` si aucune ne l'est. */
  activeLeaf?: number
  transition: Transition
  onSelect: () => void
  onSelectLeaf: (leaf: number) => void
}

function MenuEntry({
  item,
  active,
  expanded,
  layoutId,
  markLayoutId,
  leafLayoutId,
  activeLeaf,
  transition,
  onSelect,
  onSelectLeaf,
}: MenuEntryProps) {
  const { icon, label, children } = item

  return (
    <>
      <button
        type="button"
        aria-pressed={active}
        onClick={onSelect}
        // Le rembourrage ne depend pas de l'etat actif, seulement de la presence
        // d'un chevron : sinon l'entree decalerait son contenu en devenant
        // active, comme le faisait le rail avant d'etre corrige.
        className={cn(
          'relative flex h-[38px] w-full items-center gap-2 rounded-lg py-1.5 pl-3',
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
            className="absolute inset-0 rounded-lg border border-[#efefef] bg-white"
          />
        )}
        <HugeiconsIcon
          icon={icon}
          size={20}
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
      </button>

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
            {children.map((leaf, leafIndex) => {
              const leafActive = leafIndex === activeLeaf

              return (
                <button
                  key={leaf.label}
                  type="button"
                  aria-pressed={leafActive}
                  onClick={() => onSelectLeaf(leafIndex)}
                  className="relative flex h-8 w-full items-center py-1.5 pl-10"
                >
                  {/* Le filet gris est porte par chaque feuille : mis bout a bout
                      il forme la ligne continue du menu. */}
                  <span className="absolute top-0 left-[22px] h-full w-px bg-[#ebebeb]" />

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
                </button>
              )
            })}
          </motion.div>
        )}
      </AnimatePresence>
    </>
  )
}

/** Panneau de navigation, colonne de droite du duo. */
function Panel({ title }: { title: string }) {
  const transition = useSlideTransition()

  // Une seule entree active dans tout le panneau, reperee par son groupe et son
  // rang. Statique comme le rail : l'etat passera dans l'URL quand chaque
  // entree desservira un ecran.
  // `leaf` absent : l'entree elle-meme est active. `leaf` present : c'est une
  // sous-entree qui l'est, et son parent reste actif avec elle — une feuille
  // selectionnee sans son menu n'aurait pas de sens.
  const [selected, setSelected] = useState<{ group: number; item: number; leaf?: number }>({
    group: 0,
    item: 0,
  })

  // Les entrees depliees, reperees par groupe et rang. L'ensemble part de la
  // donnee, puis c'est le clic qui commande : une entree a enfants selectionne
  // et bascule son repli du meme geste.
  const [unfolded, setUnfolded] = useState(
    () =>
      new Set(
        MENU.flatMap((group, g) =>
          group.items.flatMap((item, i) => (item.expanded ? [`${g}:${i}`] : [])),
        ),
      ),
  )

  function select(group: number, item: number, foldable: boolean) {
    setSelected({ group, item })

    if (!foldable) return

    setUnfolded((prev) => {
      const next = new Set(prev)
      const key = `${group}:${item}`
      next.has(key) ? next.delete(key) : next.add(key)
      return next
    })
  }

  return (
    <div className="bg-surface flex h-full w-[248px] shrink-0 flex-col overflow-y-auto p-4">
      <div className="flex w-full flex-col gap-4">
        <div className="flex w-full flex-col gap-1">
          <div className="flex w-full items-center pb-3">
            <h2 className="font-heading text-xl font-medium text-[#1f1f1f] uppercase">{title}</h2>
          </div>
          <SearchField />
        </div>

        {MENU.map((group, groupIndex) => (
          <div
            key={group.label}
            className="flex w-full flex-col items-start gap-2 border-b border-[#ebebeb] pb-3"
          >
            <GroupLabel className="text-xs leading-[15.378px] tracking-[0.48px]">
              {group.label}
            </GroupLabel>

            <div className="flex w-full flex-col">
              {group.items.map((item, itemIndex) => (
                <MenuEntry
                  key={item.label}
                  item={item}
                  active={selected.group === groupIndex && selected.item === itemIndex}
                  activeLeaf={
                    selected.group === groupIndex && selected.item === itemIndex
                      ? selected.leaf
                      : undefined
                  }
                  expanded={unfolded.has(`${groupIndex}:${itemIndex}`)}
                  // Une identite par groupe : Framer Motion ne relie que deux
                  // noeuds de meme `layoutId`. La pastille glisse donc entre
                  // deux entrees d'un meme groupe, et se contente de disparaitre
                  // puis reapparaitre quand la selection change de groupe.
                  layoutId={`menu-highlight-${groupIndex}`}
                  markLayoutId={`menu-mark-${groupIndex}`}
                  // Un repere par menu deroulant : la barre glisse entre les
                  // feuilles d'un meme menu, et se contente d'apparaitre quand
                  // on passe d'un menu a l'autre.
                  leafLayoutId={`leaf-marker-${groupIndex}-${itemIndex}`}
                  transition={transition}
                  onSelect={() => select(groupIndex, itemIndex, Boolean(item.children))}
                  onSelectLeaf={(leaf) => setSelected({ group: groupIndex, item: itemIndex, leaf })}
                />
              ))}
            </div>
          </div>
        ))}
      </div>
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
  title: string
  user: User
}) {
  return (
    <div className="bg-surface flex h-screen w-full overflow-hidden">
      <Rail footer={<AccountButton user={user} />} />
      <Panel title={title} />
      {/* Colonne flex : le decollement de 16px vient de `PageFrame`, et la
          frame occupe tout le reste jusqu'au bas de la fenetre. */}
      <main className="bg-surface flex min-w-0 flex-1 flex-col overflow-hidden">{children}</main>
    </div>
  )
}
