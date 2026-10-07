import { Attachment02Icon, Cancel01Icon, MailReceive01Icon, ViewIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'
import { z } from 'zod'

import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  HELD_REASON,
  heldEmailQuery,
  heldEmailsQuery,
  useAttachEmail,
  useDismissEmail,
  useOpenTicketFromEmail,
} from '@/features/inbound/api'
import { projectListQuery } from '@/features/projects/api'
import { HttpError } from '@/lib/api'
import type { InboundItem } from '@/types/api'

/**
 * Les e-mails que Piilot n'a pas su ranger seul.
 *
 * Un expediteur inconnu, un client qui a plusieurs projets en cours, une
 * reponse venue d'une autre adresse, un e-mail transfere par un collegue :
 * chacun attend ici un geste — ouvrir un ticket sur le bon projet, l'ajouter a
 * un ticket existant, ou l'ecarter. Rien n'est rejete en silence.
 */
const searchSchema = z.object({ page: z.number().int().min(1).catch(1) })

export const Route = createFileRoute('/_app/pm/tickets_/a-trier')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ page: search.page }),
  loader: ({ context, deps }) => context.queryClient.query({ ...heldEmailsQuery(deps.page), staleTime: 'static' }),
  component: TriagePage,
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
const PAGE = 25

function TriagePage() {
  const { page } = Route.useSearch()
  const navigate = Route.useNavigate()
  const { data, isError, error } = useQuery(heldEmailsQuery(page))
  const total = data?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / PAGE))

  return (
    <PageFrame title="À trier" trail={[{ label: 'Tickets', to: '/pm/tickets', search: { page: 1 } }]}>
      <div className="mx-auto flex w-full max-w-[880px] flex-col gap-3 p-4">
        <p className="text-[14px] text-[#73757c]">
          Les e-mails reçus que Piilot n’a pas su ranger seul. Ouvrez un ticket sur le bon projet, ajoutez-les à un ticket existant, ou écartez-les.
        </p>

        {isError ? (
          <p className="rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
            {error instanceof HttpError ? error.message : 'Chargement impossible'}
          </p>
        ) : data !== undefined && data.items.length === 0 ? (
          <div className="flex flex-col items-center gap-2 rounded-[14px] border border-dashed border-[#e3e4e6] px-4 py-12 text-center">
            <HugeiconsIcon icon={MailReceive01Icon} size={28} strokeWidth={1.4} className="text-[#a2a3a7]" />
            <p className="text-[15px] text-[#1b1b1b]">Rien à trier.</p>
            <p className="text-[13px] text-[#73757c]">Les e-mails reconnus deviennent des tickets, ou rejoignent le leur, sans passer par ici.</p>
          </div>
        ) : (
          (data?.items ?? []).map((item) => <HeldEmail key={item.id} item={item} />)
        )}

        {pages > 1 && (
          <div className="flex items-center justify-between pt-2">
            <p className="text-[12px] text-[#777]">
              {total} e-mail{total > 1 ? 's' : ''} · page {page} sur {pages}
            </p>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => void navigate({ search: { page: page - 1 } })}>
                Précédent
              </Button>
              <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => void navigate({ search: { page: page + 1 } })}>
                Suivant
              </Button>
            </div>
          </div>
        )}
      </div>
    </PageFrame>
  )
}

const AUTRE = '__autres__'

