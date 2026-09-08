import { motion } from 'framer-motion'
import { Checkbox as CheckboxPrimitive } from 'radix-ui'
import * as React from 'react'

import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'

/**
 * Case a cocher shadcn, dont la coche se trace au lieu d'apparaitre.
 *
 * Deux ecarts avec le composant tel que la CLI le pose :
 *
 * L'icone de lucide cede la place a un trace anime — `pathLength` va de 0 a 1,
 * ce qui dessine la coche du talon vers la pointe. Une icone posee d'un bloc
 * change l'etat sans qu'on voie lequel des deux sens on vient de prendre.
 *
 * `forceMount` maintient l'indicateur dans l'arbre : Radix le demonte sinon des
 * qu'on decoche, et le trace inverse n'aurait pas le temps de se jouer.
 *
 * L'etat est suivi ici plutot que lu dans le DOM, pour que la case anime aussi
 * bien controlee que non controlee.
 */
function Checkbox({
  className,
  checked,
  defaultChecked,
  onCheckedChange,
  ...props
}: React.ComponentProps<typeof CheckboxPrimitive.Root>) {
  const [uncontrolled, setUncontrolled] = React.useState(defaultChecked ?? false)
  const state = checked ?? uncontrolled

  // L'etat indetermine n'est pas trace : il n'a pas de coche a dessiner.
  const drawn = state === true

  const transition = useSlideTransition()

  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      checked={checked}
      defaultChecked={defaultChecked}
      onCheckedChange={(next) => {
        setUncontrolled(next === true)
        onCheckedChange?.(next)
      }}
      className={cn(
        'peer relative flex size-4 shrink-0 items-center justify-center rounded-[4px] border border-input transition-colors outline-none group-has-disabled/field:opacity-50 group-has-[:focus-visible]/field-label:ring-0 group-has-[:focus-visible]/field-label:not-data-checked:border-input after:absolute after:-inset-x-3 after:-inset-y-2 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 aria-invalid:aria-checked:border-primary dark:bg-input/30 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 data-checked:border-primary data-checked:bg-primary data-checked:text-primary-foreground group-has-[:focus-visible]/field-label:data-checked:border-primary dark:data-checked:bg-primary',
        className,
      )}
      {...props}
    >
      <CheckboxPrimitive.Indicator
        forceMount
        data-slot="checkbox-indicator"
        className="grid place-content-center text-current"
      >
        <svg viewBox="0 0 14 14" fill="none" className="size-3.5">
          <motion.path
            d="M3 7.2 6 10.2 11 4.2"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
            // `initial={false}` : une case deja cochee au premier rendu doit
            // s'afficher cochee, pas se tracer sous les yeux au chargement.
            initial={false}
            animate={{ pathLength: drawn ? 1 : 0, opacity: drawn ? 1 : 0 }}
            transition={transition}
          />
        </svg>
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  )
}

export { Checkbox }
