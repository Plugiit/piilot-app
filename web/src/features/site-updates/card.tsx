import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { CHAMP, Card, Field, SaveBar } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { siteUpdatesQuery, useSiteUpdatesActions, type UpdateEnvironment } from '@/features/site-updates/api'
import { fieldErrors, HttpError } from '@/lib/api'
import { useConfirmFocus } from '@/lib/use-confirm-focus'
import type { SiteUpdates } from '@/types/api'

/**
 * Sur quel environnement les mises a jour d'un site se font, et leurs
 * lancements. C'est la CI du depot qui travaille : Piilot lui demande de partir
 * et attend son compte rendu. Aucun environnement n'est choisi d'office.
 */
const ENVIRONMENT: Record<UpdateEnvironment, string> = { preprod: 'Préproduction', prod: 'Production' }

const RUN_STATUS: Record<SiteUpdates['runs'][number]['status'], { label: string; warn: boolean }> = {
  queued: { label: 'En attente', warn: false },
  dispatching: { label: 'Demandé à GitHub', warn: false },
  dispatched: { label: 'En cours', warn: false },
  success: { label: 'Réussie', warn: false },
  failed: { label: 'Échec', warn: true },
  timed_out: { label: 'Sans compte rendu', warn: true },
}

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

function fail(fallback: string) {
  return (e: unknown) => toast.error(e instanceof HttpError ? e.message : fallback)
}

