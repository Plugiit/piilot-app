import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { CHAMP, Card, Choices, Field, SaveBar } from '@/components/settings-ui'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { type CoolifySettingsValues, useUpdateCoolifySettings } from '@/features/coolify/api'
import { HttpError } from '@/lib/api'
import type { CoolifySettings } from '@/types/api'

const INTERVALS = [
  { value: '15', label: '15 min' },
  { value: '30', label: '30 min' },
  { value: '60', label: '1 h' },
  { value: '360', label: '6 h' },
  { value: '1440', label: '24 h' },
] as const

function valuesOf(s: CoolifySettings): CoolifySettingsValues {
  return { enabled: s.enabled, base_url: s.base_url_from_env ? '' : s.base_url, poll_minutes: s.poll_minutes }
}

/** La connexion a Coolify : adresse, jeton, intervalle. Le jeton n'est jamais relu. */
export function CoolifyForm({ settings }: { settings: CoolifySettings }) {
  const update = useUpdateCoolifySettings()
  const initial = valuesOf(settings)
  const [values, setValues] = useState(initial)
  const [apiToken, setApiToken] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const dirty = JSON.stringify(values) !== JSON.stringify(initial) || apiToken !== ''

  useEffect(() => setErrors({}), [values])

  function set<K extends keyof CoolifySettingsValues>(key: K, value: CoolifySettingsValues[K]) {
    setValues((v) => ({ ...v, [key]: value }))
  }

  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        update.mutate(
          { ...values, ...(apiToken !== '' ? { api_token: apiToken } : {}) },
          {
            onSuccess: () => {
              setApiToken('')
              toast.success('Réglages enregistrés : relève demandée')
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
      <Card
        title="Coolify"
        description="Piilot lit les applications de votre instance par son API, en lecture seule. Activez l’API dans les réglages de Coolify, créez un jeton en lecture, puis collez-le ici : il est chiffré en base."
      >
        <label className="flex items-center gap-3 px-1 text-[15px] text-[#1b1b1b]">
          <Checkbox checked={values.enabled} onCheckedChange={(v) => set('enabled', v === true)} />
          Activer le module
        </label>
        <Field
          label="Adresse de Coolify"
          hint={settings.base_url_from_env ? `Fixée par COOLIFY_API_URL : ${settings.base_url}` : 'Sans /api/v1 : Piilot l’ajoute.'}
          error={errors.base_url}
        >
          <Input
            className={CHAMP}
            type="url"
            placeholder="https://coolify.votre-agence.fr"
            value={settings.base_url_from_env ? settings.base_url : values.base_url}
            disabled={settings.base_url_from_env}
            onChange={(e) => set('base_url', e.target.value)}
          />
        </Field>
        <Field
          label="Jeton d’API"
          hint={
            settings.api_token_from_env
              ? 'Fixé par COOLIFY_API_TOKEN dans l’environnement.'
              : settings.api_token_set
                ? 'Renseigné. Laissez vide pour le garder.'
                : 'Le jeton complet, avec son préfixe (« 1|… »). Si l’accès est limité par adresse IP, autorisez celle de Piilot.'
          }
        >
          <Input
            className={CHAMP}
            type="password"
            autoComplete="new-password"
            placeholder={settings.api_token_set ? '••••••••' : '1|…'}
            value={apiToken}
            disabled={settings.api_token_from_env}
            onChange={(e) => setApiToken(e.target.value)}
          />
        </Field>
        <Field label="Intervalle de relève" hint="La liste des applications est relue à cet intervalle." error={errors.poll_minutes}>
          <Choices
            value={String(values.poll_minutes) as (typeof INTERVALS)[number]['value']}
            onChange={(v) => set('poll_minutes', Number(v))}
            options={[...INTERVALS]}
          />
        </Field>
        <SaveBar
          dirty={dirty}
          pending={update.isPending}
          onCancel={() => {
            setValues(initial)
            setApiToken('')
          }}
        />
      </Card>
    </form>
  )
}
