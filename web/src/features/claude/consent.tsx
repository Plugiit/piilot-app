import { useState } from 'react'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import type { OAuthConsent } from '@/types/api'

import { NEVER, SCOPE_TEXT } from './format'

/**
 * Le consentement a une connexion de Claude.
 *
 * L'ecran nomme qui demande d'apres l'hote qui publie son identite, pas d'apres
 * le nom que le client se donne, et dit ou partira le code. Un retour vers la
 * machine locale est signale : tout programme qui y ecoute pourrait se faire
 * passer pour Claude Code.
 */
export function ConsentForm({
  request,
  email,
  pending,
  onDecide,
}: {
  request: OAuthConsent
  email: string
  pending: boolean
  onDecide: (approve: boolean, write: boolean) => void
}) {
  const asksWrite = request.scopes.includes('piilot:write')
  const [write, setWrite] = useState(asksWrite)

  return (
    <div className="flex flex-col gap-5">
      <p className="text-[15px] leading-[1.6] text-[#4b4b4f]">
        <strong className="font-medium text-[#1b1b1b]">{request.client}</strong>
        {request.surface !== request.client && ` (${request.surface})`} demande à agir dans
        Piilot avec votre compte <strong className="font-medium text-[#1b1b1b]">{email}</strong>, et vos droits seulement.
      </p>

      <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-[13px]">
        <dt className="text-[#73757c]">Identité publiée par</dt>
        <dd className="font-mono text-[#1b1b1b]">{request.host}</dd>
        <dt className="text-[#73757c]">Le code partira vers</dt>
        <dd className="font-mono text-[#1b1b1b]">{request.redirect_host}</dd>
      </dl>

      {request.local ? (
        <p
          role="note"
          className="rounded-[10px] border border-[#ffe8b7] bg-[#fff8e6] px-3 py-2 text-[13px] leading-[1.5] text-[#7a5300]"
        >
          Retour vers cet ordinateur. N’acceptez que si vous venez de lancer la connexion depuis Claude Code, sur cette
          machine.
        </p>
      ) : (
        <p role="note" className="rounded-[10px] bg-[#f7f7f8] px-3 py-2 text-[13px] leading-[1.5] text-[#4b4b4f]">
          N’acceptez que si vous venez de lancer cette connexion vous-même, depuis Claude.
        </p>
      )}

      <fieldset className="flex flex-col gap-3">
        <legend className="pb-2 text-[14px] font-medium text-[#1b1b1b]">Claude pourra</legend>
        <ScopeLine id="scope-read" checked disabled label={SCOPE_TEXT['piilot:read'].label} detail={SCOPE_TEXT['piilot:read'].detail} />
        {asksWrite && (
          <ScopeLine
            id="scope-write"
            checked={write}
            onChange={setWrite}
            label={SCOPE_TEXT['piilot:write'].label}
            detail={`${SCOPE_TEXT['piilot:write'].detail} Les suppressions sont signalées à Claude comme irréversibles : il vous demande votre accord, sauf si vous les avez toujours autorisées.`}
          />
        )}
      </fieldset>

      <div className="flex flex-col gap-2">
        <p className="text-[14px] font-medium text-[#1b1b1b]">Claude ne pourra jamais</p>
        <ul className="flex list-disc flex-col gap-1 pl-5 text-[13px] leading-[1.5] text-[#4b4b4f]">
          {NEVER.map((line) => (
            <li key={line}>{line}</li>
          ))}
        </ul>
      </div>

      <div className="grid grid-cols-2 gap-2">
        <Button type="button" size="lg" variant="outline" disabled={pending} onClick={() => onDecide(false, false)}>
          Refuser
        </Button>
        <Button type="button" size="lg" disabled={pending} onClick={() => onDecide(true, write)}>
          {pending ? 'Un instant…' : 'Autoriser'}
        </Button>
      </div>
    </div>
  )
}

function ScopeLine({
  id,
  label,
  detail,
  checked,
  disabled = false,
  onChange,
}: {
  id: string
  label: string
  detail: string
  checked: boolean
  disabled?: boolean
  onChange?: (value: boolean) => void
}) {
  return (
    <div className="flex items-start gap-3">
      <Checkbox
        id={id}
        checked={checked}
        disabled={disabled}
        onCheckedChange={(value) => onChange?.(value === true)}
        className="mt-0.5 size-5 rounded-[6px]"
        aria-describedby={`${id}-detail`}
      />
      <label htmlFor={id} className="flex flex-col gap-0.5">
        <span className="text-[14px] text-[#1b1b1b]">
          {label}
          {disabled && <span className="text-[#73757c]"> · toujours</span>}
        </span>
        <span id={`${id}-detail`} className="text-[13px] leading-[1.5] text-[#73757c]">
          {detail}
        </span>
      </label>
    </div>
  )
}
