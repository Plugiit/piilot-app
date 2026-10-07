import { CheckmarkCircle02Icon, UserAdd01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useInvite, useResendInvitation, type InviteValues } from '@/features/accounts/api'
import { ROLE, type RoleCode } from '@/features/accounts/format'
import { LinkBox } from '@/features/accounts/link-box'
import { clientListQuery } from '@/features/projects/api'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { SentLink } from '@/types/api'

/**
 * Invitation d'une personne dans Piilot.
 *
 * Deux temps dans la meme fenetre : le formulaire, puis la confirmation avec
 * le lien. Le lien s'affiche meme quand l'e-mail est parti — c'est le recours
 * quand il atterrit dans les indesirables, et le seul moyen de le transmettre
 * quand l'instance n'envoie pas d'e-mails.
 */
export function InviteDialog({
  defaultRole = 'team',
  trigger,
  initial,
  open: controlled,
  onOpenChange,
}: {
  defaultRole?: RoleCode
  /** Bouton d'ouverture ; `null` pour une fenetre pilotee de l'exterieur. */
  trigger?: ReactNode | null
  /** Champs deja connus : depuis la fiche d'un contact, tout est connu. */
  initial?: Partial<InviteValues>
  open?: boolean
  onOpenChange?: (open: boolean) => void
}) {
  const [own, setOwn] = useState(false)
  const open = controlled ?? own
  function setOpen(next: boolean) {
    setOwn(next)
    onOpenChange?.(next)
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {trigger !== null && (
        <DialogTrigger asChild>
          {trigger ?? (
            <Button size="lg" className="gap-1.5">
              <HugeiconsIcon icon={UserAdd01Icon} size={16} strokeWidth={2} />
              Inviter
            </Button>
          )}
        </DialogTrigger>
      )}

      <DialogContent className="sm:max-w-[520px]">
        {/* Monte a l'ouverture : chaque invitation repart d'un formulaire vide. */}
        {open && <InviteFlow defaultRole={defaultRole} initial={initial} onDone={() => setOpen(false)} />}
      </DialogContent>
    </Dialog>
  )
}

interface Sent extends SentLink {
  email: string
}

function InviteFlow({
  defaultRole,
  initial,
  onDone,
}: {
  defaultRole: RoleCode
  initial?: Partial<InviteValues>
  onDone: () => void
}) {
  const [sent, setSent] = useState<Sent | null>(null)
  const [key, setKey] = useState(0)

  if (sent !== null) {
    return (
      <>
        <DialogHeader>
          <div className="mb-1 flex size-10 items-center justify-center rounded-full bg-[#dcf7ea] text-[#0db471]">
            <HugeiconsIcon icon={CheckmarkCircle02Icon} size={22} strokeWidth={1.8} />
          </div>
          <DialogTitle>{sent.emailed ? 'Invitation envoyée' : 'Invitation créée'}</DialogTitle>
          <DialogDescription>
            {sent.emailed ? (
              <>
                Un e-mail vient de partir pour <strong className="text-[#1b1b1b]">{sent.email}</strong>.
                Le lien ci-dessous sert si le message n’arrive pas.
              </>
            ) : (
              <>
                Piilot n’envoie pas d’e-mails sur cette instance : transmettez ce lien à{' '}
                <strong className="text-[#1b1b1b]">{sent.email}</strong> par le moyen de votre choix.
              </>
            )}
          </DialogDescription>
        </DialogHeader>

        <LinkBox link={sent.link} expiresAt={sent.expires_at} />

        <DialogFooter>
          <Button
            type="button"
            variant="ghost"
            size="lg"
            onClick={() => {
              setSent(null)
              setKey((k) => k + 1)
            }}
          >
            Inviter une autre personne
          </Button>
          <Button type="button" size="lg" onClick={onDone}>
            Terminer
          </Button>
        </DialogFooter>
      </>
    )
  }

  return <InviteForm key={key} defaultRole={defaultRole} initial={initial} onSent={setSent} onCancel={onDone} />
}

