import { motion } from 'framer-motion'
import { createContext, useContext, useId, useState, type ReactNode } from 'react'

import { DropdownMenuContent, DropdownMenuItem } from '@/components/ui/dropdown-menu'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'

/**
 * Menu deroulant dont le fond de survol glisse d'une entree a l'autre.
 *
 * Le fond natif de Radix apparait et disparait sur place ; celui-ci est un
 * noeud unique partage par toutes les entrees d'un meme menu, que Framer Motion
 * deplace de l'une vers l'autre — la meme mecanique que la pastille des
 * onglets du graphique de charge.
 *
 * L'etat passe par un contexte plutot que par des props : une entree n'a pas a
 * savoir qu'elle participe a un fond partage, et le menu n'a pas a etiqueter
 * ses entrees pour les distinguer.
 */
const HoverMenuContext = createContext<{
  layoutId: string
  hovered: string | null
  setHovered: (id: string | null) => void
  transition: ReturnType<typeof useSlideTransition>
} | null>(null)

/** Fond de survol glissant, partage par les entrees d'un meme menu. */
export function HoverBackdrop({
  layoutId,
  transition,
  color = '#f4f4f4',
  className,
}: {
  layoutId: string
  transition: ReturnType<typeof useSlideTransition>
  color?: string
  className?: string
}) {
  return (
    <motion.span
      aria-hidden
      layoutId={layoutId}
      transition={transition}
      // La couleur passe par `style` et non par `animate` : le noeud est
      // demonte puis remonte a chaque changement d'entree, et Framer n'applique
      // les valeurs de `animate` qu'apres le montage — le premier frame
      // sortirait sans fond, ce qui fait clignoter le blanc du menu au travers.
      style={{ backgroundColor: color }}
      className={cn('absolute inset-0 rounded-md', className)}
    />
  )
}

export function HoverMenuContent({
  children,
  ...props
}: React.ComponentProps<typeof DropdownMenuContent>) {
  const layoutId = useId()
  const [hovered, setHovered] = useState<string | null>(null)
  const transition = useSlideTransition()

  return (
    <HoverMenuContext.Provider value={{ layoutId, hovered, setHovered, transition }}>
      {/* Le fond s'efface des que le curseur quitte le menu : un survol qui
          resterait affiche designerait une entree que plus personne ne
          pointe. */}
      <DropdownMenuContent onPointerLeave={() => setHovered(null)} {...props}>
        {children}
      </DropdownMenuContent>
    </HoverMenuContext.Provider>
  )
}

/**
 * Entree de menu qui participe au fond glissant.
 *
 * `onFocus` autant que `onPointerEnter` : Radix deplace le focus aux fleches du
 * clavier, et le fond doit suivre la navigation au clavier comme le curseur.
 */
export function HoverMenuItem({
  danger,
  children,
  className,
  ...props
}: React.ComponentProps<typeof DropdownMenuItem> & { danger?: boolean }) {
  const menu = useContext(HoverMenuContext)
  const id = useId()

  if (menu === null) throw new Error('HoverMenuItem doit vivre dans un HoverMenuContent')

  return (
    <DropdownMenuItem
      onPointerEnter={() => menu.setHovered(id)}
      onFocus={() => menu.setHovered(id)}
      className={cn(
        'relative text-[13px] focus:bg-transparent',
        danger ? 'text-[#e5484d] focus:text-[#e5484d]' : 'text-[#111]',
        className,
      )}
      {...props}
    >
      {menu.hovered === id && (
        <HoverBackdrop
          layoutId={menu.layoutId}
          transition={menu.transition}
          color={danger ? '#fdecec' : '#f4f4f4'}
        />
      )}

      {/* Au-dessus du fond : sans quoi le libelle disparaitrait derriere lui
          pendant le glissement. */}
      <span className="relative z-10 flex w-full items-center gap-2">{children}</span>
    </DropdownMenuItem>
  )
}

/** Ce qu'une entree de menu affiche quand elle porte une pastille de couleur. */
export function MenuDot({ color }: { color: string }) {
  return (
    <span
      aria-hidden
      className="size-1.5 shrink-0 rounded-full"
      style={{ backgroundColor: color }}
    />
  )
}

export type { ReactNode }
