import { ArrowLeft01Icon, Attachment02Icon, Download04Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useRef, useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { portalFileUrl, portalTicketQuery, useAttachToTicket, useReplyTicket } from '@/features/portal/api'
import { fileSize, PORTAL_TRACKER, portalTicketStatus } from '@/features/portal/format'
import { StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { PortalTicketDetail, PortalTicketEntry } from '@/types/api'

/**
 * Une demande et sa conversation avec l'agence.
 *
 * Un fil de messages, comme une messagerie : les reponses de l'agence a
 * gauche, celles du client a droite, et les changements de statut en simple
 * mention au milieu. Les notes internes de l'equipe n'arrivent jamais ici —
 * l'API ne les sert pas.
 */
export const Route = createFileRoute('/client/tickets/$id')({
  loader: ({ context, params }) => context.queryClient.ensureQueryData(portalTicketQuery(params.id)),
  component: ClientTicketPage,
  errorComponent: () => (
    <div className="flex flex-col items-start gap-3">
      <BackLink />
      <p className="rounded-[14px] border border-[#e8e8e9] bg-white p-6 text-[15px] text-[#4b4b4f]">
        Cette demande est introuvable, ou elle ne fait pas partie de votre espace.
      </p>
    </div>
  ),
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

function BackLink() {
  return (
    <Link to="/client/tickets" className="flex items-center gap-1 self-start text-[14px] text-[#73757c] hover:text-[#1b1b1b]">
      <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
      Support
    </Link>
  )
}

function ClientTicketPage() {
  const { id } = Route.useParams()
  const { data: ticket } = useQuery(portalTicketQuery(id))

  if (ticket === undefined) return null

  const status = portalTicketStatus(ticket.status)

  return (
    <div className="mx-auto flex w-full max-w-[720px] flex-col gap-5">
      <BackLink />

      <header className="flex flex-col gap-2">
        <h1 className="font-heading flex flex-wrap items-baseline gap-x-2 text-[24px] leading-tight font-medium text-[#1b1b1b]">
          <span className="text-[#a2a3a7] tabular-nums">#{ticket.numero}</span>
          {ticket.subject}
        </h1>
        <div className="flex flex-wrap items-center gap-2 text-[13px] text-[#8d8d8d]">
          <StatusPill label={status.label} color={status.color} pill={status.pill} />
          <span>
            {PORTAL_TRACKER[ticket.tracker].label} · {ticket.project.name}
          </span>
        </div>
      </header>

      <ol className="flex flex-col gap-3">
        <li>
          <Bubble
            mine
            author={ticket.reporter === '' ? 'Vous' : ticket.reporter}
            at={ticket.created_at}
            body={ticket.description}
          />
        </li>
        {ticket.entries.map((entry, index) => (
          <li key={`${entry.kind}-${index}`}>
            <EntryLine entry={entry} />
          </li>
        ))}
      </ol>

      <ReplyBox ticket={ticket} />

      <Files ticket={ticket} />
    </div>
  )
}

function EntryLine({ entry }: { entry: PortalTicketEntry }) {
  if (entry.kind === 'status') {
    const status = portalTicketStatus(entry.status as PortalTicketDetail['status'])

    return (
      <p className="mx-auto w-fit max-w-full rounded-full bg-[#ebebeb] px-3 py-1 text-center text-[12px] text-[#73757c]">
        Statut : <span className="font-medium text-[#1b1b1b]">{status.label}</span> · {WHEN.format(new Date(entry.at))}
      </p>
    )
  }

  return (
    <Bubble
      mine={!entry.from_agency}
      author={entry.from_agency ? `${entry.author || 'L’agence'} · agence` : entry.author || 'Vous'}
      at={entry.at}
      body={entry.body}
    />
  )
}

function Bubble({ mine, author, at, body }: { mine: boolean; author: string; at: string; body: string }) {
  return (
    <div className={cn('flex flex-col gap-1', mine ? 'items-end' : 'items-start')}>
      <span className="px-1 text-[12px] text-[#8d8d8d]">
        {author} · {WHEN.format(new Date(at))}
      </span>
      <p
        className={cn(
          'max-w-[92%] rounded-[16px] px-4 py-3 text-[15px] leading-[1.55] whitespace-pre-line sm:max-w-[80%]',
          mine ? 'rounded-br-[6px] bg-[#1b1b1b] text-white' : 'rounded-bl-[6px] border border-[#e8e8e9] bg-white text-[#1b1b1b]',
        )}
      >
        {body}
      </p>
    </div>
  )
}

function Files({ ticket }: { ticket: PortalTicketDetail }) {
  const input = useRef<HTMLInputElement>(null)
  const attach = useAttachToTicket(ticket.id)

  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-[15px] font-medium text-[#1b1b1b]">Pièces jointes</h2>
      {ticket.files.length > 0 && (
        <ul className="flex flex-col overflow-hidden rounded-[14px] border border-[#e8e8e9] bg-white">
          {ticket.files.map((file) => (
            <li key={file.id} className="border-b border-[#f3f4f4] last:border-b-0">
              <a href={portalFileUrl(file.id)} className="flex items-center gap-3 px-4 py-2.5 hover:bg-[#fafafa]">
                <HugeiconsIcon icon={Attachment02Icon} size={16} strokeWidth={1.6} className="text-[#8d8d8d]" />
                <span className="min-w-0 flex-1 truncate text-[14px] text-[#1b1b1b]">{file.filename}</span>
                <span className="text-[12px] text-[#8d8d8d]">{fileSize(file.size_bytes)}</span>
                <HugeiconsIcon icon={Download04Icon} size={16} strokeWidth={1.6} className="text-[#73757c]" />
              </a>
            </li>
          ))}
        </ul>
      )}
      <input
        ref={input}
        type="file"
        hidden
        onChange={(event) => {
          const file = event.target.files?.[0]
          event.target.value = ''
          if (file === undefined) return
          attach.mutate(file, {
            onSuccess: () => toast.success('Fichier ajouté'),
            onError: (err) =>
              toast.error(
                err instanceof HttpError ? String(err.details.file ?? err.message) : 'Envoi impossible',
              ),
          })
        }}
      />
      {ticket.files.length < 10 && (
        <Button
          type="button"
          variant="outline"
          className="gap-1.5 self-start"
          disabled={attach.isPending}
          onClick={() => input.current?.click()}
        >
          <HugeiconsIcon icon={Attachment02Icon} size={16} strokeWidth={1.6} />
          {attach.isPending ? 'Envoi…' : 'Ajouter un fichier'}
        </Button>
      )}
    </section>
  )
}

function ReplyBox({ ticket }: { ticket: PortalTicketDetail }) {
  const [body, setBody] = useState('')
  const reply = useReplyTicket(ticket.id)

  function send() {
    if (body.trim() === '') return
    reply.mutate(body.trim(), {
      onSuccess: () => {
        setBody('')
        toast.success('Message envoyé à l’agence')
      },
      onError: (err) => toast.error(err instanceof HttpError ? err.message : 'Envoi impossible'),
    })
  }

  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        send()
      }}
      className="flex flex-col gap-2 rounded-[16px] border border-[#e8e8e9] bg-white p-3"
    >
      {!ticket.open && (
        <p className="text-[13px] text-[#8d8d8d]">
          Cette demande est {ticket.status === 'done' ? 'résolue' : 'fermée'}. Vous pouvez encore écrire : l’agence sera
          prévenue.
        </p>
      )}
      <Textarea
        rows={3}
        value={body}
        onChange={(event) => setBody(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) send()
        }}
        placeholder="Votre message à l’agence…"
        aria-label="Votre message"
        className="text-[15px]"
      />
      <div className="flex justify-end">
        <Button type="submit" size="lg" disabled={reply.isPending || body.trim() === ''}>
          {reply.isPending ? 'Envoi…' : 'Envoyer'}
        </Button>
      </div>
    </form>
  )
}
