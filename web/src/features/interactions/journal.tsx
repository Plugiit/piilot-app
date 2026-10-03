import {
  Calendar03Icon,
  Call02Icon,
  CheckmarkCircle02Icon,
  Delete02Icon,
  Folder01Icon,
  Mail01Icon,
  Note01Icon,
  Ticket02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { useInfiniteQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  interactionFeedQuery,
  useCreateInteraction,
  useDeleteInteraction,
  type InteractionValues,
} from '@/features/interactions/api'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { Interaction, InteractionKind } from '@/types/api'

/** Genres d'interaction : libelle, icone, teinte de la pastille. */
export const INTERACTION_KIND: Record<InteractionKind, { label: string; icon: IconSvgElement; tint: string }> = {
  note: { label: 'Note', icon: Note01Icon, tint: '#f3f4f4' },
  call: { label: 'Appel', icon: Call02Icon, tint: '#eef0fe' },
  meeting: { label: 'Rendez-vous', icon: Calendar03Icon, tint: '#fff1d4' },
  email: { label: 'E-mail', icon: Mail01Icon, tint: '#e8f4fd' },
  project_created: { label: 'Projet créé', icon: Folder01Icon, tint: '#f3f4f4' },
  deliverable_validated: { label: 'Livrable validé', icon: CheckmarkCircle02Icon, tint: '#dcf7ea' },
  ticket_opened: { label: 'Ticket ouvert', icon: Ticket02Icon, tint: '#ffe8ec' },
}

const MANUAL: InteractionValues['kind'][] = ['note', 'call', 'meeting', 'email']

const WHEN = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'short',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})

/** Maintenant, au format d'un champ datetime-local. */
function nowLocal(): string {
  const now = new Date()
  now.setMinutes(now.getMinutes() - now.getTimezoneOffset())

  return now.toISOString().slice(0, 16)
}

/** La phrase d'un evenement automatique ; une saisie se lit telle quelle. */
function eventSentence(item: Interaction): string {
  const title = typeof item.payload.title === 'string' ? item.payload.title : ''
  switch (item.kind) {
    case 'project_created':
      return `Projet « ${title} » créé`
    case 'deliverable_validated':
      return `Livrable « ${title} » validé${typeof item.payload.version === 'number' ? ` en V${item.payload.version}` : ''}`
    case 'ticket_opened':
      return `Ticket ${typeof item.payload.numero === 'number' ? `#${item.payload.numero} ` : ''}« ${title} » ouvert`
    default:
      return item.body
  }
}

/**
 * Saisie d'une interaction : un genre, ce qui s'est dit, et quand.
 *
 * La date vaut maintenant par defaut, mais se corrige : on note souvent un
 * appel apres coup.
 */
export function InteractionForm({
  clientId,
  autoFocus = false,
  onSaved,
}: {
  clientId: string
  autoFocus?: boolean
  /** Appele apres l'enregistrement : la fenetre d'ajout se referme. */
  onSaved?: () => void
}) {
  const [kind, setKind] = useState<InteractionValues['kind']>('note')
  const [body, setBody] = useState('')
  const [when, setWhen] = useState(nowLocal)
  // La date n'est envoyee que si on l'a changee : le champ s'arrete a la
  // minute, et une saisie « maintenant » passerait sinon sous les evenements
  // de la meme minute.
  const [whenEdited, setWhenEdited] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const create = useCreateInteraction()

  function submit() {
    if (body.trim() === '') {
      setError('Écrivez ce qui s’est dit ou passé')
      return
    }
    setError(null)

    create.mutate(
      {
        clientId,
        values: {
          kind,
          body: body.trim(),
          occurred_at: whenEdited && when !== '' ? new Date(when).toISOString() : null,
        },
      },
      {
        onSuccess: () => {
          setBody('')
          setWhen(nowLocal())
          setWhenEdited(false)
          toast.success(`${INTERACTION_KIND[kind].label} ajouté${kind === 'note' ? 'e' : ''} au journal`)
          onSaved?.()
        },
        onError: (err) => toast.error(err instanceof HttpError ? err.message : 'Enregistrement impossible'),
      },
    )
  }

  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        submit()
      }}
      className="flex flex-col gap-2"
    >
      <div className="flex flex-wrap items-center gap-2">
        <div role="radiogroup" aria-label="Genre" className="flex rounded-[10px] bg-[#f3f4f4] p-0.5">
          {MANUAL.map((value) => {
            const { label, icon } = INTERACTION_KIND[value]

            return (
              <button
                key={value}
                type="button"
                role="radio"
                aria-checked={kind === value}
                onClick={() => setKind(value)}
                className={cn(
                  'flex cursor-pointer items-center gap-1 rounded-[8px] px-2.5 py-1 text-[12px] transition-colors',
                  kind === value
                    ? 'bg-white text-[#1b1b1b] shadow-[0_1px_2px_rgb(16_24_40/0.08)]'
                    : 'text-[#73757c] hover:text-[#1b1b1b]',
                )}
              >
                <HugeiconsIcon icon={icon} size={13} strokeWidth={1.8} />
                {label}
              </button>
            )
          })}
        </div>

        <Input
          type="datetime-local"
          value={when}
          max={nowLocal()}
          onChange={(event) => {
            setWhen(event.target.value)
            setWhenEdited(true)
          }}
          aria-label="Date"
          className="h-8 w-[200px] text-[12px]"
        />
      </div>

      <Textarea
        rows={2}
        autoFocus={autoFocus}
        value={body}
        onChange={(event) => setBody(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) submit()
        }}
        placeholder={
          kind === 'call'
            ? 'Ce qui s’est dit au téléphone'
            : kind === 'meeting'
              ? 'Qui était là, ce qui a été décidé'
              : kind === 'email'
                ? 'L’objet et l’essentiel de l’échange'
                : 'Une information à retenir sur ce client'
        }
        aria-invalid={error !== null}
        className="text-[13px]"
      />

      <div className="flex items-center justify-between gap-2">
        <span className={cn('text-[12px]', error !== null ? 'text-[#e5484d]' : 'text-[#a2a3a7]')}>
          {error ?? '⌘ + Entrée pour ajouter'}
        </span>
        <Button type="submit" size="sm" disabled={create.isPending}>
          {create.isPending ? 'Ajout…' : 'Ajouter au journal'}
        </Button>
      </div>
    </form>
  )
}

