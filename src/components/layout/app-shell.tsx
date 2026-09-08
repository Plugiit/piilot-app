import { useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { LogOut } from 'lucide-react'
import type { ComponentType, ReactNode } from 'react'

import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Separator } from '@/components/ui/separator'
import { logout } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { User } from '@/types/api'

export interface NavItem {
  to: string
  label: string
  icon: ComponentType<{ className?: string }>
}

interface AppShellProps {
  children: ReactNode
  nav: NavItem[]
  user: User
  /** Libelle de l'espace courant : « Agence » ou « Espace client ». */
  area: string
}

/** Initiales servant de repli quand aucun avatar n'est defini. */
function initials(user: User): string {
  const letters = `${user.firstname.at(0) ?? ''}${user.lastname.at(0) ?? ''}`.trim()
  return letters === '' ? user.email.slice(0, 2).toUpperCase() : letters.toUpperCase()
}

/**
 * Coquille commune aux deux espaces : rail de navigation fixe a gauche,
 * contenu a droite.
 *
 * L'application sert le back-office et le portail client ; seuls la
 * navigation et le libelle d'espace changent. Le reste — hauteur bornee,
 * deconnexion, identite — est identique, et c'est ce qui justifie une seule
 * application plutot que deux.
 *
 * La hauteur est bornee a la fenetre (`h-screen` + `overflow-hidden`) et
 * chaque vue gere son propre defilement. Sans cette contrainte, un tableau
 * long ferait defiler l'en-tete et la navigation avec lui.
 */
export function AppShell({ children, nav, user, area }: AppShellProps) {
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
    <div className="bg-sidebar flex h-screen w-full overflow-hidden">
      <aside className="bg-sidebar flex w-56 shrink-0 flex-col border-r">
        <div className="flex h-14 flex-col justify-center px-4">
          <span className="font-semibold">Plugiit</span>
          <span className="text-muted-foreground text-[11px] tracking-wide uppercase">{area}</span>
        </div>

        <Separator />

        <nav className="flex-1 space-y-0.5 p-2">
          {nav.map(({ to, label, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              // Le prefetch au survol est ce qui rend la navigation instantanee :
              // le loader de la route cible s'execute pendant que le curseur
              // parcourt encore la distance jusqu'au lien.
              preload="intent"
              // Sans `exact`, l'accueil d'un espace reste actif sur toutes ses
              // sous-routes, puisque son chemin en est le prefixe.
              activeOptions={{ exact: to === '/client' }}
              className="text-muted-foreground hover:bg-accent hover:text-accent-foreground flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors"
              activeProps={{
                className: cn('bg-accent text-accent-foreground font-medium'),
              }}
            >
              <Icon className="size-4 shrink-0" />
              {label}
            </Link>
          ))}
        </nav>

        <Separator />

        <div className="p-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="lg" className="w-full justify-start gap-2 px-2">
                <Avatar className="size-6">
                  <AvatarFallback className="text-[10px]">{initials(user)}</AvatarFallback>
                </Avatar>
                <span className="truncate">
                  {user.firstname} {user.lastname}
                </span>
              </Button>
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
        </div>
      </aside>

      <main className="bg-background min-w-0 flex-1 overflow-hidden">{children}</main>
    </div>
  )
}
