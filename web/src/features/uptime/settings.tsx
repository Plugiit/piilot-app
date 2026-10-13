import { RefreshIcon, Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { CopyLine } from '@/components/copy-line'
import { CHAMP, Card, Choices, Field, SaveBar } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  uptimeIncidentsQuery,
  uptimeMonitorsQuery,
  useRotateUptimeSecret,
  useUpdateUptimeSettings,
  type UptimeSettingsValues,
} from '@/features/uptime/api'
import { formatUptime } from '@/features/uptime/format'
import { IncidentList, MonitorStatus } from '@/features/uptime/ui'
import { HttpError } from '@/lib/api'
import type { UptimeSettings } from '@/types/api'

/**
 * Les cartes de la page du module de disponibilite, une par onglet ou
 * presque : la connexion et le webhook, les sites, les incidents.
 */

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

function valuesOf(s: UptimeSettings): UptimeSettingsValues {
  return { enabled: s.enabled, base_url: s.base_url_from_env ? '' : s.base_url, poll_minutes: s.poll_minutes }
}

export function SettingsForm({ settings }: { settings: UptimeSettings }) {
  const update = useUpdateUptimeSettings()
  const initial = valuesOf(settings)
  const [values, setValues] = useState(initial)
  const [apiKey, setApiKey] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const dirty = JSON.stringify(values) !== JSON.stringify(initial) || apiKey !== ''

  useEffect(() => setErrors({}), [values])

  function set<K extends keyof UptimeSettingsValues>(key: K, value: UptimeSettingsValues[K]) {
    setValues((v) => ({ ...v, [key]: value }))
  }

  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        update.mutate(
          { ...values, ...(apiKey !== '' ? { api_key: apiKey } : {}) },
          {
            onSuccess: () => {
              setApiKey('')
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
        title="Uptime Kuma"
        description="Piilot lit la page /metrics de votre instance. Créez une clé dans Kuma, Paramètres → Clés API, et collez-la ici : elle est chiffrée en base."
      >
        <label className="flex items-center gap-3 px-1 text-[15px] text-[#1b1b1b]">
          <Checkbox checked={values.enabled} onCheckedChange={(v) => set('enabled', v === true)} />
          Activer le module
        </label>
        <Field
          label="Adresse de Kuma"
          hint={settings.base_url_from_env ? `Fixée par KUMA_URL : ${settings.base_url}` : 'Sans /metrics : Piilot l’ajoute.'}
          error={errors.base_url}
        >
          <Input
            className={CHAMP}
            type="url"
            placeholder="https://status.votre-agence.fr"
            value={settings.base_url_from_env ? settings.base_url : values.base_url}
            disabled={settings.base_url_from_env}
            onChange={(e) => set('base_url', e.target.value)}
          />
        </Field>
        <Field
          label="Clé d’API"
          hint={
            settings.api_key_from_env
              ? 'Fixée par KUMA_API_KEY dans l’environnement.'
              : settings.api_key_set
                ? 'Renseignée. Laissez vide pour la garder.'
                : 'Facultative si /metrics est ouvert.'
          }
        >
          <Input
            className={CHAMP}
            type="password"
            autoComplete="new-password"
            placeholder={settings.api_key_set ? '••••••••' : 'uk2_…'}
            value={apiKey}
            disabled={settings.api_key_from_env}
            onChange={(e) => setApiKey(e.target.value)}
          />
        </Field>
        <Field
          label="Intervalle de relève"
          hint="Chaque relève ajoute un contrôle par site à l’historique du jour. Une relève ratée ne compte jamais comme une panne."
          error={errors.poll_minutes}
        >
          <Choices
            value={String(values.poll_minutes) as '1' | '2' | '3' | '5'}
            onChange={(v) => set('poll_minutes', Number(v))}
            options={[
              { value: '1', label: '1 min' },
              { value: '2', label: '2 min' },
              { value: '3', label: '3 min' },
              { value: '5', label: '5 min' },
            ]}
          />
        </Field>
        <SaveBar
          dirty={dirty}
          pending={update.isPending}
          onCancel={() => {
            setValues(initial)
            setApiKey('')
          }}
        />
      </Card>
    </form>
  )
}

export function WebhookCard({ settings }: { settings: UptimeSettings }) {
  const rotate = useRotateUptimeSecret()
  const [confirm, setConfirm] = useState(false)

  return (
    <Card
      title="Webhook"
      description="Facultatif, recommandé : Kuma prévient Piilot à chaque changement d’état, avec l’heure exacte et la raison de la panne. L’adresse porte le secret, gardez-la pour vous."
    >
      <Field
        label="Adresse à déclarer dans Kuma"
        hint="Kuma → Paramètres → Notifications → Configurer une notification : type « Webhook », corps « application/json », cochez « Appliquer à tous les moniteurs existants »."
      >
        <CopyLine value={settings.webhook_url} secret />
      </Field>
      <p className="px-1 text-[13px] text-[#73757c]">
        {settings.status.webhook_last_at === null
          ? 'Aucun évènement reçu pour l’instant : les pannes sont déduites des relèves, à l’intervalle près.'
          : `Dernier évènement reçu : ${WHEN.format(new Date(settings.status.webhook_last_at))}.`}
      </p>
      <div className="flex flex-wrap items-center gap-2 px-1">
        {!confirm ? (
          <Button type="button" variant="ghost" size="sm" className="gap-1.5" onClick={() => setConfirm(true)}>
            <HugeiconsIcon icon={RefreshIcon} size={14} strokeWidth={1.8} />
            Changer le secret
          </Button>
        ) : (
          <span className="flex items-center gap-2 text-[13px] text-[#73757c]">
            Le webhook déclaré dans Kuma cessera d’être accepté.
            <Button
              type="button"
              size="sm"
              disabled={rotate.isPending}
              onClick={() =>
                rotate.mutate(undefined, {
                  onSuccess: () => {
                    setConfirm(false)
                    toast.success('Nouveau secret : mettez à jour l’adresse dans Kuma.')
                  },
                })
              }
            >
              Confirmer
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={() => setConfirm(false)}>
              Annuler
            </Button>
          </span>
        )}
      </div>
    </Card>
  )
}

const PAGE_SIZE = 25

export function MonitorsCard() {
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const { data } = useQuery(uptimeMonitorsQuery(query.trim(), page, PAGE_SIZE))
  const pages = Math.max(1, Math.ceil((data?.total ?? 0) / PAGE_SIZE))

  return (
    <Card title="Sites" description="Tels que la dernière relève les a vus. Un site retiré de Kuma reste ici avec son historique, marqué « retiré ».">
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
          {query.trim() === '' ? 'Aucun site relevé pour l’instant.' : 'Aucun site ne correspond.'}
        </p>
      ) : (
        <ul className="flex flex-col">
          {(data?.items ?? []).map((m) => (
            <li key={m.id} className="flex items-center gap-3 border-b border-[#f3f4f4] py-2 last:border-b-0">
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="flex items-center gap-2 truncate text-[14px] text-[#1b1b1b]">
                  {m.name}
                  {!m.present && <span className="text-[12px] text-[#a2a3a7]">retiré</span>}
                </span>
                <span className="truncate text-[12px] text-[#8d8d8d]">
                  {m.url !== '' ? m.url : m.hostname !== '' ? m.hostname : m.kind}
                  {m.projects.length > 0 && ` · ${m.projects.join(', ')}`}
                </span>
              </span>
              <span className="hidden w-[90px] shrink-0 text-right text-[12px] text-[#73757c] tabular-nums md:block">
                {m.avg_ping_30d === null ? '' : `${m.avg_ping_30d} ms`}
              </span>
              <span
                className={`hidden w-[70px] shrink-0 text-right text-[12px] tabular-nums md:block ${m.cert_days !== null && m.cert_days < 14 ? 'font-medium text-[#e5484d]' : 'text-[#73757c]'}`}
                title="Jours avant l’expiration du certificat"
              >
                {m.cert_days === null ? '' : `${m.cert_days} j`}
              </span>
              <span className="w-[72px] shrink-0 text-right text-[13px] font-medium text-[#1b1b1b] tabular-nums" title="Disponibilité sur trente jours">
                {formatUptime(m.uptime_30d)}
              </span>
              <MonitorStatus status={m.status} />
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

export function IncidentsCard() {
  const [page, setPage] = useState(1)
  const { data } = useQuery(uptimeIncidentsQuery(page))
  const pages = Math.max(1, Math.ceil((data?.total ?? 0) / 10))

  return (
    <Card title="Incidents" description="De la première alerte au retour en ligne. Gardés sans limite.">
      <IncidentList items={data?.items ?? []} />
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
