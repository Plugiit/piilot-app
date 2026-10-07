import { MailSend01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { invitationListQuery, useResendInvitation, useRevokeInvitation } from '@/features/accounts/api'
import { displayName, roleOf } from '@/features/accounts/format'
import { LinkBox } from '@/features/accounts/link-box'
import { AccountsToolbar, matchesSearch, ToolbarSearch } from '@/features/accounts/toolbar'
import { HttpError } from '@/lib/api'
import { sessionQuery } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { Invitation, SentLink } from '@/types/api'

export const Route = createFileRoute('/_app/parametres/comptes/invitations')({
  component: InvitationsPage,
})

const DATE = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long' })

/**
 * Invitations qui attendent une reponse.
 *
 * Une invitation expiree reste listee : c'est ici qu'on la renvoie, et elle
 * disparaitrait sinon sans que personne ne sache qu'elle n'a pas abouti.
 * Une invitation acceptee, elle, quitte la liste : la personne est devenue un
 * compte, dans l'onglet voisin.
 */
function InvitationsPage() {
  const { data, isPending } = useQuery(invitationListQuery)
  const { data: session } = useQuery(sessionQuery)
  const canManage = session?.permissions.includes('users.write') === true

  const resend = useResendInvitation()
  const revoke = useRevokeInvitation()
  const [link, setLink] = useState<(SentLink & { email: string }) | null>(null)
  const [cancelling, setCancelling] = useState<Invitation | null>(null)

  const items = data?.items ?? []

  // Recherche locale : la liste est bornee et deja la, filtrer ici evite un
  // aller-retour pour quelques lignes.
  const [query, setQuery] = useState('')
  const shown = items.filter((invitation) =>
    matchesSearch(
      query,
      displayName(invitation),
      invitation.email,
      invitation.client?.name,
      roleOf(invitation.role).label,
      invitation.invited_by,
    ),
  )

  function doResend(invitation: Invitation) {
    resend.mutate(invitation.id, {
      onSuccess: (sent) => setLink({ ...sent, email: invitation.email }),
      onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Renvoi impossible'),
    })
  }

  return (
    <div className="flex flex-1 flex-col gap-3 p-4">
      <AccountsToolbar>
        <ToolbarSearch
          value={query}
          onChange={setQuery}
          placeholder="Rechercher une invitation"
          label="Rechercher une invitation"
        />
      </AccountsToolbar>

      {isPending && <div className="h-[120px] animate-pulse rounded-[12px] bg-[#fafafa]" />}

      {!isPending && items.length > 0 && shown.length === 0 && (
        <p className="rounded-[12px] border border-dashed border-[#d0d1d3] px-4 py-10 text-center text-[14px] text-[#73757c]">
          Aucune invitation ne correspond à « {query} ».
        </p>
      )}

      {!isPending && items.length === 0 && (
        <div className="flex flex-col items-center gap-2 rounded-[12px] border border-dashed border-[#d0d1d3] px-4 py-12 text-center">
          <HugeiconsIcon icon={MailSend01Icon} size={28} strokeWidth={1.4} className="text-[#a2a3a7]" />
          <p className="text-[14px] font-medium text-[#1b1b1b]">Aucune invitation en attente</p>
          <p className="max-w-[360px] text-[13px] text-[#73757c]">
            Les invitations envoyées apparaissent ici jusqu’à ce que la personne active son compte.
          </p>
        </div>
      )}

      {shown.length > 0 && (
        <ul className="overflow-hidden rounded-[12px] border border-[#e8e8e9]">
          {shown.map((invitation) => {
            const expired = invitation.status === 'expired'
            const role = roleOf(invitation.role)
            return (
              <li
                key={invitation.id}
                className="flex flex-wrap items-center gap-3 border-t border-[#e8e8e9] bg-white px-3 py-3 first:border-t-0"
              >
                <div className="flex min-w-[220px] flex-1 flex-col">
                  <span className="truncate text-[14px] font-medium text-[#1b1b1b]">
                    {displayName(invitation)}
                  </span>
                  <span className="truncate text-[13px] text-[#73757c]">
                    {invitation.firstname || invitation.lastname ? invitation.email : ''}
                    {invitation.firstname || invitation.lastname ? ' · ' : ''}
                    {role.label}
                    {invitation.client !== null && ` · ${invitation.client.name}`}
                    {invitation.invited_by !== '' && ` · par ${invitation.invited_by}`}
                  </span>
                </div>

                <span
                  className={cn(
                    'rounded-full px-2 py-0.5 text-[12px] whitespace-nowrap',
                    expired ? 'bg-[#fdecec] text-[#c4333a]' : 'bg-[#fff1d4] text-[#7a5300]',
                  )}
                >
                  {expired
                    ? `Expirée le ${DATE.format(new Date(invitation.expires_at))}`
                    : `En attente · jusqu’au ${DATE.format(new Date(invitation.expires_at))}`}
                </span>

                {canManage && (
                  <div className="flex items-center gap-1.5">
                    <Button
                      size="sm"
                      variant={expired ? 'default' : 'outline'}
                      disabled={resend.isPending}
                      onClick={() => doResend(invitation)}
                    >
                      Renvoyer
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setCancelling(invitation)}>
                      Annuler
                    </Button>
                  </div>
                )}
              </li>
            )
          })}
        </ul>
      )}

      <Dialog open={link !== null} onOpenChange={(open) => !open && setLink(null)}>
        <DialogContent className="sm:max-w-[520px]">
          {link !== null && (
            <>
              <DialogHeader>
                <DialogTitle>Invitation renvoyée</DialogTitle>
                <DialogDescription>
                  {link.emailed
                    ? `Un nouvel e-mail est parti pour ${link.email}. L’ancien lien ne fonctionne plus.`
                    : `Transmettez ce nouveau lien à ${link.email}. L’ancien ne fonctionne plus.`}
                </DialogDescription>
              </DialogHeader>
              <LinkBox link={link.link} expiresAt={link.expires_at} />
              <DialogFooter>
                <Button size="lg" onClick={() => setLink(null)}>
                  Terminer
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={cancelling !== null} onOpenChange={(open) => !open && setCancelling(null)}>
        <DialogContent className="sm:max-w-[440px]">
          {cancelling !== null && (
            <>
              <DialogHeader>
                <DialogTitle>Annuler l’invitation ?</DialogTitle>
                <DialogDescription>
                  Le lien envoyé à {cancelling.email} cessera de fonctionner. Vous pourrez inviter
                  cette adresse de nouveau plus tard.
                </DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <Button variant="ghost" size="lg" onClick={() => setCancelling(null)}>
                  Garder
                </Button>
                <Button
                  variant="destructive"
                  size="lg"
                  disabled={revoke.isPending}
                  onClick={() =>
                    revoke.mutate(cancelling.id, {
                      onSuccess: () => {
                        toast.success('Invitation annulée')
                        setCancelling(null)
                      },
                      onError: (error) =>
                        toast.error(error instanceof HttpError ? error.message : 'Annulation impossible'),
                    })
                  }
                >
                  Annuler l’invitation
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
