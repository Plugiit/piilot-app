import { Message01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useReducedMotion } from 'framer-motion'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { peopleQuery } from '@/features/projects/api'
import { usePostTicketMessage } from '@/features/tickets/api'
import {
  TICKET_PRIORITY,
  TICKET_PRIORITY_ORDER,
  TICKET_STATUS,
  TICKET_STATUS_ORDER,
} from '@/features/tickets/format'
import { SlideTabs } from '@/features/tickets/tabs'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { Person, TicketDetail, TicketPriority, TicketStatus } from '@/types/api'

/**
 * Valeur du menu d'assignation qui remet le ticket a prendre.
 *
 * Une chaine plutot que `null` : un `Select` ne sait pas porter une valeur
 * nulle, il afficherait son marque-page a la place de l'intitule.
 */
const PERSONNE = 'personne'

type Destinataire = 'client' | 'interne'

/** Nom affichable d'un compte, l'un des deux champs pouvant etre vide. */
function nameOf(person: Person): string {
  return `${person.firstname} ${person.lastname}`.trim() || 'Sans nom'
}

/**
 * Le redacteur.
 *
 * Replie par defaut, derriere un bouton « Répondre ». Un champ de saisie
 * toujours ouvert en bas d'un fil dit qu'on attend une reponse, alors que la
 * plupart des ouvertures d'un ticket servent a le relire.
 *
 * Deplie, il porte aussi les trois changements que l'entree peut annoncer :
 * statut, priorite, assignation. C'est le parti de l'ecran — repondre et faire
 * avancer un ticket sont un seul geste dans la vraie vie, et les separer en
 * deux actions est ce qui laisse des tickets « A faire » alors qu'ils sont
 * traites.
 *
 * Seuls les changements REELS sont journalises : le serveur compare l'avant et
 * l'apres, demander « passer en cours » sur un ticket deja en cours n'ecrit
 * rien.
 */
