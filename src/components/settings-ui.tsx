import type { ReactNode } from 'react'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

/**
 * Briques communes aux onglets de reglages.
 *
 * Le prefixe `-` du fichier le tient hors du routage : c'est un module de
 * composants, pas un ecran.
 */

/**
 * Carte de reglages : un bandeau gris qui nomme la section, un corps blanc qui
 * porte les champs. La coquille grise deborde de 4px, ce qui detache le corps
 * sans lui poser de bordure.
 */
export function Card({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: ReactNode
}) {
  return (
    <section className="flex w-full flex-col gap-1 rounded-[12px] bg-[#f3f4f4] p-1">
      <header className="flex flex-col gap-0.5 p-2">
        <h2 className="text-[18px] leading-[1.5] font-medium tracking-[-0.18px] text-[#1b1b1b]">
          {title}
        </h2>
        {description !== undefined && (
          <p className="text-[14px] leading-[1.5] text-[#73757c]">{description}</p>
        )}
      </header>
      <div className="flex w-full flex-col gap-3 rounded-[10px] bg-white p-2">{children}</div>
    </section>
  )
}

/** Intitule au-dessus, champ en dessous, message d'erreur sous les deux. */
export function Field({
  label,
  hint,
  error,
  children,
}: {
  label: string
  hint?: string
  error?: string
  children: ReactNode
}) {
  return (
    <div className="flex w-full flex-col gap-1.5">
      <p className="text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">{label}</p>
      {children}
      {hint !== undefined && error === undefined && (
        <p className="text-[13px] text-[#73757c]">{hint}</p>
      )}
      {error !== undefined && <p className="text-[13px] text-[#e5484d]">{error}</p>}
    </div>
  )
}

/** Habillage commun des champs de saisie. */
export const CHAMP =
  'h-auto w-full rounded-[12px] border-[#e8e8e9] bg-white px-3 py-2 text-[14px] leading-[1.5] text-[#1b1b1b] placeholder:text-[#73757c]'

/**
 * Un choix parmi quelques-uns, pose en boutons plutot qu'en liste deroulante.
 *
 * Trois ou quatre valeurs figees se lisent d'un coup d'oeil ; les replier dans
 * un menu demanderait un clic pour savoir ce qui est possible.
 */
export function Choices<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T
  options: { value: T; label: string; color?: string }[]
  onChange: (value: T) => void
}) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      {options.map((option) => {
        const choisi = option.value === value

        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={choisi}
            onClick={() => onChange(option.value)}
            className={cn(
              'flex h-12 min-w-[110px] flex-1 cursor-pointer items-center justify-center rounded-[12px] border p-3 text-[16px] transition-colors',
              choisi
                ? option.color === undefined
                  ? 'border-brand bg-brand text-white'
                  : 'border-transparent text-white'
                : 'border-[#e8e8e9] bg-white text-[#1b1b1b] hover:bg-[#f8f8f8]',
            )}
            style={choisi && option.color !== undefined ? { backgroundColor: option.color } : undefined}
          >
            {option.label}
          </button>
        )
      })}
    </div>
  )
}

/**
 * Les formulaires de reglages portent tous `noValidate`.
 *
 * Sans lui, la validation native du navigateur bloque l'envoi avant React des
 * qu'un `min` ou un `step` n'est pas respecte : l'utilisateur voit une bulle
 * du navigateur, dans la langue de celui-ci, et jamais le message de l'ecran —
 * qui ne s'affiche pas, puisque la soumission n'a pas eu lieu. Les attributs
 * restent sur les champs : ils bornent les fleches, ils n'arbitrent plus.
 */

/**
 * Barre d'enregistrement.
 *
 * La maquette n'en montre aucune : un ecran de reglages en a besoin, sans quoi
 * rien ne part jamais. Le bouton reste inerte tant que rien n'a change, pour
 * qu'un clic sans effet ne ressemble pas a un echec.
 */
export function SaveBar({
  dirty,
  pending,
  onCancel,
}: {
  dirty: boolean
  pending: boolean
  onCancel: () => void
}) {
  return (
    <div className="flex items-center justify-end gap-2">
      <Button type="button" variant="ghost" size="lg" onClick={onCancel} disabled={!dirty}>
        Annuler
      </Button>
      <Button type="submit" size="lg" disabled={pending || !dirty}>
        {pending ? 'Enregistrement…' : 'Enregistrer'}
      </Button>
    </div>
  )
}
