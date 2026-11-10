import { useQuery } from '@tanstack/react-query'
import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'

import claudeUrl from '@/assets/integrations/claude.svg'
import { CopyLine } from '@/components/copy-line'
import { CARD_ACTION, IntegrationCard } from '@/components/integration-card'
import { Button } from '@/components/ui/button'
import { StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { useConfirmFocus } from '@/lib/use-confirm-focus'
import type { ClaudeConnection } from '@/types/api'

import { claudeQuery, useDisconnectClaude } from './api'
import { claudeCard, claudeCodeCommand, formatDay, formatLastUse, scopesLabel } from './format'

const CONNECTED = { label: 'Connecté', color: '#0db471', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } }

/**
 * La carte « Claude » du compte : l'adresse du serveur MCP a saisir dans
 * Claude, et l'etat des connexions. Le detail des connexions vit sous la
 * grille, ou il a la place d'une liste.
 */
export function ClaudeCard() {
  const account = useQuery(claudeQuery)
  const card = claudeCard(account.data, account.isError)

  let description: ReactNode
  let action: ReactNode
  let badge: ReactNode = null

  switch (card.kind) {
    case 'loading':
      description = 'Laissez Claude travailler dans Piilot, avec les droits de votre compte.'
      action = <div aria-hidden className="h-9 rounded-[8px] bg-[#f3f4f4]" />
      break

    case 'error':
      description = 'Les connexions n’ont pas pu être lues.'
      action = (
        <button
          type="button"
          className={`${CARD_ACTION} cursor-pointer disabled:pointer-events-none disabled:opacity-50`}
          disabled={account.isFetching}
          onClick={() => void account.refetch()}
        >
          Réessayer
        </button>
      )
      break

    case 'idle':
      description = 'Ajoutez cette adresse comme connecteur dans Claude : il agira avec vos droits, après votre accord.'
      action = <CopyLine value={card.mcpUrl} />
      break

    case 'connected':
      badge = <StatusPill {...CONNECTED} />
      description = (
        <>
          {card.count === 1 ? 'Une connexion' : `${card.count} connexions`} en cours.{' '}
          {card.lastUsedAt === null ? 'Pas encore utilisée.' : `Dernier appel le ${formatDay(card.lastUsedAt)}.`}
        </>
      )
      action = <CopyLine value={card.mcpUrl} />
      break
  }

  return (
    <IntegrationCard
      logo={claudeUrl}
      name="Claude"
      badge={badge}
      description={<span aria-live="polite">{description}</span>}
      action={action}
    />
  )
}

/**
 * Sous la grille : comment connecter Claude, et les connexions en cours, que
 * l'on coupe d'un geste.
 */
export function ClaudeConnections() {
  const { data: account } = useQuery(claudeQuery)
  if (account === undefined) return null

  return (
    <div className="flex w-full flex-col gap-4 rounded-[12px] border border-[#e8e8e9] bg-white p-4">
      <div className="grid gap-4 md:grid-cols-2">
        <Steps title="claude.ai et Claude Desktop">
          <li>
            Dans Claude : <span className="text-[#1b1b1b]">Personnaliser › Connecteurs › Ajouter un connecteur personnalisé</span>.
          </li>
          <li>Collez l’adresse de la carte, puis connectez-vous à Piilot et autorisez.</li>
          <li>Desktop reprend le connecteur de votre compte claude.ai.</li>
        </Steps>
        <Steps title="Claude Code">
          <li>
            <div className="flex flex-col gap-1.5">
              Dans un terminal :
              <CopyLine value={claudeCodeCommand(account.mcp_url)} />
            </div>
          </li>
          <li>
            Puis <code className="font-mono text-[#1b1b1b]">/mcp</code> dans Claude Code pour vous connecter.
          </li>
        </Steps>
      </div>

      {account.connections.length > 0 && (
        <div className="flex flex-col">
          <h3 className="pb-2 text-[14px] font-medium text-[#1b1b1b]">Connexions en cours</h3>
          <ul className="flex flex-col divide-y divide-[#f0f0f1] border-t border-[#f0f0f1]">
            {account.connections.map((connection) => (
              <ConnectionRow key={connection.id} connection={connection} />
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

function Steps({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <h3 className="text-[14px] font-medium text-[#1b1b1b]">{title}</h3>
      <ol className="flex list-decimal flex-col gap-1.5 pl-5 text-[13px] leading-[1.5] text-[#4b4b4f]">{children}</ol>
    </div>
  )
}

function ConnectionRow({ connection }: { connection: ClaudeConnection }) {
  const [confirming, setConfirming] = useState(false)
  const disconnect = useDisconnectClaude()
  const { panelRef, actionsRef } = useConfirmFocus(confirming)

  return (
    <li className="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex min-w-0 flex-col gap-0.5">
        <span className="text-[14px] text-[#1b1b1b]">
          {connection.client}
          <span className="text-[#73757c]"> · {scopesLabel(connection.scopes)}</span>
        </span>
        <span className="text-[12px] text-[#73757c]">
          {connection.surface !== connection.client && `${connection.surface} · `}
          Connectée le {formatDay(new Date(connection.created_at))} · {formatLastUse(connection.last_used_at)}
        </span>
      </div>

      {confirming ? (
        <div
          ref={panelRef}
          tabIndex={-1}
          role="alertdialog"
          aria-label={`Déconnecter ${connection.client}`}
          className="flex flex-col gap-2 text-[13px] text-[#4b4b4f] outline-none sm:items-end"
        >
          <span>Claude perdra l’accès dès sa prochaine requête.</span>
          <div className="flex gap-2">
            <Button type="button" size="sm" variant="ghost" onClick={() => setConfirming(false)}>
              Annuler
            </Button>
            <Button
              type="button"
              size="sm"
              variant="destructive"
              disabled={disconnect.isPending}
              onClick={() =>
                disconnect.mutate(connection.id, {
                  onSuccess: () => toast.success(`${connection.client} déconnecté`),
                  onError: (error) =>
                    toast.error(error instanceof HttpError ? error.message : 'La connexion n’a pas pu être coupée'),
                })
              }
            >
              Déconnecter
            </Button>
          </div>
        </div>
      ) : (
        <div ref={actionsRef} className="flex shrink-0">
          <Button type="button" size="sm" variant="outline" onClick={() => setConfirming(true)}>
            Déconnecter
          </Button>
        </div>
      )}
    </li>
  )
}