export function TicketRedacteur({ ticket }: { ticket: TicketDetail }) {
  const [open, setOpen] = useState(false)
  const [body, setBody] = useState('')
  const [destinataire, setDestinataire] = useState<Destinataire>('client')

  // Les trois menus portent l'etat courant du ticket, pas un « inchangé » : on
  // voit ou en est le ticket sans quitter le redacteur, et l'on change ce qu'on
  // veut. Renvoyer une valeur identique n'ecrit rien — le serveur compare
  // l'avant et l'apres, et ne journalise que les changements reels.
  const [status, setStatus] = useState<string>(ticket.status)
  const [priority, setPriority] = useState<string>(ticket.priority)
  const [assignee, setAssignee] = useState<string>(ticket.assignee?.id ?? PERSONNE)

  // La liste des comptes ne part qu'a l'ouverture du redacteur : la plupart des
  // visites ne servent qu'a relire le ticket.
  const { data: people } = useQuery({ ...peopleQuery, enabled: open })

  const cadre = useRef<HTMLDivElement>(null)
  const reduced = useReducedMotion()

  // Deplier le redacteur en bas d'un fil deja deroule le pousse sous le pli.
  //
  // `autoFocus` sur la zone de texte en amene deja une partie a l'ecran, mais
  // le navigateur ne remonte que le champ focalise : la rangee des trois menus
  // et les boutons restaient dessous. C'est le cadre entier qu'il faut, d'ou ce
  // second defilement qui s'execute apres.
  //
  // `end` et non `start` : le cadre se cale sur le bas de la zone visible, ce
  // qui montre le plus de fil possible au-dessus.
  useEffect(() => {
    if (!open) return

    cadre.current?.scrollIntoView({
      block: 'end',
      behavior: reduced ? 'instant' : 'smooth',
    })
  }, [open, reduced])

  const post = usePostTicketMessage(ticket.id)

  const interne = destinataire === 'interne'
  const empty = body.trim() === ''

  /**
   * Repart de l'etat courant du ticket.
   *
   * Appele a l'ouverture et non seulement a la fermeture : apres une
   * inscription, le ticket a bouge, et rouvrir le redacteur sur les anciennes
   * valeurs proposerait de defaire ce qu'on vient de faire.
   */
  function reset() {
    setBody('')
    setStatus(ticket.status)
    setPriority(ticket.priority)
    setAssignee(ticket.assignee?.id ?? PERSONNE)
  }

  function submit() {
    if (empty) return

    post.mutate(
      {
        body,
        is_internal: interne,
        status: status as TicketStatus,
        priority: priority as TicketPriority,
        change_assignee: true,
        assignee_id: assignee === PERSONNE ? null : assignee,
      },
      {
        onSuccess: () => {
          reset()
          setOpen(false)
          toast.success(interne ? 'Note interne inscrite' : 'Réponse inscrite au registre')
          // Le destinataire n'est pas remis a « client » : on enchaine souvent
          // deux notes internes, et repasser en public sans le vouloir est la
          // faute qu'on ne rattrape pas.
        },
        onError: (error) => {
          toast.error(error instanceof HttpError ? error.message : 'Inscription impossible')
        },
      },
    )
  }

  if (!open) {
    return (
      <div className="flex">
        <Button
          type="button"
          variant="outline"
          size="lg"
          onClick={() => {
            reset()
            setOpen(true)
          }}
        >
          <HugeiconsIcon icon={Message01Icon} size={16} strokeWidth={1.8} />
          Répondre
        </Button>
      </div>
    )
  }

  return (
    <div
      ref={cadre}
      className={cn(
        'overflow-hidden rounded-[12px] border bg-[#f9f9f9] shadow-[0_1px_2px_0_rgb(16_24_40/0.05)]',
        interne ? 'border-[#ffd3b9]' : 'border-[#ebebeb]',
      )}
    >
      {/* Le destinataire se choisit avant d'ecrire, pas apres : c'est ce qui
          change la nature de ce qu'on tape. */}
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[#ebebeb] bg-white px-2 pt-2">
        <SlideTabs
          label="Destinataire de l’entrée"
          value={destinataire}
          onChange={setDestinataire}
          options={[
            { value: 'client', label: 'Réponse au client' },
            { value: 'interne', label: 'Note interne' },
          ]}
        />

        <span className="pr-2 pb-2 text-[12px] text-[#a2a3a7]">
          {interne ? 'Visible par l’agence seule' : 'Visible par le client'}
        </span>
      </div>

      <Textarea
        value={body}
        onChange={(event) => setBody(event.target.value)}
        placeholder={
          interne ? 'Ce que l’équipe doit savoir, et que le client ne verra pas…' : 'Votre réponse…'
        }
        aria-label="Votre entrée"
        autoFocus
        className="min-h-[124px] resize-y rounded-none border-0 bg-transparent px-4 py-3.5 text-[14px] focus-visible:ring-0"
      />

      {/* Une seule rangee : l'etat du ticket a gauche, les deux gestes a
          droite. `flex-wrap` laisse les boutons passer a la ligne sur une
          fenetre etroite plutot que d'ecraser les menus. */}
      <div className="flex flex-wrap items-center gap-2 border-t border-[#ebebeb] bg-white px-3 py-2.5 text-[12.5px] text-[#4f5059]">
        <span className="pr-1">État</span>

        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger
            size="sm"
            className="h-7 gap-1.5 bg-white text-[12.5px]"
            aria-label="Statut"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {TICKET_STATUS_ORDER.map((value) => (
              <SelectItem key={value} value={value}>
                {TICKET_STATUS[value]!.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={priority} onValueChange={setPriority}>
          <SelectTrigger
            size="sm"
            className="h-7 gap-1.5 bg-white text-[12.5px]"
            aria-label="Priorité"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {TICKET_PRIORITY_ORDER.map((value) => (
              <SelectItem key={value} value={value}>
                {TICKET_PRIORITY[value]!.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={assignee} onValueChange={setAssignee}>
          <SelectTrigger
            size="sm"
            className="h-7 gap-1.5 bg-white text-[12.5px]"
            aria-label="Personne assignée"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={PERSONNE}>À prendre</SelectItem>
            {/* L'assigne courant est ajoute d'office : la liste des comptes
                n'arrive qu'a l'ouverture, et sans lui le menu afficherait un
                trou le temps du chargement. */}
            {ticket.assignee !== null &&
              !(people?.items ?? []).some((p: Person) => p.id === ticket.assignee?.id) && (
                <SelectItem value={ticket.assignee.id}>
                  {`${ticket.assignee.firstname} ${ticket.assignee.lastname}`.trim() || 'Sans nom'}
                </SelectItem>
              )}
            {(people?.items ?? []).map((person: Person) => (
              <SelectItem key={person.id} value={person.id}>
                {nameOf(person)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <div className="ml-auto flex gap-2">
          <Button
            type="button"
            variant="ghost"
            size="lg"
            disabled={post.isPending}
            onClick={() => {
              reset()
              setOpen(false)
            }}
          >
            Annuler
          </Button>
          <Button type="button" size="lg" disabled={empty || post.isPending} onClick={submit}>
            {post.isPending ? 'Envoi…' : 'Envoyer'}
          </Button>
        </div>
      </div>
    </div>
  )
}
