import { useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { LogOut } from 'lucide-react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'

import logoUrl from '@/assets/sidebar/logo.svg'
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

export interface PortalNavItem {
  to: '/client' | '/client/tickets'
  label: string
}

/** Initiales, a defaut de photo. */
function initials(user: User): string {
  const letters = `${user.firstname.at(0) ?? ''}${user.lastname.at(0) ?? ''}`.trim()

  return letters === '' ? user.email.slice(0, 2).toUpperCase() : letters.toUpperCase()
}

/**
 * Coquille du portail client.
 *
 * Une barre en haut et non un rail : le portail se consulte autant sur un
 * telephone, entre deux rendez-vous, que sur un ecran. Deux entrees de menu ne
 * justifient pas une colonne, et le contenu prend toute la largeur utile.
 *
 * Sur mobile, le menu passe sous la barre, en onglets : il reste visible sans
 * menu a deplier, puisqu'il n'y a que deux destinations.
 */
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
  // « Projets » couvre aussi la page d'un projet et celle d'un livrable : tout
  // ce qui n'est pas le support.
  const active = (to: PortalNavItem['to']) =>
    to === '/client/tickets' ? pathname.startsWith('/client/tickets') : !pathname.startsWith('/client/tickets')

  const links = nav.map(({ to, label }) => (
    <Link
      key={to}
      to={to}
      preload="intent"
      aria-current={active(to) ? 'page' : undefined}
      className={cn(
        'relative flex h-full items-center px-1 text-[14px] transition-colors',
        active(to)
          ? 'font-medium text-[#1b1b1b] after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:rounded-full after:bg-brand'
          : 'text-[#73757c] hover:text-[#1b1b1b]',
      )}
    >
      {label}
    </Link>
  ))

  return (
    <div className="flex min-h-screen flex-col bg-[#f8f8f8]">
      <header className="sticky top-0 z-30 border-b border-[#e8e8e9] bg-white">
        <div className="mx-auto flex h-14 max-w-[1040px] items-center gap-6 px-4">
          <Link to="/client" className="flex shrink-0 items-center gap-2">
            <img src={logoUrl} alt="" className="size-7" />
            <span className="flex flex-col leading-tight">
              <span className="text-[15px] font-semibold text-[#1b1b1b]">Piilot</span>
              <span className="text-[11px] tracking-wide text-[#8d8d8d] uppercase">Espace client</span>
            </span>
          </Link>

          <nav aria-label="Navigation" className="hidden h-full items-stretch gap-5 sm:flex">
            {links}
          </nav>

          <DropdownMenu>
            <DropdownMenuTrigger
              aria-label="Mon compte"
              className="ml-auto flex cursor-pointer items-center gap-2 rounded-full py-1 pr-1 pl-3 transition-colors hover:bg-[#f3f4f4]"
            >
              <span className="hidden text-[14px] text-[#1b1b1b] sm:inline">{user.firstname}</span>
              <span className="flex size-8 items-center justify-center rounded-full bg-[#1b1b1b] text-[12px] font-medium text-white">
                {initials(user)}
              </span>
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

        <nav
          aria-label="Navigation"
          className="flex h-11 items-stretch gap-5 border-t border-[#f0f0f1] px-4 sm:hidden"
        >
          {links}
        </nav>
      </header>

      <main className="mx-auto w-full max-w-[1040px] flex-1 px-4 py-6 sm:py-8">{children}</main>
    </div>
  )
}
