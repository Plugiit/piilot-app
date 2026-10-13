import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { ArrowDown01Icon, CustomerSupportIcon, DashboardSquare01Icon, Folder01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { AnimatePresence, motion } from 'framer-motion'
import { LogOut } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import logoUrl from '@/assets/sidebar/logo-mark.svg'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { portalSitesQuery } from '@/features/portal/api'
import { AudienceBand, defaultSite } from '@/features/portal/audience-band'
import { logout } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { User } from '@/types/api'

export interface PortalNavItem {
  to: '/client' | '/client/projets' | '/client/tickets'
  label: string
}

/**
 * Coquille du portail client.
 *
 * Une barre en haut et non un rail : le portail se consulte autant sur un
 * telephone, entre deux rendez-vous, que sur un ecran. Trois entrees de menu
 * ne justifient pas une colonne, et le contenu prend toute la largeur utile.
 *
 * La barre a la couleur de la barre laterale de l'agence (surface), comme le
 * fond de la page : elle s'y pose sans cadre. Le logo, les entrees en texte,
 * et a droite l'adresse du compte, qui ouvre son menu. Survol et etat actif
 * ont le meme style. Sur mobile, le menu passe sous la barre.
 *
 * Barre et pages prennent toute la largeur de la fenetre. Le tableau de bord
 * pose en plus sa feuille blanche bord a bord sur le fond de la marque.
 */
/** Une entree de la barre : discrete au repos, pleine au survol comme active. */
const NAV_ITEM =
  'font-heading flex h-7 items-center gap-1.5 rounded-[8px] px-2.5 text-[14px] leading-none font-medium whitespace-nowrap transition-colors'
const NAV_IDLE = 'text-white/40 hover:bg-white/10 hover:text-white'
const NAV_ACTIVE = 'bg-white/10 text-white'

/** L'icone de chaque section, a cote de son libelle. */
const NAV_ICONS: Record<PortalNavItem['to'], IconSvgElement> = {
  '/client': DashboardSquare01Icon,
  '/client/projets': Folder01Icon,
  '/client/tickets': CustomerSupportIcon,
}

export function PortalShell({ nav, user, children }: { nav: PortalNavItem[]; user: User; children: ReactNode }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  async function handleLogout() {
    // Meme filet que le back-office : on sort meme si le serveur ne repond pas.
    try {
      await logout()
    } catch {
      toast.error('La déconnexion n’a pas pu être confirmée par le serveur.')
    }
    queryClient.clear()
    void navigate({ to: '/login' })
  }

  const pathname = useRouterState({ select: (state) => state.location.pathname })
  // « Projets » couvre aussi la page d'un projet et celle d'un livrable.
  const section = pathname.startsWith('/client/tickets')
    ? '/client/tickets'
    : pathname === '/client' || pathname === '/client/'
      ? '/client'
      : '/client/projets'
  const active = (to: PortalNavItem['to']) => to === section
  const wide = section === '/client'

  // Le bandeau des sites. Sur la liste des projets : l'audience, le site le
  // plus visite d'abord, un selecteur pour les autres. Sur la page d'un
  // projet : son site seul, avec sa disponibilite ; sans statistiques, la
  // disponibilite seule. Un choix vaut pour la page ou il a ete fait.
  const projectPage = /^\/client\/projets(?:\/([^/]+))?\/?$/.exec(pathname)
  const { data: clientSites } = useQuery({ ...portalSitesQuery, enabled: projectPage !== null })
  const projectId = projectPage?.[1]
  const allSites = clientSites?.audience ?? []
  const sites = projectId === undefined ? allSites : allSites.filter((s) => s.project_ids.includes(projectId))
  const uptime = projectId === undefined ? [] : (clientSites?.uptime ?? []).filter((u) => u.project_id === projectId)
  const [choice, setChoice] = useState<{ path: string; site: string } | null>(null)
  const selected = (choice?.path === pathname ? choice.site : defaultSite(sites)) ?? ''
  const showAudience = projectPage !== null && (sites.length > 0 || uptime.length > 0)

  const links = nav.map(({ to, label }) => (
    <Link
      key={to}
      to={to}
      preload="intent"
      aria-current={active(to) ? 'page' : undefined}
      className={cn(NAV_ITEM, active(to) ? NAV_ACTIVE : NAV_IDLE)}
    >
      <HugeiconsIcon icon={NAV_ICONS[to]} size={16} strokeWidth={1.8} className="shrink-0" />
      {label}
    </Link>
  ))

  return (
    // La barre a la couleur de la marque, le fond aussi : chaque page s'y pose
    // en feuille aux coins hauts arrondis.
    <div className="bg-brand flex min-h-screen flex-col print:bg-white">
      <header className="bg-brand sticky top-0 z-30 print:hidden">
        <div className="flex h-10 items-center gap-2 px-4">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            {/* Le motif seul, sans son carre, centre dans la barre. */}
            <Link to="/client" aria-label="Tableau de bord" className="flex shrink-0 items-center">
              <img src={logoUrl} alt="Piilot" className="h-[22px] w-auto brightness-0 invert" />
            </Link>
            <nav aria-label="Navigation" className="hidden items-center gap-1 sm:flex">
              {links}
            </nav>
          </div>

          <DropdownMenu>
            <DropdownMenuTrigger
              aria-label="Mon compte"
              className={cn(NAV_ITEM, NAV_IDLE, 'min-w-0 cursor-pointer data-[state=open]:bg-white/10 data-[state=open]:text-white')}
            >
              <span className="max-w-[180px] truncate sm:max-w-[280px]">{user.email}</span>
              <HugeiconsIcon icon={ArrowDown01Icon} size={14} strokeWidth={1.8} className="shrink-0" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuLabel className="truncate text-sm font-medium">
                {user.firstname} {user.lastname}
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => void handleLogout()}>
                <LogOut />
                Déconnexion
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <nav aria-label="Navigation" className="flex h-10 items-center gap-1 border-t border-white/20 px-2.5 sm:hidden">
          {links}
        </nav>
      </header>

      {/* La barre s'ouvre en douceur sur le bandeau, et se referme de meme. Il
          defile avec la page : colle en haut, il mangerait l'ecran d'un
          telephone. */}
      <AnimatePresence initial={false}>
        {showAudience && (
          <motion.div
            key="audience"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 'auto', opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.35, ease: [0.22, 1, 0.36, 1] }}
            className="overflow-hidden"
          >
            <AudienceBand sites={sites} uptime={uptime} selected={selected} onSelect={(site) => setChoice({ path: pathname, site })} />
          </motion.div>
        )}
      </AnimatePresence>

      {/* La feuille, blanche, va jusqu'au bas de la fenetre. Le tableau de
          bord la veut sans marge, il place lui-meme ses colonnes. */}
      <main
        className={cn(
          'flex w-full flex-1 flex-col overflow-hidden rounded-t-[20px] print:rounded-none',
          'bg-white',
          !wide && 'px-4 pt-4 pb-8 sm:pt-5',
        )}
      >
        {children}
      </main>
    </div>
  )
}