function InviteForm({
  defaultRole,
  initial,
  onSent,
  onCancel,
}: {
  defaultRole: RoleCode
  initial?: Partial<InviteValues>
  onSent: (sent: Sent) => void
  onCancel: () => void
}) {
  const invite = useInvite()
  const resend = useResendInvitation()

  const [values, setValues] = useState<InviteValues>({
    email: '',
    firstname: '',
    lastname: '',
    role: defaultRole,
    client_id: null,
    ...initial,
  })
  const [errors, setErrors] = useState<Record<string, string>>({})
  // Invitation deja en attente pour l'adresse : on propose de la renvoyer
  // plutot que d'afficher un refus sans issue.
  const [pending, setPending] = useState<string | null>(null)

  const { data: clients } = useQuery({ ...clientListQuery(), enabled: values.role === 'client' })

  function set<K extends keyof InviteValues>(key: K, value: InviteValues[K]) {
    setValues((prev) => ({ ...prev, [key]: value }))
    setErrors((prev) => ({ ...prev, [key]: '' }))
    if (key === 'email') setPending(null)
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault()

    const local: Record<string, string> = {}
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(values.email.trim())) local.email = 'Adresse invalide'
    if (values.role === 'client' && !values.client_id) {
      local.client_id = 'Choisissez le client dont la personne suivra les projets'
    }
    if (Object.keys(local).length > 0) {
      setErrors(local)
      return
    }

    try {
      const result = await invite.mutateAsync({
        ...values,
        email: values.email.trim(),
        client_id: values.role === 'client' ? values.client_id : null,
      })
      onSent({ ...result, email: result.invitation.email })
    } catch (error) {
      if (!(error instanceof HttpError)) {
        toast.error('Invitation impossible')
        return
      }
      if (error.code === 'INVITATION_PENDING' && typeof error.details.invitation_id === 'string') {
        setPending(error.details.invitation_id)
        return
      }
      if (error.code === 'ACCOUNT_EXISTS') {
        setErrors({ email: 'Un compte existe déjà avec cette adresse' })
        return
      }
      if (error.code === 'VALIDATION_FAILED') {
        setErrors(Object.fromEntries(Object.entries(error.details).map(([k, v]) => [k, String(v)])))
        return
      }
      toast.error(error.message)
    }
  }

  async function resendPending() {
    if (pending === null) return
    try {
      const link = await resend.mutateAsync(pending)
      onSent({ ...link, email: values.email.trim() })
    } catch (error) {
      toast.error(error instanceof HttpError ? error.message : 'Renvoi impossible')
    }
  }

  return (
    <form noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>Inviter dans Piilot</DialogTitle>
        <DialogDescription>
          La personne reçoit un lien pour choisir son mot de passe. Son compte n’existe qu’une fois
          l’invitation acceptée.
        </DialogDescription>
      </DialogHeader>

      <label className="flex flex-col gap-1.5">
        <span className="text-[14px] font-medium text-[#1b1b1b]">Adresse e-mail</span>
        <Input
          type="email"
          autoFocus
          autoComplete="off"
          value={values.email}
          onChange={(event) => set('email', event.target.value)}
          placeholder="prenom.nom@exemple.fr"
          aria-invalid={Boolean(errors.email)}
        />
        {errors.email && <span className="text-[13px] text-[#e5484d]">{errors.email}</span>}
      </label>

      {pending !== null && (
        <div className="flex items-center justify-between gap-3 rounded-[10px] bg-[#fff1d4] px-3 py-2.5 text-[13px] text-[#7a5300]">
          <span>Une invitation attend déjà une réponse pour cette adresse.</span>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={resend.isPending}
            onClick={() => void resendPending()}
          >
            {resend.isPending ? 'Envoi…' : 'La renvoyer'}
          </Button>
        </div>
      )}

      <fieldset className="flex flex-col gap-1.5">
        <legend className="mb-1.5 text-[14px] font-medium text-[#1b1b1b]">Rôle</legend>
        <div className="flex flex-col gap-2">
          {(Object.keys(ROLE) as RoleCode[]).map((role) => {
            const chosen = values.role === role
            return (
              <button
                key={role}
                type="button"
                aria-pressed={chosen}
                onClick={() => set('role', role)}
                className={cn(
                  'flex cursor-pointer items-start gap-3 rounded-[10px] border px-3 py-2.5 text-left transition-colors',
                  chosen ? 'border-brand bg-[#fff6f0]' : 'border-[#e8e8e9] hover:bg-[#f8f8f8]',
                )}
              >
                <span
                  aria-hidden
                  className={cn(
                    'mt-1 flex size-4 shrink-0 items-center justify-center rounded-full border',
                    chosen ? 'border-brand' : 'border-[#c4c4c4]',
                  )}
                >
                  {chosen && <span className="bg-brand size-2 rounded-full" />}
                </span>
                <span className="flex flex-col gap-0.5">
                  <span className="text-[14px] font-medium text-[#1b1b1b]">{ROLE[role].label}</span>
                  <span className="text-[13px] text-[#73757c]">{ROLE[role].hint}</span>
                </span>
              </button>
            )
          })}
        </div>
      </fieldset>

      {values.role === 'client' && (
        <label className="flex flex-col gap-1.5">
          <span className="text-[14px] font-medium text-[#1b1b1b]">Client</span>
          <Select
            value={values.client_id ?? undefined}
            onValueChange={(value) => set('client_id', value)}
          >
            <SelectTrigger aria-invalid={Boolean(errors.client_id)} className="w-full">
              <SelectValue placeholder="Choisir le client" />
            </SelectTrigger>
            <SelectContent>
              {(clients?.items ?? []).map((client) => (
                <SelectItem key={client.id} value={client.id}>
                  {client.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {errors.client_id ? (
            <span className="text-[13px] text-[#e5484d]">{errors.client_id}</span>
          ) : (
            <span className="text-[13px] text-[#73757c]">
              La personne ne verra que les projets de ce client.
            </span>
          )}
        </label>
      )}

      <div className="grid grid-cols-2 gap-3">
        <label className="flex flex-col gap-1.5">
          <span className="text-[14px] font-medium text-[#1b1b1b]">
            Prénom <span className="font-normal text-[#a2a3a7]">facultatif</span>
          </span>
          <Input value={values.firstname} onChange={(event) => set('firstname', event.target.value)} />
        </label>
        <label className="flex flex-col gap-1.5">
          <span className="text-[14px] font-medium text-[#1b1b1b]">
            Nom <span className="font-normal text-[#a2a3a7]">facultatif</span>
          </span>
          <Input value={values.lastname} onChange={(event) => set('lastname', event.target.value)} />
        </label>
      </div>
      <p className="-mt-2 text-[12px] text-[#73757c]">
        Pour personnaliser l’e-mail. La personne confirme son nom en activant son compte.
      </p>

      <DialogFooter>
        <Button type="button" variant="ghost" size="lg" onClick={onCancel}>
          Annuler
        </Button>
        <Button type="submit" size="lg" disabled={invite.isPending}>
          {invite.isPending ? 'Envoi…' : 'Envoyer l’invitation'}
        </Button>
      </DialogFooter>
    </form>
  )
}
