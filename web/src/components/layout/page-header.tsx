import type { ReactNode } from 'react'

interface PageHeaderProps {
  title: string
  subtitle?: string
  actions?: ReactNode
}

/**
 * En-tete de page. `shrink-0` est delibere : dans une colonne flex a hauteur
 * fixe, l'en-tete garde sa taille et c'est la zone de contenu qui absorbe le
 * reste et defile.
 */
export function PageHeader({ title, subtitle, actions }: PageHeaderProps) {
  return (
    <header className="border-sidebar-border flex shrink-0 items-center justify-between gap-4 border-b px-4 py-3 sm:px-6">
      <div className="min-w-0">
        <h1 className="truncate text-lg font-semibold">{title}</h1>
        {subtitle && <p className="text-muted-foreground truncate text-[13px]">{subtitle}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </header>
  )
}
