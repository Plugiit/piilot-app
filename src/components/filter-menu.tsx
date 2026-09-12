import { ArrowDown01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useId, useState, type ReactNode } from 'react'

import { HoverBackdrop } from '@/components/hover-menu'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useSlideTransition } from '@/lib/motion'
import { cn } from '@/lib/utils'

/**
 * Menu de filtre, partage par les ecrans qui en portent une barre.
 *
 * La liste des projets et l'ecran « Taches » ont la meme : recherche a gauche,
 * menus de filtre au milieu, creation au bout. Recopier le menu aurait fait
 * deux filtres qui se ressemblent jusqu'au premier ajustement.
 */

/**
 * Une entree de menu de filtre. `color` pose une pastille, `total` un compte.
 *
 * Le compte est facultatif : il supposait de connaitre tous les projets, ce
 * qu'une liste paginee par le serveur ne permet plus. Le retrouver couterait
 * une requete d'agregat par filtre — a faire le jour ou le tableau de bord en
 * aura besoin de toute facon.
 */
export interface Option {
  value: string
  label: string
  color?: string
  total?: number
}

/**
 * Entree de menu dont le fond de survol glisse depuis l'entree precedente.
 *
 * Le fond natif de Radix est neutralise (`focus:bg-transparent`) et remplace
 * par un noeud unique partage entre toutes les entrees d'un meme menu : c'est
 * le `layoutId` qui le fait glisser plutot que disparaitre ici et reapparaitre
 * la — la meme mecanique que la pastille des onglets du graphique de charge.
 *
 * `onFocus` autant que `onPointerEnter` : Radix deplace le focus a la fleche
 * du clavier, et le fond doit suivre la navigation au clavier comme il suit le
 * curseur.
 *
 * L'indicateur de selection est remonte au-dessus du fond par son `data-slot` :
 * pose apres lui dans le DOM, le fond opaque le masquerait.
 */
function HoverItem({
  value,
  layoutId,
  hovered,
  onHover,
  transition,
  children,
}: {
  value: string
  layoutId: string
  hovered: string | null
  onHover: (value: string) => void
  transition: ReturnType<typeof useSlideTransition>
  children: ReactNode
}) {
  return (
    <DropdownMenuRadioItem
      value={value}
      onPointerEnter={() => onHover(value)}
      onFocus={() => onHover(value)}
      className="relative text-[13px] focus:bg-transparent [&>[data-slot=dropdown-menu-radio-item-indicator]]:z-10"
    >
      {hovered === value && <HoverBackdrop layoutId={layoutId} transition={transition} />}

      {/* Au-dessus du fond : sans quoi le libelle disparaitrait derriere lui
          pendant le glissement. */}
      <span className="relative z-10 flex w-full items-center gap-1.5">{children}</span>
    </DropdownMenuRadioItem>
  )
}

/**
 * Filtre en menu deroulant.
 *
 * Le declencheur porte la valeur choisie et non le nom du champ : « Client »
 * seul obligerait a ouvrir le menu pour savoir sur quoi la liste est filtree.
 * Sans selection il porte le nom, ce qui dit ce qu'il ouvrira.
 *
 * Un groupe radio et non des cases : un projet n'a qu'un statut, et le multi-
 * selection demanderait de rendre « Client : 3 sélectionnés » lisible dans un
 * bouton de 140px. Il viendra si le besoin se presente.
 */
export function FilterMenu({
  name,
  all,
  value,
  options,
  onChange,
}: {
  name: string
  /** Libelle de l'entree qui leve le filtre. */
  all: string
  value: string | undefined
  options: Option[]
  onChange: (value: string | undefined) => void
}) {
  const selected = options.find((option) => option.value === value)

  // Un identifiant par instance : trois menus partageant un `layoutId` verraient
  // leur fond tenter de glisser de l'un a l'autre.
  const layoutId = useId()
  const [hovered, setHovered] = useState<string | null>(null)
  const transition = useSlideTransition()

  return (
    <DropdownMenu onOpenChange={(open) => !open && setHovered(null)}>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="lg"
          className={cn('gap-1.5 text-[13px]', selected && 'border-[#d4d4d4]')}
        >
          {selected?.color && (
            <span
              aria-hidden
              className="size-1.5 shrink-0 rounded-full"
              style={{ backgroundColor: selected.color }}
            />
          )}

          <span className={selected ? 'text-[#111]' : 'text-[#64748b]'}>
            {selected?.label ?? name}
          </span>

          <HugeiconsIcon
            icon={ArrowDown01Icon}
            size={14}
            strokeWidth={2}
            className="text-[#8d8d8d]"
          />
        </Button>
      </DropdownMenuTrigger>

      {/* `align="start"` : le menu tombe sous le bord gauche de son bouton, donc
          la colonne de libelles reste alignee avec ce qu'on vient de lire. */}
      {/* Le fond de survol s'efface des que le curseur quitte le menu : un
          survol qui resterait affiche apres coup designerait une entree que
          plus personne ne pointe. */}
      <DropdownMenuContent
        align="start"
        className="min-w-[200px]"
        onPointerLeave={() => setHovered(null)}
      >
        <DropdownMenuRadioGroup
          value={value ?? ''}
          onValueChange={(next) => onChange(next === '' ? undefined : next)}
        >
          <HoverItem
            value=""
            layoutId={layoutId}
            hovered={hovered}
            onHover={setHovered}
            transition={transition}
          >
            {all}
          </HoverItem>

          <DropdownMenuSeparator />

          {options.map((option) => (
            <HoverItem
              key={option.value}
              value={option.value}
              layoutId={layoutId}
              hovered={hovered}
              onHover={setHovered}
              transition={transition}
            >
              {option.color && (
                <span
                  aria-hidden
                  className="size-1.5 shrink-0 rounded-full"
                  style={{ backgroundColor: option.color }}
                />
              )}
              <span className="truncate">{option.label}</span>
              {/* `!` sur la couleur : Radix repeint tous les descendants de
                  l'entree focalisee, ce qui ecraserait ce gris. */}
              {option.total !== undefined && (
                <span className="ml-auto pl-2 text-[12px] text-[#8d8d8d]! tabular-nums">
                  {option.total}
                </span>
              )}
            </HoverItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