/**
 * Le fil du journal. Sur la fiche d'un client, `clientId` le restreint ; sur
 * l'ecran du CRM, chaque ligne nomme son client.
 */
export function InteractionList({
  clientId,
  source,
  showClient = false,
}: {
  clientId?: string
  source?: string
  showClient?: boolean
}) {
  const { data, isPending, isError, error, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteQuery(
    interactionFeedQuery(clientId, source),
  )
  const items = data?.pages.flatMap((page) => page.items) ?? []

  if (isError) {
    return (
      <p className="text-[13px] text-[#e5484d]">{error instanceof HttpError ? error.message : 'Chargement impossible'}</p>
    )
  }

  if (isPending) {
    return <div className="h-[120px] animate-pulse rounded-[10px] bg-[#fafafa]" />
  }

  if (items.length === 0) {
    return (
      <p className="py-6 text-center text-[13px] text-[#73757c]">
        Rien au journal pour l’instant. Les projets créés, les livrables validés et les tickets ouverts s’y
        inscrivent d’eux-mêmes.
      </p>
    )
  }

  return (
    <div className="flex flex-col">
      <ol className="flex flex-col">
        {items.map((item) => (
          <InteractionRow key={item.id} item={item} showClient={showClient} />
        ))}
      </ol>
      {hasNextPage && (
        <Button
          variant="ghost"
          size="sm"
          className="mt-2 self-center"
          disabled={isFetchingNextPage}
          onClick={() => void fetchNextPage()}
        >
          {isFetchingNextPage ? 'Chargement…' : 'Afficher plus ancien'}
        </Button>
      )}
    </div>
  )
}

function InteractionRow({ item, showClient }: { item: Interaction; showClient: boolean }) {
  const { label, icon, tint } = INTERACTION_KIND[item.kind]
  const remove = useDeleteInteraction()
  const author = item.author === null ? null : `${item.author.firstname} ${item.author.lastname}`.trim()

  return (
    <li className="group flex gap-3 border-b border-[#f3f4f4] py-2.5 last:border-b-0">
      <span
        aria-hidden
        className="flex size-8 shrink-0 items-center justify-center rounded-full text-[#4b4b4f]"
        style={{ backgroundColor: tint }}
      >
        <HugeiconsIcon icon={icon} size={15} strokeWidth={1.8} />
      </span>

      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <p className="flex flex-wrap items-center gap-x-1.5 text-[12px] text-[#8d8d8d]">
          <span className="font-medium text-[#4b4b4f]">{label}</span>
          {showClient && (
            <>
              <span aria-hidden>·</span>
              <Link to="/crm/clients/$id" params={{ id: item.client.id }} className="hover:underline">
                {item.client.name}
              </Link>
            </>
          )}
          {item.project !== null && (
            <>
              <span aria-hidden>·</span>
              <Link to="/pm/projets/$id" params={{ id: item.project.id }} className="hover:underline">
                {item.project.name}
              </Link>
            </>
          )}
        </p>
        <p className={cn('text-[14px] leading-[1.5] whitespace-pre-line text-[#1b1b1b]', !item.manual && 'text-[#4b4b4f]')}>
          {eventSentence(item)}
        </p>
        <p className="text-[12px] text-[#a2a3a7]">
          {WHEN.format(new Date(item.occurred_at))}
          {author !== null && ` · ${author}`}
        </p>
      </div>

      {item.manual && (
        <button
          type="button"
          aria-label="Effacer cette interaction"
          disabled={remove.isPending}
          onClick={() =>
            remove.mutate(item.id, {
              onError: (err) => toast.error(err instanceof HttpError ? err.message : 'Suppression impossible'),
            })
          }
          className="shrink-0 cursor-pointer self-start text-[#a2a3a7] opacity-0 transition-opacity group-hover:opacity-100 hover:text-[#e5484d] focus-visible:opacity-100"
        >
          <HugeiconsIcon icon={Delete02Icon} size={15} strokeWidth={1.6} />
        </button>
      )}
    </li>
  )
}
