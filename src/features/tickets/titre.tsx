import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'

import { useRenameTicket } from '@/features/tickets/api'
import { HttpError } from '@/lib/api'

/**
 * Titre d'un ticket, modifiable sur place.
 *
 * Le champ occupe exactement la boite du texte qu'il remplace : c'est la seule
 * facon d'ouvrir l'edition sans que le reste de l'ecran bouge d'un pixel.
 *
 * La technique est celle du champ auto-dimensionne : le texte et la zone de
 * saisie sont poses dans la MEME cellule de grille. Le texte, rendu invisible,
 * continue de donner sa taille a la cellule ; la zone s'y etale. Comme les deux
 * portent la meme typographie, la boite ne change pas — et quand on tape, le
 * texte fantome grandit et la zone suit, y compris sur plusieurs lignes.
 *
 * Un `<textarea>` et non un `<input>` : un sujet long se replie sur deux ou
 * trois lignes, ce qu'un `input` refuse de faire.
 */
export function TitreModifiable({ ticketId, subject }: { ticketId: string; subject: string }) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(subject)

  const zone = useRef<HTMLTextAreaElement>(null)
  const rename = useRenameTicket(ticketId)

  // Annulation demandee : la perte de focus qui suit ne doit pas enregistrer.
  const annule = useRef(false)

  // Le titre a pu changer ailleurs — un autre onglet, une invalidation de
  // cache. Tant qu'on n'edite pas, le brouillon suit.
  useEffect(() => {
    if (!editing) setDraft(subject)
  }, [subject, editing])

  function start() {
    setDraft(subject)
    setEditing(true)
  }

  useEffect(() => {
    if (!editing) return

    const element = zone.current
    if (element === null) return

    element.focus()
    // Tout selectionner : on ouvre un titre pour le refaire bien plus souvent
    // que pour y corriger une lettre.
    element.select()
  }, [editing])

  /**
   * Enregistre, une fois et une seule.
   *
   * Toutes les sorties passent par ici, et par la perte de focus uniquement :
   * `Entree` se contente de retirer le focus. Sans cela, valider au clavier
   * declenchait DEUX enregistrements — celui de la touche, puis celui du blur
   * provoque par la fermeture du champ —, qui partaient avant que le premier
   * n'ait repondu. Les deux lisaient donc l'ancien titre et inscrivaient chacun
   * sa ligne au journal.
   */
  function commit() {
    if (rename.isPending) return

    if (annule.current) {
      annule.current = false
      setDraft(subject)
      setEditing(false)
      return
    }

    const valeur = draft.trim()

    // Vide : on ne peut pas enregistrer, et laisser le champ ouvert bloquerait
    // l'ecran. On revient au titre d'avant.
    if (valeur === '' || valeur === subject) {
      setDraft(subject)
      setEditing(false)
      return
    }

    rename.mutate(
      { subject: valeur },
      {
        onSuccess: () => {
          setEditing(false)
          toast.success('Ticket renommé')
        },
        onError: (error) => {
          toast.error(error instanceof HttpError ? error.message : 'Renommage impossible')
          setDraft(subject)
          setEditing(false)
        },
      },
    )
  }

  if (!editing) {
    return (
      <button
        type="button"
        onClick={start}
        title="Cliquer pour renommer"
        // Ni `flex-1` ni largeur imposee : le bouton se dimensionne sur son
        // texte, comme le faisait le titre avant d'etre modifiable. Le cadre
        // d'edition fait de meme, d'ou l'absence de saut entre les deux.
        className="-mx-1 min-w-0 cursor-text rounded-[6px] px-1 text-left font-[inherit] text-[length:inherit] leading-[inherit] hover:bg-[#e8e8e9]"
      >
        {subject}
      </button>
    )
  }

  return (
    // La cellule unique : le fantome et la zone s'y superposent.
    <span className="-mx-1 grid min-w-0">
      <span
        aria-hidden
        className="invisible px-1 [grid-area:1/1] font-[inherit] text-[length:inherit] leading-[inherit] whitespace-pre-wrap"
      >
        {/* L'espace final evite que la cellule perde une ligne au moment ou
            l'on tape un retour a la ligne. */}
        {draft}{' '}
      </span>

      <textarea
        ref={zone}
        value={draft}
        rows={1}
        disabled={rename.isPending}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={commit}
        onKeyDown={(event) => {
          // Les deux touches retirent le focus, et c'est le blur qui tranche :
          // un seul chemin de sortie, donc un seul enregistrement possible.
          if (event.key === 'Enter') {
            // Valide plutot que d'inserer une ligne : un titre n'en a pas.
            event.preventDefault()
            zone.current?.blur()
          }

          if (event.key === 'Escape') {
            event.preventDefault()
            annule.current = true
            zone.current?.blur()
          }
        }}
        aria-label="Sujet du ticket"
        className="resize-none overflow-hidden rounded-[6px] bg-white px-1 [grid-area:1/1] font-[inherit] text-[length:inherit] leading-[inherit] outline-2 outline-[#ff782b]"
      />
    </span>
  )
}
