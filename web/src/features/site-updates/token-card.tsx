import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'

import { CHAMP, Card, Field } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { actionsTokenQuery, useActionsToken } from '@/features/site-updates/api'
import { HttpError } from '@/lib/api'

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })

/**
 * Le jeton qui permet a Piilot de demander le lancement d'un workflow. Il se
 * saisit et se remplace, il ne se relit jamais.
 */
export function ActionsTokenCard() {
  const { data } = useQuery(actionsTokenQuery)
  const { save, clear } = useActionsToken()
  const [token, setToken] = useState('')
  const [confirming, setConfirming] = useState(false)
  const fail = (fallback: string) => (e: unknown) => toast.error(e instanceof HttpError ? e.message : fallback)

  return (
    <Card
      title="Lancement des mises à jour"
      description="Jeton GitHub à portée fine, limité aux dépôts des sites, avec la seule permission « Actions : lecture et écriture ». Il sert à lancer le workflow de mise à jour choisi sur chaque projet. Chiffré en base, il ne se réaffiche jamais."
    >
      <p className="px-1 text-[14px] text-[#1b1b1b]">
        {data === undefined ? ' ' : data.configured ? `Jeton enregistré${data.updated_at !== null ? ` le ${WHEN.format(new Date(data.updated_at))}` : ''}.` : 'Aucun jeton : aucune mise à jour ne peut partir.'}
      </p>
      <form
        noValidate
        className="flex flex-col gap-2 sm:flex-row sm:items-end"
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate(token, {
            onSuccess: () => {
              setToken('')
              toast.success('Jeton enregistré')
            },
            onError: fail('Enregistrement impossible'),
          })
        }}
      >
        <div className="min-w-0 flex-1">
          <Field label={data?.configured === true ? 'Remplacer le jeton' : 'Jeton GitHub'}>
            <Input className={CHAMP} type="password" autoComplete="new-password" value={token} onChange={(e) => setToken(e.target.value)} />
          </Field>
        </div>
        <Button type="submit" variant="outline" disabled={token.trim() === '' || save.isPending}>
          Enregistrer
        </Button>
        {data?.configured === true && !confirming && (
          <Button type="button" variant="ghost" onClick={() => setConfirming(true)}>
            Retirer
          </Button>
        )}
      </form>
      {data?.configured === true && confirming && (
        <div role="alert" className="flex flex-col gap-2 rounded-[8px] border border-[#ffe8b7] bg-[#fffaf0] px-3 py-2 text-[13px] text-[#9a6a00] sm:flex-row sm:items-center">
          <span className="min-w-0 sm:flex-1">Sans jeton, plus aucune mise à jour ne peut partir, y compris les planifiées.</span>
          <div className="flex flex-wrap justify-end gap-2">
            <Button type="button" variant="ghost" size="sm" onClick={() => setConfirming(false)}>
              Annuler
            </Button>
            <Button
              type="button"
              variant="destructive"
              size="sm"
              disabled={clear.isPending}
              onClick={() =>
                clear.mutate(undefined, {
                  onSuccess: () => {
                    setConfirming(false)
                    toast.success('Jeton retiré')
                  },
                  onError: fail('Retrait impossible'),
                })
              }
            >
              Retirer le jeton
            </Button>
          </div>
        </div>
      )}
    </Card>
  )
}
