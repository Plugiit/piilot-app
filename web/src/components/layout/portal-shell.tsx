import { useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { CustomerSupportIcon, DashboardSquare01Icon, Folder01Icon, PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { LogOut } from 'lucide-react'
import type { ReactNode } from 'react'
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
import { logout } from '@/lib/auth'
import { userInitials } from '@/lib/initials'
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
 * fond de la page : elle s'y pose sans cadre. Le logo, un filet, les
 * entrees en texte — l'active dans la couleur de la marque —, et a droite les
 * gestes en boutons carres. Sur mobile, le menu passe sous la barre.
 *
 * Le tableau de bord prend toute la largeur de la fenetre : trois colonnes de
 * cartes, la ou une page de projet se lit en une colonne bornee.
 */
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

  const links = nav.map(({ to, label }) => (
    <Link
      key={to}
      to={to}
      preload="intent"
      aria-current={active(to) ? 'page' : undefined}
      className={cn(
        'font-heading flex h-7 items-center gap-1.5 rounded-[8px] px-2.5 text-[14px] leading-none font-medium whitespace-nowrap transition-colors',
        active(to) ? 'bg-white/10 text-white' : 'text-white/40 hover:text-white/70',
      )}
    >
      <HugeiconsIcon icon={NAV_ICONS[to]} size={16} strokeWidth={1.8} className="shrink-0" />
      {label}
    </Link>
  ))

  return (
    // Essai : la barre a la couleur de la marque. Sur le tableau de bord, le
    // fond aussi, pour que les coins arrondis de la feuille blanche s'y posent.
    <div className={cn('flex min-h-screen flex-col print:bg-white', wide ? 'bg-brand' : 'bg-surface')}>
      <header className="bg-brand sticky top-0 z-30 print:hidden">
        <div className={cn('mx-auto flex h-10 items-center gap-2 px-4', wide ? 'max-w-none' : 'max-w-[1040px]')}>
          <div className="flex min-w-0 flex-1 items-center gap-3">
            {/* Le motif seul, sans son carre, centre dans la barre. */}
            <Link to="/client" aria-label="Tableau de bord" className="flex shrink-0 items-center">
              <img src={logoUrl} alt="Piilot" className="h-[22px] w-auto brightness-0 invert" />
            </Link>
            <span aria-hidden className="hidden h-4 w-px bg-white/30 sm:block" />
            <nav aria-label="Navigation" className="hidden items-center gap-1 sm:flex">
              {links}
            </nav>
          </div>

          <Link
            to="/client/tickets/nouvelle"
            aria-label="Nouvelle demande"
            title="Nouvelle demande"
            className="flex size-7 shrink-0 items-center justify-center rounded-[8px] border border-white/25 bg-white/15 text-white transition-colors hover:bg-white/25"
          >
            <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={1.8} />
          </Link>

          <DropdownMenu>
            <DropdownMenuTrigger
              aria-label="Mon compte"
              className="text-brand flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-[8px] bg-white text-[11px] font-semibold transition-opacity hover:opacity-90"
            >
              {userInitials(user)}
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
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
        </div>

        <nav aria-label="Navigation" className="flex h-10 items-center gap-1 border-t border-white/20 px-2.5 sm:hidden">
          {links}
        </nav>
      </header>

      {/* Le tableau de bord pose sa surface blanche bord a bord, jusqu'au bas
          de la fenetre : pas de marge autour, la page s'en charge. */}
      <main className={cn('mx-auto flex w-full flex-1 flex-col', wide ? 'max-w-none' : 'max-w-[1040px] px-4 pt-2 pb-8 sm:pt-4')}>
        {children}
      </main>
    </div>
  )
}