function HeldEmail({ item }: { item: InboundItem }) {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [project, setProject] = useState<string>(item.projects.length === 1 ? item.projects[0]!.id : '')
  const [numero, setNumero] = useState('')
  const openTicket = useOpenTicketFromEmail()
  const attach = useAttachEmail()
  const dismiss = useDismissEmail()
  const pending = openTicket.isPending || attach.isPending || dismiss.isPending

  // Les autres projets ne se chargent qu'a la demande : la plupart des e-mails
  // vont sur un projet de leur client.
  const [others, setOthers] = useState(item.client === null)
  const { data: allProjects } = useQuery({
    ...projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }),
    enabled: others,
  })
  const own = new Set(item.projects.map((p) => p.id))

  const report = (error: unknown) => toast.error(error instanceof HttpError ? error.message : 'Action impossible')

  return (
    <article className="flex flex-col gap-3 rounded-[14px] border border-[#e8e8e9] bg-white p-4">
      <header className="flex flex-wrap items-start justify-between gap-2">
        <div className="flex min-w-0 flex-col gap-0.5">
          <span className="truncate text-[15px] font-medium text-[#1b1b1b]">{item.subject || '(sans objet)'}</span>
          <span className="truncate text-[13px] text-[#73757c]">
            {item.from_name !== '' ? `${item.from_name} · ` : ''}
            {item.from_address}
            {item.client !== null && ` · ${item.client.name}`}
          </span>
        </div>
        <div className="flex shrink-0 flex-col items-end gap-1">
          <span className="rounded-full bg-[#fff2ea] px-2 py-0.5 text-[12px] text-[#b84a0c]">{HELD_REASON[item.reason] ?? item.reason}</span>
          <span className="text-[12px] text-[#a2a3a7] tabular-nums">{WHEN.format(new Date(item.received_at))}</span>
        </div>
      </header>

      {open ? (
        <FullEmail id={item.id} />
      ) : (
        item.excerpt !== '' && <p className="line-clamp-3 text-[14px] leading-[1.6] whitespace-pre-line text-[#4b4b4f]">{item.excerpt}</p>
      )}

      <div className="flex flex-wrap items-center gap-3 text-[12.5px] text-[#73757c]">
        {item.attachments > 0 && (
          <span className="flex items-center gap-1">
            <HugeiconsIcon icon={Attachment02Icon} size={14} strokeWidth={1.8} />
            {item.attachments} pièce{item.attachments > 1 ? 's' : ''} jointe{item.attachments > 1 ? 's' : ''}
          </span>
        )}
        <button type="button" className="flex cursor-pointer items-center gap-1 hover:text-[#1b1b1b]" onClick={() => setOpen((v) => !v)}>
          <HugeiconsIcon icon={ViewIcon} size={14} strokeWidth={1.8} />
          {open ? 'Réduire' : 'Lire l’e-mail en entier'}
        </button>
      </div>

      <div className="flex flex-col gap-2 border-t border-[#f3f4f4] pt-3 lg:flex-row lg:items-center">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <Select
            value={project}
            onValueChange={(value) => {
              if (value === AUTRE) {
                setOthers(true)
                return
              }
              setProject(value)
            }}
          >
            <SelectTrigger className="h-9 min-w-0 flex-1 text-[13px]" aria-label="Projet du ticket">
              <SelectValue placeholder="Choisir le projet" />
            </SelectTrigger>
            <SelectContent>
              {item.projects.length > 0 && (
                <SelectGroup>
                  <SelectLabel>{item.client?.name ?? 'Projets du client'}</SelectLabel>
                  {item.projects.map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              )}
              {others ? (
                <SelectGroup>
                  <SelectLabel>Tous les projets</SelectLabel>
                  {(allProjects?.items ?? [])
                    .filter((p) => !own.has(p.id))
                    .map((p) => (
                      <SelectItem key={p.id} value={p.id}>
                        {p.name}
                      </SelectItem>
                    ))}
                </SelectGroup>
              ) : (
                <SelectItem value={AUTRE}>
                  <span className="text-[#73757c]">Un autre projet…</span>
                </SelectItem>
              )}
            </SelectContent>
          </Select>
          <Button
            size="sm"
            className="h-9"
            disabled={pending || project === ''}
            onClick={() =>
              openTicket.mutate(
                { id: item.id, projectId: project },
                {
                  onSuccess: ({ ticket_id }) => {
                    toast.success('Ticket ouvert, accusé de réception envoyé', {
                      action: { label: 'Ouvrir', onClick: () => void navigate({ to: '/pm/tickets/$id', params: { id: ticket_id } }) },
                    })
                  },
                  onError: report,
                },
              )
            }
          >
            Créer le ticket
          </Button>
        </div>

        <span className="hidden text-[12px] text-[#a2a3a7] lg:block">ou</span>

        <form
          className="flex items-center gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            const n = Number(numero.replace(/\D/g, ''))
            if (!n) return
            attach.mutate(
              { id: item.id, numero: n },
              {
                onSuccess: ({ ticket_id }) =>
                  toast.success(`Ajouté au ticket #${n}`, {
                    action: { label: 'Ouvrir', onClick: () => void navigate({ to: '/pm/tickets/$id', params: { id: ticket_id } }) },
                  }),
                onError: report,
              },
            )
          }}
        >
          <Input
            value={numero}
            onChange={(e) => setNumero(e.target.value)}
            placeholder="#47"
            inputMode="numeric"
            aria-label="Numéro du ticket"
            className="h-9 w-[84px] text-[13px]"
          />
          <Button type="submit" variant="outline" size="sm" className="h-9" disabled={pending || numero.replace(/\D/g, '') === ''}>
            Ajouter au ticket
          </Button>
        </form>

        <Button
          variant="ghost"
          size="sm"
          className="h-9 gap-1 text-[#73757c]"
          disabled={pending}
          onClick={() => dismiss.mutate(item.id, { onSuccess: () => toast.success('E-mail écarté'), onError: report })}
        >
          <HugeiconsIcon icon={Cancel01Icon} size={14} strokeWidth={1.8} />
          Écarter
        </Button>
      </div>
    </article>
  )
}

/** Le texte complet, citations comprises, et les pieces jointes. */
function FullEmail({ id }: { id: string }) {
  const { data, isPending } = useQuery(heldEmailQuery(id))
  if (isPending || data === undefined) return <div className="h-[80px] animate-pulse rounded-[10px] bg-[#fafafa]" />

  return (
    <div className="flex flex-col gap-2">
      {data.to.length > 0 && <p className="text-[12px] text-[#8d8d8d]">À : {data.to.join(', ')}</p>}
      <pre className="max-h-[420px] overflow-auto rounded-[10px] bg-[#fafafa] p-3 font-sans text-[13.5px] leading-[1.6] whitespace-pre-wrap text-[#1b1b1b]">
        {data.body || '(e-mail sans texte)'}
      </pre>
      {data.files.length > 0 && (
        <p className="text-[12.5px] text-[#73757c]">
          Pièces jointes : {data.files.map((f) => f.filename).join(', ')} — elles rejoindront le ticket.
        </p>
      )}
      {data.ticket_id !== null && (
        <Link to="/pm/tickets/$id" params={{ id: data.ticket_id }} className="text-[12.5px] text-[#1b1b1b] underline">
          Ticket concerné
        </Link>
      )}
    </div>
  )
}
