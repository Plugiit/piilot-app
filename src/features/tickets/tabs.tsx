import { motion } from 'framer-motion'
import { useId } from 'react'

import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'

/**
 * Onglets pilotes par un etat, et non par la route.
 *
 * `TabBar` fait le meme geste pour les ecrans, mais ses onglets sont des liens :
 * une adresse par vue. Ici les deux usages — la vue du registre, le
 * destinataire d'une entree — ne meritent pas d'entree d'historique, et le
 * second ne peut meme pas en avoir : c'est un champ de formulaire.
 *
 * Le filet est un noeud unique porte par `layoutId` : Framer le glisse d'un
 * onglet a l'autre au lieu de l'effacer ici pour le repeindre la. Le ressort
 * est celui de tout le reste de l'app, pour qu'aucun mouvement voisin ne
 * detonne.
 */
export function SlideTabs<T extends string>({
  value,
  onChange,
  options,
  label,
  className,
}: {
  value: T
  onChange: (value: T) => void
  options: { value: T; label: string }[]
  /** Ce que le groupe est, pour qui ne voit pas l'ecran. */
  label: string
  className?: string
}) {
  const transition = useSlideTransition()

  // Unique par instance : deux barres montees ensemble verraient sinon leur
  // filet sauter de l'une a l'autre.
  const filet = useId()

  return (
    <div role="tablist" aria-label={label} className={cn('flex gap-0.5', className)}>
      {options.map((option) => {
        const active = option.value === value

        return (
          <button
            key={option.value}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onChange(option.value)}
            className={cn(
              'relative cursor-pointer rounded-t-[8px] px-3 pt-[7px] pb-[9px] text-[13px] transition-colors',
              active ? 'font-medium text-[#1b1b1b]' : 'text-[#73757c] hover:text-[#1b1b1b]',
            )}
          >
            {option.label}

            {active && (
              <motion.span
                aria-hidden
                layoutId={filet}
                transition={transition}
                className="bg-brand absolute right-2.5 -bottom-px left-2.5 h-0.5 rounded-full"
              />
            )}
          </button>
        )
      })}
    </div>
  )
}
