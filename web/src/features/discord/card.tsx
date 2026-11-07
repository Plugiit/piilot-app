import { useQuery } from '@tanstack/react-query'
import { type ReactNode, useEffect, useState } from 'react'
import { toast } from 'sonner'

import discordUrl from '@/assets/integrations/discord.svg'
import { CopyLine } from '@/components/copy-line'
import { CARD_ACTION, IntegrationCard } from '@/components/integration-card'
import { Button } from '@/components/ui/button'
import { StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { useConfirmFocus } from '@/lib/use-confirm-focus'

import { discordStatusQuery, useDiscordActions } from './api'
import { discordCard, formatDay, formatTime, linkedSince, type PendingCode } from './format'

const LINKED = { label: 'Lié', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } }
const WAITING = { label: 'En attente', color: '#eab308', pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' } }

// Le pied des cartes d'integration est d'ordinaire un lien ; en bouton, il
// lui faut un etat desactive.
const ACTION = `${CARD_ACTION} cursor-pointer disabled:pointer-events-none disabled:opacity-50`

const fail = (fallback: string) => (error: unknown) => toast.error(error instanceof HttpError ? error.message : fallback)

/**
 * La carte « Discord » du compte : lier son compte au serveur de l'agence,
 * pour que le bot y donne les roles qui vont avec.
 *
 * Piilot ne parle jamais a Discord. La carte tire un code, le membre le tape
 * dans Discord, le bot le rachete ; la carte le voit en relisant la liaison.
 */
export function DiscordCard() {
  const [pending, setPending] = useState<PendingCode | null>(null)
  const [now, setNow] = useState(() => new Date())
  const [confirming, setConfirming] = useState(false)
  const status = useQuery(discordStatusQuery(pending))
  const { createCode, unlink } = useDiscordActions()
  const card = discordCard(status.data, status.isError, pending, now)
  const confirmOpen = confirming && card.kind === 'linked'
  const { panelRef, actionsRef } = useConfirmFocus(confirmOpen)
  const redeemed = pending !== null && linkedSince(status.data, pending)

  // Rien ne change ailleurs quand le code expire : un minuteur fait basculer
  // la carte.
  useEffect(() => {
    if (pending === null) return
    const timer = window.setTimeout(() => setNow(new Date()), Math.max(0, pending.expiresAt.getTime() - Date.now()) + 100)
    return () => window.clearTimeout(timer)
  }, [pending])

  // Le bot a rachete le code : la carte repasse d'elle-meme a la liaison, il
  // reste a le dire.
  useEffect(() => {
    if (redeemed) toast.success('Compte Discord lié')
  }, [redeemed])

  function draw() {
    const before = { discord_user_id: status.data?.discord_user_id ?? null, linked_at: status.data?.linked_at ?? null }
    createCode.mutate(undefined, {
      onSuccess: (code) => {
        setNow(new Date())
        setPending({ code: code.code, expiresAt: new Date(code.expires_at), before })
      },
      onError: fail('Le code n’a pas pu être créé'),
    })
  }

  const drawButton = (label: string) => (
    <button type="button" className={ACTION} disabled={createCode.isPending} onClick={draw}>
      {createCode.isPending ? 'Création du code…' : label}
    </button>
  )

  let description: ReactNode
  let action: ReactNode
  let badge: ReactNode = null

  switch (card.kind) {
    case 'loading':
      description = 'Vos rôles sur le serveur Discord de l’agence.'
      action = <div aria-hidden className="h-10 rounded-[12px] bg-[#f3f4f4]" />
      break

    case 'error':
      description = 'La liaison n’a pas pu être lue.'
      action = (
        <button type="button" className={ACTION} disabled={status.isFetching} onClick={() => void status.refetch()}>
          Réessayer
        </button>
      )
      break

    case 'unavailable':
      description = 'Le bot du serveur Discord de l’agence n’est pas encore branché. La liaison ouvrira dès qu’il le sera.'
      action = (
        <p className="flex h-10 items-center justify-center rounded-[12px] border border-dashed border-[#e8e8e9] text-[14px] font-medium text-[#a2a3a7]">
          Bot non branché
        </p>
      )
      break

    case 'unlinked':
      description = 'Liez votre compte pour recevoir vos rôles sur le serveur Discord de l’agence. Ils suivent ensuite votre compte Piilot.'
      action = drawButton('Lier mon compte')
      break

    case 'waiting':
      badge = <StatusPill {...WAITING} />
      description = (
        <>
          Dans Discord, tapez <code className="font-mono text-[#1b1b1b]">/lier</code> et collez ce code. Valable jusqu’à{' '}
          {formatTime(card.expiresAt)}, une seule fois.
        </>
      )
      action = (
        <div className="flex flex-col gap-2">
          <CopyLine value={card.code} />
          {drawButton('Nouveau code')}
        </div>
      )
      break

    case 'expired':
      description = 'Le code a expiré sans avoir servi.'
      action = (
        <div className="grid grid-cols-2 gap-2">
          <button type="button" className={ACTION} onClick={() => setPending(null)}>
            Annuler
          </button>
          {drawButton('Nouveau code')}
        </div>
      )
      break

    case 'linked':
      badge = <StatusPill {...LINKED} />
      description = (
        <>
          Lié au compte Discord <span className="font-mono text-[#1b1b1b]">{card.discordUserId}</span> depuis le{' '}
          {formatDay(card.linkedAt)}. Vos rôles sur le serveur suivent votre compte Piilot.
        </>
      )
      action = (
        <div className={card.canRelink ? 'grid grid-cols-2 gap-2' : 'grid'}>
          {card.canRelink && drawButton('Changer')}
          <button type="button" className={ACTION} onClick={() => setConfirming(true)}>
            Délier
          </button>
        </div>
      )
      break
  }

  // La confirmation prend la place des gestes. Le focus y entre, puis revient
  // aux gestes, quels qu'ils soient apres la deliaison.
  const footer = confirmOpen ? (
    <div
      ref={panelRef}
      tabIndex={-1}
      role="alertdialog"
      aria-label="Confirmer la déliaison"
      className="flex flex-col gap-2 text-[13px] text-[#4b4b4f] outline-none"
    >
      <span>Le bot vous retirera vos rôles sur le serveur.</span>
      <div className="grid grid-cols-2 gap-2">
        <Button type="button" variant="ghost" onClick={() => setConfirming(false)}>
          Annuler
        </Button>
        <Button
          type="button"
          variant="destructive"
          disabled={unlink.isPending}
          onClick={() =>
            unlink.mutate(undefined, {
              onSuccess: () => {
                // Le code qui avait servi ne doit pas reparaitre.
                setPending(null)
                setConfirming(false)
                toast.success('Compte Discord délié')
              },
              onError: fail('La liaison n’a pas pu être supprimée'),
            })
          }
        >
          Délier
        </Button>
      </div>
    </div>
  ) : (
    <div ref={actionsRef} className="flex flex-col">
      {action}
    </div>
  )

  return (
    <IntegrationCard
      logo={discordUrl}
      name="Discord"
      badge={badge}
      // Le bot lie le compte hors de la page : le changement s'annonce.
      description={<span aria-live="polite">{description}</span>}
      action={footer}
    />
  )
}
