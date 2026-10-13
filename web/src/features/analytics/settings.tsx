import { Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { CHAMP, Card, Choices, Field, SaveBar } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { analyticsWebsitesQuery, useUpdateAnalyticsSettings, type AnalyticsSettingsValues } from '@/features/analytics/api'
import { HttpError } from '@/lib/api'
import type { AnalyticsSettings } from '@/types/api'

/**
 * Les cartes de la page du module de statistiques : la connexion a Umami,
 * et les sites qu'il mesure.
 */

function valuesOf(s: AnalyticsSettings): AnalyticsSettingsValues {
  return {
    enabled: s.enabled,
    base_url: s.base_url_from_env ? '' : s.base_url,
    auth_mode: s.auth_mode,
    username: s.auth_from_env ? '' : s.username,
    api_version: s.version_pinned ? (s.api_version as 2 | 3) : 0,
    cache_minutes: s.cache_minutes,
  }
}

export function AnalyticsSettingsForm({ settings }: { settings: AnalyticsSettings }) {
  const update = useUpdateAnalyticsSettings()
  const initial = valuesOf(settings)
  const [values, setValues] = useState(initial)
  const [password, setPassword] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const dirty = JSON.stringify(values) !== JSON.stringify(initial) || password !== '' || apiKey !== ''
  const authLocked = settings.auth_from_env
  const mode = authLocked ? settings.auth_mode : values.auth_mode

  useEffect(() => setErrors({}), [values])

  function set<K extends keyof AnalyticsSettingsValues>(key: K, value: AnalyticsSettingsValues[K]) {
    setValues((v) => ({ ...v, [key]: value }))
  }

  return (
    <form
      noValidate
      className="flex flex-col gap-3"
      onSubmit={(event) => {
        event.preventDefault()
        update.mutate(
          { ...values, ...(password !== '' ? { password } : {}), ...(apiKey !== '' ? { api_key: apiKey } : {}) },
          {
            onSuccess: () => {
              setPassword('')
              setApiKey('')
              toast.success('Réglages enregistrés : vérification demandée')
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
        title="Umami"
        description="Piilot lit les statistiques de votre instance Umami, ou d’Umami Cloud, et les montre à chaque client pour le site de production de son projet. Les secrets sont chiffrés en base et ne quittent jamais le serveur."
      >
        <label className="flex items-center gap-3 px-1 text-[15px] text-[#1b1b1b]">
          <Checkbox checked={values.enabled} onCheckedChange={(v) => set('enabled', v === true)} />
          Activer le module
        </label>
        <Field label="Hébergement" hint={authLocked ? 'Fixé par les variables UMAMI_* de l’environnement.' : undefined}>
          <Choices
            value={mode}
            onChange={(v) => !authLocked && set('auth_mode', v)}
            options={[
              { value: 'login', label: 'Instance auto-hébergée' },
              { value: 'api_key', label: 'Umami Cloud' },
            ]}
          />
        </Field>
        <Field
          label="Adresse"
          hint={
            settings.base_url_from_env
              ? `Fixée par UMAMI_URL : ${settings.base_url}`
              : mode === 'api_key'
                ? 'Pour Umami Cloud : https://api.umami.is/v1'
                : 'La racine de l’instance, sans /api.'
          }
          error={errors.base_url}
        >
          <Input
            className={CHAMP}
            type="url"
            placeholder={mode === 'api_key' ? 'https://api.umami.is/v1' : 'https://stats.votre-agence.fr'}
            value={settings.base_url_from_env ? settings.base_url : values.base_url}
            disabled={settings.base_url_from_env}
            onChange={(e) => set('base_url', e.target.value)}
          />
        </Field>
        {mode === 'login' ? (
          <div className="flex flex-col gap-3 sm:flex-row">
            <div className="min-w-0 flex-1">
              <Field label="Identifiant" error={errors.username}>
                <Input
                  className={CHAMP}
                  autoComplete="off"
                  value={authLocked ? settings.username : values.username}
                  disabled={authLocked}
                  onChange={(e) => set('username', e.target.value)}
                />
              </Field>
            </div>
            <div className="min-w-0 flex-1">
              <Field label="Mot de passe" hint={settings.password_set ? 'Renseigné. Laissez vide pour le garder.' : 'Un compte en lecture suffit.'}>
                <Input
                  className={CHAMP}
                  type="password"
                  autoComplete="new-password"
                  placeholder={settings.password_set ? '••••••••' : ''}
                  value={password}
                  disabled={authLocked}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </Field>
            </div>
          </div>
        ) : (
          <Field label="Clé d’API" hint={settings.api_key_set ? 'Renseignée. Laissez vide pour la garder.' : 'Umami Cloud → Paramètres → Clés API.'}>
            <Input
              className={CHAMP}
              type="password"
              autoComplete="new-password"
              placeholder={settings.api_key_set ? '••••••••' : ''}
              value={apiKey}
              disabled={authLocked}
              onChange={(e) => setApiKey(e.target.value)}
            />
          </Field>
        )}
        <Field
          label="Version d’Umami"
          hint={
            values.api_version === 0
              ? settings.api_version === 0
                ? 'Détectée à la première vérification.'
                : `Détectée : ${settings.api_version}.x. Une mise à jour d’Umami est reconnue d’elle-même.`
              : 'Choisie à la main : à n’utiliser que si la détection échoue.'
          }
          error={errors.api_version}
        >
          <Choices
            value={String(values.api_version) as '0' | '2' | '3'}
            onChange={(v) => set('api_version', Number(v) as 0 | 2 | 3)}
            options={[
              { value: '0', label: 'Automatique' },
              { value: '2', label: '2.x' },
              { value: '3', label: '3.x' },
            ]}
          />
        </Field>
        <Field
          label="Fraîcheur des chiffres"
          hint="Les sept et trente derniers jours et le mois en cours sont relus à ce rythme ; le mois précédent et les douze mois, une fois par jour."
          error={errors.cache_minutes}
        >
          <Choices
            value={String(values.cache_minutes) as '5' | '10' | '15'}
            onChange={(v) => set('cache_minutes', Number(v))}
            options={[
              { value: '5', label: '5 min' },
              { value: '10', label: '10 min' },
              { value: '15', label: '15 min' },
            ]}
          />
        </Field>
        <SaveBar
          dirty={dirty}
          pending={update.isPending}
          onCancel={() => {
            setValues(initial)
            setPassword('')
            setApiKey('')
          }}
        />
      </Card>
    </form>
  )
}

const PAGE_SIZE = 25

export function AnalyticsWebsitesCard() {
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const { data } = useQuery(analyticsWebsitesQuery(query.trim(), page, PAGE_SIZE))
  const pages = Math.max(1, Math.ceil((data?.total ?? 0) / PAGE_SIZE))

  return (
    <Card title="Sites" description="Les sites mesurés par Umami, tels que la dernière vérification les a vus. Liez-les aux projets depuis leurs paramètres.">
      <div className="relative">
        <HugeiconsIcon icon={Search01Icon} size={16} strokeWidth={1.6} className="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-[#8d8d8d]" />
        <Input
          type="search"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setPage(1)
          }}
          placeholder="Rechercher un site"
          aria-label="Rechercher un site"
          className="h-9 pl-8 text-[13px]"
        />
      </div>
      {data !== undefined && data.items.length === 0 ? (
        <p className="py-6 text-center text-[13px] text-[#73757c]">
          {query.trim() === '' ? 'Aucun site pour l’instant : testez la connexion.' : 'Aucun site ne correspond.'}
        </p>
      ) : (
        <ul className="flex flex-col">
          {(data?.items ?? []).map((w) => (
            <li key={w.id} className="flex items-center gap-3 border-b border-[#f3f4f4] py-2 last:border-b-0">
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="flex items-center gap-2 truncate text-[14px] text-[#1b1b1b]">
                  {w.name}
                  {!w.present && <span className="text-[12px] text-[#a2a3a7]">supprimé d’Umami</span>}
                </span>
                <span className="truncate text-[12px] text-[#8d8d8d]">{w.domain}</span>
              </span>
              <span className="truncate text-right text-[12px] text-[#73757c]">
                {w.projects.length === 0 ? 'Aucun projet' : w.projects.join(', ')}
              </span>
            </li>
          ))}
        </ul>
      )}
      {pages > 1 && (
        <div className="flex items-center justify-end gap-2 text-[13px] text-[#73757c]">
          Page {page} sur {pages}
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
            Précédent
          </Button>
          <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => setPage((p) => p + 1)}>
            Suivant
          </Button>
        </div>
      )}
    </Card>
  )
}
