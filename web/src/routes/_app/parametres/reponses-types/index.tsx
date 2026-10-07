import { Delete02Icon, PencilEdit02Icon, PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { PageFrame } from '@/components/layout/page-frame'
import { CHAMP, Card, Field } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { replyTemplatesQuery, useDeleteReplyTemplate, useSaveReplyTemplate } from '@/features/tickets/replies'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import type { ReplyTemplate } from '@/types/api'

/**
 * Les reponses types des tickets : les demandes qui reviennent — un mot de
 * passe oublie, un acces FTP, une mise a jour planifiee — ont leur texte tout
 * pret, a inserer depuis le redacteur d'un ticket et a ajuster avant l'envoi.
 */
export const Route = createFileRoute('/_app/parametres/reponses-types/')({
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'tickets.write')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  component: ReplyTemplatesPage,
})

function ReplyTemplatesPage() {
  const { data, isPending } = useQuery(replyTemplatesQuery)
  const [editing, setEditing] = useState<ReplyTemplate | 'new' | null>(null)

  return (
    <PageFrame title="Réponses types">
      <div className="flex flex-col gap-3 p-4">
        <Card
          title="Réponses types"
          description="Insérées depuis « Réponses types » au-dessus de la réponse à un ticket, puis relues avant l’envoi. {prenom}, {numero}, {sujet} et {projet} se remplissent tout seuls."
        >
          {isPending ? (
            <div className="h-[80px] animate-pulse rounded-[10px] bg-[#fafafa]" />
          ) : (
            <ul className="flex flex-col">
              {(data?.items ?? []).map((t) =>
                editing !== 'new' && editing?.id === t.id ? (
                  <li key={t.id} className="border-b border-[#f3f4f4] py-2 last:border-b-0">
                    <Editor template={t} onDone={() => setEditing(null)} />
                  </li>
                ) : (
                  <Row key={t.id} template={t} onEdit={() => setEditing(t)} />
                ),
              )}
              {(data?.items.length ?? 0) === 0 && editing !== 'new' && (
                <li className="px-1 py-4 text-[14px] text-[#73757c]">Aucune réponse type pour l’instant.</li>
              )}
            </ul>
          )}
          {editing === 'new' ? (
            <Editor onDone={() => setEditing(null)} />
          ) : (
            <div className="flex">
              <Button type="button" variant="outline" className="gap-1.5" onClick={() => setEditing('new')}>
                <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
                Nouvelle réponse type
              </Button>
            </div>
          )}
        </Card>
      </div>
    </PageFrame>
  )
}

function Row({ template, onEdit }: { template: ReplyTemplate; onEdit: () => void }) {
  const remove = useDeleteReplyTemplate()
  const [confirm, setConfirm] = useState(false)

  return (
    <li className="flex items-start gap-3 border-b border-[#f3f4f4] px-1 py-3 last:border-b-0">
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-[15px] text-[#1b1b1b]">{template.title}</span>
        <span className="line-clamp-2 text-[13px] whitespace-pre-line text-[#73757c]">{template.body}</span>
      </div>
      {confirm ? (
        <span className="flex items-center gap-1">
          <Button
            size="sm"
            variant="destructive"
            disabled={remove.isPending}
            onClick={() => remove.mutate(template.id, { onSuccess: () => toast.success('Réponse type retirée') })}
          >
            Retirer
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setConfirm(false)}>
            Annuler
          </Button>
        </span>
      ) : (
        <span className="flex items-center gap-1">
          <Button size="icon-sm" variant="ghost" aria-label="Modifier" onClick={onEdit}>
            <HugeiconsIcon icon={PencilEdit02Icon} size={16} strokeWidth={1.8} />
          </Button>
          <Button size="icon-sm" variant="ghost" aria-label="Retirer" onClick={() => setConfirm(true)}>
            <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.8} />
          </Button>
        </span>
      )}
    </li>
  )
}

function Editor({ template, onDone }: { template?: ReplyTemplate; onDone: () => void }) {
  const save = useSaveReplyTemplate()
  const [title, setTitle] = useState(template?.title ?? '')
  const [body, setBody] = useState(template?.body ?? 'Bonjour {prenom},\n\n')
  const [errors, setErrors] = useState<Record<string, string>>({})

  return (
    <form
      noValidate
      className="flex flex-col gap-3 rounded-[10px] bg-[#fafafa] p-3"
      onSubmit={(event) => {
        event.preventDefault()
        save.mutate(
          { id: template?.id, title, body },
          {
            onSuccess: () => {
              toast.success(template === undefined ? 'Réponse type ajoutée' : 'Réponse type modifiée')
              onDone()
            },
            onError: (error) => {
              if (error instanceof HttpError && error.code === 'VALIDATION_FAILED') {
                setErrors(Object.fromEntries(Object.entries(error.details).map(([k, v]) => [k, String(v)])))
                return
              }
              toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible')
            },
          },
        )
      }}
    >
      <Field label="Titre" error={errors.title}>
        <Input className={CHAMP} autoFocus value={title} placeholder="Accès FTP" onChange={(e) => setTitle(e.target.value)} />
      </Field>
      <Field label="Texte" error={errors.body}>
        <Textarea className={`${CHAMP} min-h-[140px]`} value={body} onChange={(e) => setBody(e.target.value)} />
      </Field>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="ghost" onClick={onDone}>
          Annuler
        </Button>
        <Button type="submit" disabled={save.isPending}>
          {save.isPending ? 'Enregistrement…' : 'Enregistrer'}
        </Button>
      </div>
    </form>
  )
}
