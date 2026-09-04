import { useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { LogOut } from 'lucide-react'
import type { ComponentType, ReactNode } from 'react'

import { Button } from '@/components/ui/button'
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
    <div className="flex h-screen w-full overflow-hidden bg-surface">
      <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-surface-raised">
        <div className="flex h-14 flex-col justify-center px-4">
          <span className="font-semibold text-ink">Plugiit</span>
          <span className="text-[11px] uppercase tracking-wide text-ink-muted">{area}</span>
        </div>

        <nav className="flex-1 space-y-0.5 px-2">
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
              activeOptions={{ exact: to === '/admin' || to === '/client' }}
              className="flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm text-ink-soft transition-colors hover:bg-surface-sunken hover:text-ink"
              activeProps={{ className: cn('bg-brand-50 text-brand-600 hover:bg-brand-50') }}
            >
              <Icon className="size-4 shrink-0" />
              {label}
            </Link>
          ))}
        </nav>

        <div className="space-y-1 border-t border-border p-2">
          <p className="truncate px-2.5 text-xs text-ink-muted">
            {user.firstname} {user.lastname}
          </p>
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-start"
            onClick={() => void handleLogout()}
          >
            <LogOut className="size-4" />
            Déconnexion
          </Button>
        </div>
      </aside>

      <main className="min-w-0 flex-1 overflow-hidden">{children}</main>
    </div>
  )
}
