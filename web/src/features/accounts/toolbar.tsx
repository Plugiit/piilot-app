import { Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { createContext, useContext, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

import { Input } from '@/components/ui/input'

/**
 * Barre d'outils de l'ecran « Comptes et rôles ».
 *
 * La barre est au chassis, mais la recherche et les filtres appartiennent a
 * l'onglet « Comptes » : il les y projette par un portail. Le contexte porte
 * l'element cible, pose par le chassis une fois monte.
 *
 * Hors du fichier de route : le decoupage du routeur deplace le composant de
 * la route dans un module a part, et un contexte declare a cote de lui y
 * serait recopie — deux contextes, et un portail qui ne trouve rien.
 */
export const AccountsToolbarSlot = createContext<HTMLDivElement | null>(null)

/** Place `children` dans la barre d'outils du chassis, a gauche d'« Inviter ». */
export function AccountsToolbar({ children }: { children: ReactNode }) {
  const slot = useContext(AccountsToolbarSlot)
  if (slot === null) return null
  return createPortal(children, slot)
}

/** Champ de recherche de la barre, le meme sur les trois onglets. */
export function ToolbarSearch({
  value,
  onChange,
  placeholder,
  label,
}: {
  value: string
  onChange: (value: string) => void
  placeholder: string
  label: string
}) {
  return (
    <div className="relative min-w-[200px] flex-1">
      <HugeiconsIcon
        icon={Search01Icon}
        size={16}
        strokeWidth={1.6}
        className="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-[#8d8d8d]"
      />
      <Input
        type="search"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        aria-label={label}
        className="h-9 pl-8 text-[13px]"
      />
    </div>
  )
}

/** Vrai si l'un des champs contient la recherche, sans tenir compte de la casse ni des accents. */
export function matchesSearch(query: string, ...fields: (string | null | undefined)[]): boolean {
  const q = fold(query)
  if (q === '') return true
  return fields.some((field) => field != null && fold(field).includes(q))
}

function fold(value: string): string {
  return value
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
}
