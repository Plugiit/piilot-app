import { MailReceive01Icon, RefreshIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, redirect } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { CopyLine } from '@/components/copy-line'
import { PageFrame } from '@/components/layout/page-frame'
import { CHAMP, Card, Choices, Field, SaveBar } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import {
  inboundSettingsQuery,
  useRequestPoll,
  useRotateInboundSecret,
  useUpdateInboundSettings,
  type InboundSettingsValues,
} from '@/features/inbound/api'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import type { InboundSettings } from '@/types/api'

/**
 * Reglage de l'e-mail entrant : l'adresse de support, la boite a relever, et
 * les webhooks des fournisseurs pour qui veut l'instantane.
 *
 * Deux portes, au choix ou ensemble : la releve IMAP marche partout et
 * n'expose rien ; un webhook range l'e-mail en quelques secondes. Un meme
 * e-mail arrive par les deux n'est range qu'une fois.
 */
export const Route = createFileRoute('/_app/parametres/emails/')({
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'users.write')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  component: InboundSettingsPage,
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit', second: '2-digit' })

const PROVIDERS: { key: string; label: string; hint: string }[] = [
  { key: 'postmark', label: 'Postmark', hint: 'Inbound Stream → Settings → Webhook URL. Cochez « Include raw email content » pour garder l’e-mail intact.' },
  { key: 'mailgun', label: 'Mailgun', hint: 'Routes → forward() vers cette adresse suivie de « &mime » : une adresse qui finit par « mime » fait envoyer l’e-mail brut, pièces jointes comprises.' },
  { key: 'brevo', label: 'Brevo', hint: 'Inbound parsing → webhook. Brevo ne transmet pas les pièces jointes au webhook : préférez IMAP si vos clients en envoient.' },
  { key: 'raw', label: 'E-mail brut', hint: 'Le corps de la requête est l’e-mail MIME : Cloudflare Email Workers, CloudMailin en mode brut, un script maison.' },
]

function valuesOf(s: InboundSettings): InboundSettingsValues {
  return {
    address: s.address,
    imap_enabled: s.imap.enabled,
    imap_host: s.imap.host,
    imap_port: s.imap.port,
    imap_security: s.imap.security,
    imap_username: s.imap.username,
    imap_folder: s.imap.folder,
  }
}

function InboundSettingsPage() {
  const { data, isPending } = useQuery(inboundSettingsQuery)

  return (
    <PageFrame title="E-mails entrants">
      <div className="flex flex-col gap-3 p-4">
        {isPending || data === undefined ? (
          <div className="h-[240px] animate-pulse rounded-[12px] bg-[#f3f4f4]" />
        ) : (
          <>
            <StatusCard settings={data} />
            <SettingsForm key={data.address + data.imap.host + data.imap.username} settings={data} />
            <WebhookCard settings={data} />
            <HowItWorks />
          </>
        )}
      </div>
    </PageFrame>
  )
}

function StatusCard({ settings }: { settings: InboundSettings }) {
  const poll = useRequestPoll()
  const s = settings.status

  return (
    <Card title="État" description="Ce que la relève et le rangement ont fait. Les e-mails sont rangés en tâche de fond, toutes les dix secondes.">
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2 px-1 text-[14px] text-[#1b1b1b]">
        <span>
          <strong className="tabular-nums">{s.processed_day}</strong> rangé{s.processed_day > 1 ? 's' : ''} en 24 h
        </span>
        <Link to="/pm/tickets/a-trier" search={{ page: 1 }} className="hover:underline">
          <strong className="tabular-nums">{s.held}</strong> à trier
        </Link>
        {s.pending > 0 && <span className="text-[#73757c]">{s.pending} en cours de rangement</span>}
        {s.failed_week > 0 && <span className="text-[#a30f2c]">{s.failed_week} illisible{s.failed_week > 1 ? 's' : ''} cette semaine</span>}
      </div>
      {settings.imap.enabled && (
        <div className="flex flex-wrap items-center gap-2 px-1 text-[13px] text-[#73757c]">
          {s.last_poll_at === null ? 'Boîte jamais relevée.' : `Dernière relève : ${WHEN.format(new Date(s.last_poll_at))}.`}
          {s.last_poll_error !== '' && <span className="text-[#a30f2c]">{s.last_poll_error}</span>}
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="ml-auto gap-1.5"
            disabled={poll.isPending || s.poll_pending}
            onClick={() => poll.mutate(undefined, { onError: (e) => toast.error(e instanceof HttpError ? e.message : 'Demande impossible') })}
          >
            <HugeiconsIcon icon={RefreshIcon} size={14} strokeWidth={1.8} className={s.poll_pending ? 'animate-spin' : undefined} />
            {s.poll_pending ? 'Relève en cours…' : 'Relever maintenant'}
          </Button>
        </div>
      )}
      {!settings.reply_enabled && (
        <p className="rounded-[10px] bg-[#fff2ea] px-3 py-2 text-[13px] text-[#b84a0c]">
          Sans adresse de support, les e-mails des tickets n’invitent pas à répondre : le client doit passer par son espace.
        </p>
      )}
    </Card>
  )
}

function SettingsForm({ settings }: { settings: InboundSettings }) {
  const update = useUpdateInboundSettings()
  const initial = valuesOf(settings)
  const [values, setValues] = useState(initial)
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const dirty = JSON.stringify(values) !== JSON.stringify(initial) || password !== ''
  const imapLocked = settings.imap.from_env

  useEffect(() => setErrors({}), [values])

  function set<K extends keyof InboundSettingsValues>(key: K, value: InboundSettingsValues[K]) {
    setValues((v) => ({ ...v, [key]: value }))
  }

  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        update.mutate(
          { ...values, ...(password !== '' ? { imap_password: password } : {}) },
          {
            onSuccess: () => {
              setPassword('')
              toast.success('Réglages enregistrés')
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
      className="flex flex-col gap-3"
    >
      <Card title="Adresse de support" description="Celle que vos clients connaissent. Les réponses aux e-mails des tickets y reviennent sous la forme support+t47.xxxxxxxxxx@…, que votre boîte doit accepter — c’est le cas de Gmail, Google Workspace, Microsoft 365, OVH, Infomaniak et de la plupart des hébergeurs.">
        <Field
          label="Adresse"
          hint={settings.address_from_env ? 'Fixée par INBOUND_ADDRESS dans l’environnement.' : undefined}
          error={errors.address}
        >
          <Input
            type="email"
            className={CHAMP}
            placeholder="support@votre-agence.fr"
            value={values.address}
            disabled={settings.address_from_env}
            onChange={(e) => set('address', e.target.value)}
          />
        </Field>
      </Card>

      <Card
        title="Relève IMAP"
        description="Piilot relève la boîte chaque minute, range chaque nouveau message et le marque comme lu. Les messages déjà lus ne sont pas touchés."
      >
        {imapLocked && <p className="px-1 text-[13px] text-[#73757c]">Fixée par les variables INBOUND_IMAP_* de l’environnement : elle se change là-bas.</p>}
        <label className="flex items-center gap-3 px-1 text-[15px] text-[#1b1b1b]">
          <Checkbox checked={values.imap_enabled} disabled={imapLocked} onCheckedChange={(v) => set('imap_enabled', v === true)} />
          Relever la boîte
        </label>
        <div className="flex flex-col gap-3 sm:flex-row">
          <div className="min-w-0 flex-1">
            <Field label="Serveur" error={errors.imap_host}>
              <Input className={CHAMP} placeholder="imap.votre-hebergeur.fr" value={values.imap_host} disabled={imapLocked} onChange={(e) => set('imap_host', e.target.value)} />
            </Field>
          </div>
          <div className="sm:w-[120px]">
            <Field label="Port" error={errors.imap_port}>
              <Input className={CHAMP} inputMode="numeric" value={String(values.imap_port)} disabled={imapLocked} onChange={(e) => set('imap_port', Number(e.target.value.replace(/\D/g, '')) || 0)} />
            </Field>
          </div>
        </div>
        <Field label="Chiffrement" error={errors.imap_security}>
          <Choices
            value={values.imap_security}
            onChange={(v) => !imapLocked && set('imap_security', v)}
            options={[
              { value: 'tls', label: 'TLS (port 993)' },
              { value: 'none', label: 'Aucun — test local' },
            ]}
          />
        </Field>
        <div className="flex flex-col gap-3 sm:flex-row">
          <div className="min-w-0 flex-1">
            <Field label="Identifiant">
              <Input className={CHAMP} autoComplete="off" value={values.imap_username} disabled={imapLocked} onChange={(e) => set('imap_username', e.target.value)} />
            </Field>
          </div>
          <div className="min-w-0 flex-1">
            <Field
              label="Mot de passe"
              hint={settings.imap.password_set ? 'Renseigné. Laissez vide pour le garder.' : 'Un mot de passe d’application si la boîte a la double authentification.'}
            >
              <Input
                className={CHAMP}
                type="password"
                autoComplete="new-password"
                placeholder={settings.imap.password_set ? '••••••••' : ''}
                value={password}
                disabled={imapLocked}
                onChange={(e) => setPassword(e.target.value)}
              />
            </Field>
          </div>
        </div>
        <Field label="Dossier">
          <Input className={CHAMP} value={values.imap_folder} disabled={imapLocked} onChange={(e) => set('imap_folder', e.target.value)} />
        </Field>
        <SaveBar
          dirty={dirty}
          pending={update.isPending}
          onCancel={() => {
            setValues(initial)
            setPassword('')
          }}
        />
      </Card>
    </form>
  )
}

function WebhookCard({ settings }: { settings: InboundSettings }) {
  const rotate = useRotateInboundSecret()
  const [confirm, setConfirm] = useState(false)

  return (
    <Card
      title="Webhook d’un fournisseur"
      description="Facultatif : pour un rangement en quelques secondes plutôt qu’à la minute. L’adresse porte le secret, gardez-la pour vous."
    >
      {PROVIDERS.map((p) => (
        <Field key={p.key} label={p.label} hint={p.hint}>
          <CopyLine value={settings.webhook.urls[p.key] ?? ''} secret />
        </Field>
      ))}
      {!settings.webhook.from_env && (
        <div className="flex flex-wrap items-center gap-2 px-1">
          {!confirm ? (
            <Button type="button" variant="ghost" size="sm" className="gap-1.5" onClick={() => setConfirm(true)}>
              <HugeiconsIcon icon={RefreshIcon} size={14} strokeWidth={1.8} />
              Changer le secret
            </Button>
          ) : (
            <span className="flex items-center gap-2 text-[13px] text-[#73757c]">
              Les webhooks configurés cesseront d’être acceptés.
              <Button
                type="button"
                size="sm"
                disabled={rotate.isPending}
                onClick={() =>
                  rotate.mutate(undefined, {
                    onSuccess: () => {
                      setConfirm(false)
                      toast.success('Nouveau secret : mettez à jour l’adresse chez votre fournisseur.')
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
      )}
    </Card>
  )
}

function HowItWorks() {
  return (
    <Card title="Ce que Piilot en fait" description="Un e-mail est rangé seul quand il n’y a pas de doute ; sinon, il attend dans « À trier ».">
      <ul className="flex list-disc flex-col gap-1.5 pl-5 text-[14px] text-[#1b1b1b]">
        <li>
          <strong>Réponse à un ticket</strong> : reconnue à l’adresse support+t47.… ou au fil de l’e-mail, elle s’ajoute au ticket avec ses pièces jointes, sans la citation du message précédent. Un ticket clos rouvre.
        </li>
        <li>
          <strong>Nouvelle demande</strong> d’un compte du portail ou d’un contact du CRM : un ticket sur le projet du client s’il n’en a qu’un en cours, avec un accusé de réception. Sinon, à trier.
        </li>
        <li>
          <strong>Expéditeur inconnu</strong>, réponse d’une autre adresse, transfert d’un collègue : à trier, jamais rejeté en silence.
        </li>
        <li>
          <strong>Écartés d’office</strong> : réponses automatiques, rebonds, listes de diffusion, e-mails de Piilot qui reviennent.
        </li>
      </ul>
      <p className="flex items-center gap-1.5 px-1 text-[13px] text-[#73757c]">
        <HugeiconsIcon icon={MailReceive01Icon} size={14} strokeWidth={1.8} />
        Un collègue qui répond à l’e-mail d’un ticket écrit une note interne : rien ne part au client sans passer par Piilot.
      </p>
    </Card>
  )
}