export function SiteUpdatesCard({ projectId, canManage }: { projectId: string; canManage: boolean }) {
  const { data, isError } = useQuery(siteUpdatesQuery(projectId))
  if (isError) return <p className="px-1 text-[14px] text-[#9a6a00]">Mises à jour indisponibles pour le moment.</p>
  if (data === undefined) return <div className="h-[240px] animate-pulse rounded-[12px] bg-[#f3f4f4] motion-reduce:animate-none" />
  return (
    <Card
      title="Mises à jour du site"
      description="Choisissez l’environnement à mettre à jour. Piilot demande au dépôt de lancer son workflow et attend son compte rendu. Sans réponse sous deux heures, ou après un échec, les admins sont prévenus et la planification s’arrête jusqu’à une relance."
    >
      {!data.repo_supported && (
        <p className="px-1 text-[14px] text-[#9a6a00]">
          {data.repo === '' ? 'Ce projet n’a pas de dépôt : renseignez-le dans ses paramètres.' : 'Seuls les dépôts GitHub sont pris en charge pour le moment.'}
        </p>
      )}
      {data.repo_supported && !data.token_configured && (
        <p className="px-1 text-[14px] text-[#9a6a00]">
          Aucun jeton GitHub.{' '}
          <Link to="/parametres/git" className="underline">
            Le renseigner
          </Link>
        </p>
      )}
      {data.repo_changed && <p className="px-1 text-[14px] text-[#9a6a00]">Le dépôt du projet a changé ({data.repo === '' ? 'aucun' : data.repo}). Rien ne part tant que vous n’avez pas enregistré à nouveau ces réglages.</p>}
      <TargetForm key={JSON.stringify(data.target)} projectId={projectId} view={data} canManage={canManage} />
      {data.runs.length > 0 && (
        <ul className="flex flex-col">
          {data.runs.map((r) => (
            <li key={r.id} className="flex flex-wrap items-baseline gap-x-3 border-b border-[#f3f4f4] py-2 text-[13px] last:border-b-0">
              <span className="text-[#73757c]">{WHEN.format(new Date(r.requested_at))}</span>
              <span className="text-[#1b1b1b]">{ENVIRONMENT[r.environment]}</span>
              <span className={RUN_STATUS[r.status].warn ? 'text-[#9a6a00]' : r.status === 'success' ? 'text-[#006f1f]' : 'text-[#73757c]'}>{RUN_STATUS[r.status].label}</span>
              {r.trigger === 'schedule' && <span className="text-[#73757c]">planifiée</span>}
              {r.message !== '' && <span className="min-w-0 flex-1 break-words text-[#73757c]">{r.message}</span>}
              {r.run_url !== '' && (
                <a href={r.run_url} target="_blank" rel="noreferrer" className="underline text-[#73757c] hover:text-[#1b1b1b]">
                  Voir le workflow
                </a>
              )}
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}

function TargetForm({ projectId, view, canManage }: { projectId: string; view: SiteUpdates; canManage: boolean }) {
  const act = useSiteUpdatesActions(projectId)
  const target = view.target
  const initial = {
    environment: target?.environment ?? '',
    workflow: target?.workflow ?? 'piilot-update.yml',
    ref: target?.ref ?? 'main',
    every: target?.every_days != null ? String(target.every_days) : '',
    enabled: target?.enabled ?? true,
  }
  const [values, setValues] = useState(initial)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const changed = JSON.stringify(values) !== JSON.stringify(initial)
  const dirty = changed || view.repo_changed
  const set = <K extends keyof typeof values>(k: K, v: (typeof values)[K]) => setValues((s) => ({ ...s, [k]: v }))
  const scheduledOnProd = values.environment === 'prod' && values.every !== ''

  return (
    <form
      noValidate
      className="flex flex-col gap-3"
      onSubmit={(e) => {
        e.preventDefault()
        if (values.environment !== 'prod' && values.environment !== 'preprod') {
          setErrors({ environment: 'Choisissez un environnement' })
          return
        }
        setErrors({})
        act.setTarget.mutate(
          { environment: values.environment, workflow: values.workflow.trim(), ref: values.ref.trim(), every_days: values.every === '' ? null : Number(values.every), enabled: values.enabled },
          {
            onSuccess: () => toast.success('Réglages enregistrés'),
            onError: (error) => {
              const fields = fieldErrors(error)
              setErrors(fields)
              if (Object.keys(fields).length === 0) fail('Enregistrement impossible')(error)
            },
          },
        )
      }}
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-[220px_minmax(0,1fr)_160px_160px]">
        <Field label="Environnement" {...(errors.environment !== undefined ? { error: errors.environment } : {})}>
          <Select value={values.environment} disabled={!canManage} onValueChange={(v) => set('environment', v as UpdateEnvironment)}>
            <SelectTrigger className="w-full" aria-label="Environnement">
              <SelectValue placeholder="À choisir" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="preprod">{ENVIRONMENT.preprod}</SelectItem>
              <SelectItem value="prod">{ENVIRONMENT.prod}</SelectItem>
            </SelectContent>
          </Select>
        </Field>
        <Field label="Workflow" hint="Fichier de .github/workflows." {...(errors.workflow !== undefined ? { error: errors.workflow } : {})}>
          <Input className={CHAMP} disabled={!canManage} value={values.workflow} onChange={(e) => set('workflow', e.target.value)} />
        </Field>
        <Field label="Branche" {...(errors.ref !== undefined ? { error: errors.ref } : {})}>
          <Input className={CHAMP} disabled={!canManage} value={values.ref} onChange={(e) => set('ref', e.target.value)} />
        </Field>
        <Field label="Tous les (jours)" hint="Vide : à la demande." {...(errors.every_days !== undefined ? { error: errors.every_days } : {})}>
          <Input className={CHAMP} type="number" min={1} max={90} disabled={!canManage} value={values.every} onChange={(e) => set('every', e.target.value)} />
        </Field>
      </div>
      {scheduledOnProd && <p className="px-1 text-[13px] text-[#9a6a00]">Attention : la production sera mise à jour sans geste de votre part, à chaque échéance. La première vient après ce délai, à compter de l’enregistrement.</p>}
      <label className="flex items-center gap-3 px-1 text-[14px] text-[#1b1b1b]">
        <Checkbox checked={values.enabled} disabled={!canManage} onCheckedChange={(v) => set('enabled', v === true)} />
        Mises à jour actives sur ce projet
      </label>
      {canManage && <SaveBar dirty={dirty} pending={act.setTarget.isPending} onCancel={() => { setValues(initial); setErrors({}) }} />}
      {canManage && target !== null && target.enabled && <RunButton projectId={projectId} view={view} environment={target.environment} unsaved={dirty} />}
    </form>
  )
}

function RunButton({ projectId, view, environment, unsaved }: { projectId: string; view: SiteUpdates; environment: UpdateEnvironment; unsaved: boolean }) {
  const act = useSiteUpdatesActions(projectId)
  const [confirming, setConfirming] = useState(false)
  const { panelRef, actionsRef } = useConfirmFocus(confirming)
  const active = view.runs.some((r) => r.status === 'queued' || r.status === 'dispatching' || r.status === 'dispatched')
  const disabled = active || unsaved || !view.repo_supported || view.repo_changed || !view.token_configured || act.run.isPending

  function launch() {
    act.run.mutate(environment, {
      onSuccess: () => {
        setConfirming(false)
        toast.success('Mise à jour demandée')
      },
      onError: fail('Lancement impossible'),
    })
  }

  if (confirming) {
    return (
      <div
        ref={panelRef}
        tabIndex={-1}
        role="alertdialog"
        aria-label="Confirmer la mise à jour de la production"
        className="flex flex-col gap-2 rounded-[8px] border border-[#ffe8b7] bg-[#fffaf0] px-3 py-2 text-[13px] text-[#9a6a00] outline-none sm:flex-row sm:items-center"
      >
        <span className="min-w-0 sm:flex-1">Mettre à jour la production maintenant ? Le site du client est concerné.</span>
        <div className="flex flex-wrap items-center justify-end gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={() => setConfirming(false)}>
            Annuler
          </Button>
          <Button type="button" variant="destructive" size="sm" disabled={disabled} onClick={launch}>
            Lancer sur la production
          </Button>
        </div>
      </div>
    )
  }
  return (
    <div ref={actionsRef} className="flex flex-wrap items-center gap-3">
      <Button type="button" variant="outline" disabled={disabled} onClick={() => (environment === 'prod' ? setConfirming(true) : launch())}>
        Lancer sur {ENVIRONMENT[environment].toLowerCase()}
      </Button>
      {active && <span className="text-[13px] text-[#73757c]">Une mise à jour est en cours.</span>}
      {!active && unsaved && <span className="text-[13px] text-[#73757c]">Enregistrez d’abord vos modifications.</span>}
    </div>
  )
}
