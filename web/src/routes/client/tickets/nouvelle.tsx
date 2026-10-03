import { ArrowLeft01Icon, Attachment02Icon, Cancel01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useRef, useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { portalProjectsQuery, useCreateTicket, type NewTicketValues } from '@/features/portal/api'
import { fileSize, PORTAL_PRIORITY, PORTAL_TRACKER } from '@/features/portal/format'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'

/**
 * Nouvelle demande.
 *
 * Une page et non un dialogue : sur un telephone, un dialogue qui porte une
 * description et des pieces jointes ne laisse plus voir ce qu'on ecrit. Le
 * projet est choisi d'office quand le client n'en a qu'un.
 */
export const Route = createFileRoute('/client/tickets/nouvelle')({
  loader: ({ context }) => context.queryClient.ensureQueryData(portalProjectsQuery),
  component: NewTicketPage,
})

const MAX_FILES = 10

function NewTicketPage() {
  const navigate = useNavigate()
  const { data } = useQuery(portalProjectsQuery)
  const projects = (data?.items ?? []).filter((project) => project.status !== 'livre').concat(
    (data?.items ?? []).filter((project) => project.status === 'livre'),
  )

  const [values, setValues] = useState<NewTicketValues>({
    project_id: projects.length === 1 ? projects[0]!.id : '',
    tracker: 'anomalie',
    priority: 'normal',
    subject: '',
    description: '',
  })
  const [files, setFiles] = useState<File[]>([])
  const [errors, setErrors] = useState<Record<string, string>>({})
  const input = useRef<HTMLInputElement>(null)
  const create = useCreateTicket()

  function patch(next: Partial<NewTicketValues>) {
    setValues((prev) => ({ ...prev, ...next }))
    setErrors((prev) => {
      const out = { ...prev }
      for (const key of Object.keys(next)) delete out[key]
      return out
    })
  }

  function submit() {
    const local: Record<string, string> = {}
    if (values.project_id === '') local.project_id = 'Choisissez le projet concerné'
    if (values.subject.trim() === '') local.subject = 'Donnez un titre à votre demande'
    if (values.description.trim() === '') local.description = 'Décrivez votre demande'
    setErrors(local)
    if (Object.keys(local).length > 0) return

    create.mutate(
      { values: { ...values, subject: values.subject.trim(), description: values.description.trim() }, files },
      {
        onSuccess: ({ ticket, failed }) => {
          toast.success(`Demande #${ticket.numero} envoyée. L’agence est prévenue.`)
          if (failed.length > 0) {
            toast.error(`Pièce jointe non envoyée : ${failed.join(', ')}. Ajoutez-la depuis la demande.`)
          }
          void navigate({ to: '/client/tickets/$id', params: { id: ticket.id } })
        },
        onError: (err) => {
          if (err instanceof HttpError && err.code === 'VALIDATION_FAILED') {
            setErrors(Object.fromEntries(Object.entries(err.details).map(([k, v]) => [k, String(v)])))
            return
          }
          toast.error(err instanceof HttpError ? err.message : 'Envoi impossible')
        },
      },
    )
  }

  return (
    <div className="mx-auto flex w-full max-w-[640px] flex-col gap-5">
      <Link to="/client/tickets" className="flex items-center gap-1 self-start text-[14px] text-[#73757c] hover:text-[#1b1b1b]">
        <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
        Support
      </Link>

      <h1 className="font-heading text-[26px] leading-tight font-medium text-[#1b1b1b]">Nouvelle demande</h1>

      <form
        noValidate
        onSubmit={(event) => {
          event.preventDefault()
          submit()
        }}
        className="flex flex-col gap-5 rounded-[16px] border border-[#e8e8e9] bg-white p-4 sm:p-5"
      >
        <Field label="Projet concerné" error={errors.project_id}>
          <Select value={values.project_id} onValueChange={(project_id) => patch({ project_id })}>
            <SelectTrigger className="h-11 w-full text-[15px]" aria-invalid={errors.project_id !== undefined}>
              <SelectValue placeholder="Choisir un projet" />
            </SelectTrigger>
            <SelectContent>
              {projects.map((project) => (
                <SelectItem key={project.id} value={project.id}>
                  {project.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>

        <Field label="De quoi s’agit-il ?">
          <div role="radiogroup" className="grid grid-cols-1 gap-2 sm:grid-cols-3">
            {(Object.keys(PORTAL_TRACKER) as NewTicketValues['tracker'][]).map((tracker) => (
              <Choice
                key={tracker}
                selected={values.tracker === tracker}
                onSelect={() => patch({ tracker })}
                label={PORTAL_TRACKER[tracker].label}
                hint={PORTAL_TRACKER[tracker].hint}
              />
            ))}
          </div>
        </Field>

        <Field label="Titre" error={errors.subject}>
          <Input
            value={values.subject}
            onChange={(event) => patch({ subject: event.target.value })}
            placeholder="Le formulaire de contact ne s’envoie pas"
            className="h-11 text-[15px]"
            aria-invalid={errors.subject !== undefined}
          />
        </Field>

        <Field label="Description" error={errors.description}>
          <Textarea
            rows={6}
            value={values.description}
            onChange={(event) => patch({ description: event.target.value })}
            placeholder={
              values.tracker === 'anomalie'
                ? 'Sur quelle page, ce que vous avez fait, ce qui s’est passé, et ce que vous attendiez.'
                : values.tracker === 'evolution'
                  ? 'Ce que vous aimeriez, et pourquoi.'
                  : 'Votre question, avec le contexte utile.'
            }
            className="text-[15px]"
            aria-invalid={errors.description !== undefined}
          />
        </Field>

        <Field label="Priorité" error={errors.priority}>
          <div role="radiogroup" className="grid grid-cols-3 gap-2">
            {(Object.keys(PORTAL_PRIORITY) as NewTicketValues['priority'][]).map((priority) => (
              <Choice
                key={priority}
                selected={values.priority === priority}
                onSelect={() => patch({ priority })}
                label={PORTAL_PRIORITY[priority].label}
                hint={PORTAL_PRIORITY[priority].hint}
              />
            ))}
          </div>
        </Field>

        <Field label="Pièces jointes" hint="Captures d’écran, documents : dix au plus.">
          <input
            ref={input}
            type="file"
            multiple
            hidden
            onChange={(event) => {
              const picked = Array.from(event.target.files ?? [])
              setFiles((prev) => [...prev, ...picked].slice(0, MAX_FILES))
              event.target.value = ''
            }}
          />
          {files.length > 0 && (
            <ul className="flex flex-col gap-1">
              {files.map((file, index) => (
                <li key={`${file.name}-${index}`} className="flex items-center gap-2 text-[14px] text-[#1b1b1b]">
                  <HugeiconsIcon icon={Attachment02Icon} size={14} strokeWidth={1.6} className="text-[#8d8d8d]" />
                  <span className="min-w-0 flex-1 truncate">{file.name}</span>
                  <span className="text-[12px] text-[#8d8d8d]">{fileSize(file.size)}</span>
                  <button
                    type="button"
                    aria-label={`Retirer ${file.name}`}
                    onClick={() => setFiles((prev) => prev.filter((_, i) => i !== index))}
                    className="cursor-pointer text-[#a2a3a7] hover:text-[#1b1b1b]"
                  >
                    <HugeiconsIcon icon={Cancel01Icon} size={14} strokeWidth={1.8} />
                  </button>
                </li>
              ))}
            </ul>
          )}
          {files.length < MAX_FILES && (
            <Button type="button" variant="outline" className="gap-1.5 self-start" onClick={() => input.current?.click()}>
              <HugeiconsIcon icon={Attachment02Icon} size={16} strokeWidth={1.6} />
              Ajouter un fichier
            </Button>
          )}
        </Field>

        <Button type="submit" size="lg" className="h-12 text-[15px]" disabled={create.isPending}>
          {create.isPending ? 'Envoi…' : 'Envoyer la demande'}
        </Button>
      </form>
    </div>
  )
}

function Field({ label, hint, error, children }: { label: string; hint?: string; error?: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-[14px] font-medium text-[#1b1b1b]">{label}</span>
      {hint !== undefined && <span className="-mt-1 text-[12px] text-[#8d8d8d]">{hint}</span>}
      {children}
      {error !== undefined && <span className="text-[13px] text-[#e5484d]">{error}</span>}
    </div>
  )
}

function Choice({
  selected,
  onSelect,
  label,
  hint,
}: {
  selected: boolean
  onSelect: () => void
  label: string
  hint: string
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={onSelect}
      className={cn(
        'flex cursor-pointer flex-col items-start gap-0.5 rounded-[12px] border px-3 py-2.5 text-left transition-colors',
        selected ? 'border-brand bg-[#fff6f0]' : 'border-[#e8e8e9] hover:border-[#d0d1d3]',
      )}
    >
      <span className="text-[14px] font-medium text-[#1b1b1b]">{label}</span>
      <span className="text-[12px] leading-snug text-[#8d8d8d]">{hint}</span>
    </button>
  )
}
