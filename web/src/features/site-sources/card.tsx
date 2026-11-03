import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'

import { CHAMP, Card, Field, SaveBar } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { siteSourcesQuery, useSiteSourcesActions } from '@/features/site-sources/api'
import { fieldErrors, HttpError } from '@/lib/api'
import { useConfirmFocus } from '@/lib/use-confirm-focus'
import type { SiteSources } from '@/types/api'

/**
 * Ce que Piilot surveille du site sans y toucher : ses sauvegardes, qui se
 * signalent elles-memes, et ses domaines. Le silence alerte autant qu'un echec.
 */
const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit' })
const DAY = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })

function fail(fallback: string) {
  return (e: unknown) => toast.error(e instanceof HttpError ? e.message : fallback)
}

export function SiteSourcesCard({ projectId, canEdit, canManageToken }: { projectId: string; canEdit: boolean; canManageToken: boolean }) {
  const { data, isError } = useQuery(siteSourcesQuery(projectId))
  if (isError) return <p className="px-1 text-[14px] text-[#9a6a00]">Sauvegardes et domaines indisponibles pour le moment.</p>
  if (data === undefined) return <div className="h-[240px] animate-pulse rounded-[12px] bg-[#f3f4f4] motion-reduce:animate-none" />
  return (
    <>
      <BackupCard projectId={projectId} view={data} canManageToken={canManageToken} />
      <DomainsCard projectId={projectId} view={data} canEdit={canEdit} />
    </>
  )
}

function BackupCard({ projectId, view, canManageToken }: { projectId: string; view: SiteSources; canManageToken: boolean }) {
  const act = useSiteSourcesActions(projectId)
  const [token, setToken] = useState<string | null>(null)
  const [confirming, setConfirming] = useState<'rotate' | 'remove' | null>(null)
  const { panelRef, actionsRef } = useConfirmFocus(confirming !== null)
  const backup = view.backup

  function rotate() {
    act.rotate.mutate(undefined, {
      onSuccess: (r) => {
        setToken(r.token)
        setConfirming(null)
      },
      onError: fail('Création du jeton impossible'),
    })
  }

  return (
    <Card title="Sauvegardes du site" description="Le script de sauvegarde du site appelle Piilot à la fin de chaque exécution. Si l’appel n’arrive pas dans le délai attendu, ou signale un échec, les admins sont prévenus.">
      {backup === null ? (
        <p className="px-1 text-[14px] text-[#73757c]">Les sauvegardes de ce projet ne sont pas surveillées.</p>
      ) : (
        <>
          <p className={`px-1 text-[14px] ${backup.healthy ? 'text-[#1b1b1b]' : 'text-[#9a6a00]'}`}>
            {backup.last_success_at === null
              ? 'Aucune sauvegarde réussie signalée pour le moment.'
              : `Dernière sauvegarde réussie le ${WHEN.format(new Date(backup.last_success_at))}.`}
            {!backup.healthy && backup.last_success_at !== null && ' En défaut : elle date trop, ou une exécution plus récente a échoué.'}
          </p>
          <BackupForm key={`${backup.expected_hours}-${backup.last_restore_test_on ?? ''}`} projectId={projectId} expectedHours={backup.expected_hours} restoreTestOn={backup.last_restore_test_on} canEdit={canManageToken} />
          {backup.runs.length > 0 && (
            <ul className="flex flex-col">
              {backup.runs.map((r) => (
                <li key={r.finished_at + r.status} className="flex flex-wrap items-baseline gap-x-3 border-b border-[#f3f4f4] py-2 text-[13px] last:border-b-0">
                  <span className="text-[#73757c]">{WHEN.format(new Date(r.finished_at))}</span>
                  <span className={r.status === 'success' ? 'text-[#006f1f]' : 'text-[#9a6a00]'}>{r.status === 'success' ? 'Réussie' : 'Échec'}</span>
                  {r.message !== '' && <span className="min-w-0 flex-1 break-words text-[#73757c]">{r.message}</span>}
                </li>
              ))}
            </ul>
          )}
        </>
      )}

      {token !== null && (
        <div className="flex flex-col gap-2 rounded-[8px] border border-[#ffe8b7] bg-[#fffaf0] p-3 text-[13px] text-[#9a6a00]">
          <p className="font-medium">Copiez ce jeton maintenant : il ne s’affichera plus.</p>
          <div className="overflow-x-auto rounded-[6px] bg-white p-2">
            <pre className="text-[12px] text-[#1b1b1b]">{`curl -fsS -X POST ${view.hook_url} \\
  -H "Authorization: Bearer ${token}" \\
  -H "Content-Type: application/json" \\
  -d '{"status":"success"}'`}</pre>
          </div>
          <p>Le script envoie « success » ou « failed ». Rangez le jeton dans les secrets du site, jamais dans son dépôt.</p>
        </div>
      )}

      {canManageToken && confirming === null && (
        <div ref={actionsRef} className="flex flex-wrap gap-2">
          {backup === null ? (
            <Button type="button" variant="outline" disabled={act.rotate.isPending} onClick={rotate}>
              Surveiller les sauvegardes
            </Button>
          ) : (
            <>
              <Button type="button" variant="outline" onClick={() => setConfirming('rotate')}>
                Changer le jeton
              </Button>
              <Button type="button" variant="ghost" onClick={() => setConfirming('remove')}>
                Cesser la surveillance
              </Button>
            </>
          )}
        </div>
      )}
      {confirming !== null && (
        <div
          ref={panelRef}
          tabIndex={-1}
          role="alertdialog"
          aria-label={confirming === 'rotate' ? 'Confirmer le changement de jeton' : 'Confirmer la fin de la surveillance'}
          className="flex flex-col gap-2 text-[13px] text-[#4b4b4f] outline-none sm:flex-row sm:items-center"
        >
          <span className="min-w-0 sm:flex-1">
            {confirming === 'rotate' ? 'L’ancien jeton cessera aussitôt de fonctionner : le script du site devra recevoir le nouveau.' : 'Plus aucune alerte ne partira pour ce projet. L’historique reste.'}
          </span>
          <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={() => setConfirming(null)}>
            Annuler
          </Button>
          <Button
            type="button"
            variant="destructive"
            size="sm"
            disabled={act.rotate.isPending || act.remove.isPending}
            onClick={() => {
              if (confirming === 'rotate') {
                rotate()
                return
              }
              act.remove.mutate(undefined, {
                onSuccess: () => {
                  setToken(null)
                  setConfirming(null)
                },
                onError: fail('Retrait impossible'),
              })
            }}
          >
            Confirmer
          </Button>
          </div>
        </div>
      )}
    </Card>
  )
}

function BackupForm({ projectId, expectedHours, restoreTestOn, canEdit }: { projectId: string; expectedHours: number; restoreTestOn: string | null; canEdit: boolean }) {
  const act = useSiteSourcesActions(projectId)
  const [hours, setHours] = useState(String(expectedHours))
  const [restore, setRestore] = useState(restoreTestOn ?? '')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const dirty = hours !== String(expectedHours) || restore !== (restoreTestOn ?? '')
  return (
    <form
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        setErrors({})
        act.update.mutate(
          { expected_hours: Number(hours), last_restore_test_on: restore === '' ? null : restore },
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
      className="flex flex-col gap-3"
    >
      <div className="flex flex-col gap-3 sm:flex-row">
        <div className="sm:w-[220px]">
          <Field label="Délai attendu (heures)" hint="26 pour une sauvegarde quotidienne." {...(errors.expected_hours !== undefined ? { error: errors.expected_hours } : {})}>
            <Input className={CHAMP} type="number" min={1} max={744} disabled={!canEdit} value={hours} onChange={(e) => setHours(e.target.value)} />
          </Field>
        </div>
        <div className="sm:w-[220px]">
          <Field label="Dernier test de restauration" hint="Une sauvegarde jamais restaurée n’est pas une preuve." {...(errors.last_restore_test_on !== undefined ? { error: errors.last_restore_test_on } : {})}>
            <Input className={CHAMP} type="date" disabled={!canEdit} value={restore} onChange={(e) => setRestore(e.target.value)} />
          </Field>
        </div>
      </div>
      {canEdit && <SaveBar dirty={dirty} pending={act.update.isPending} onCancel={() => { setHours(String(expectedHours)); setRestore(restoreTestOn ?? ''); setErrors({}) }} />}
    </form>
  )
}

function DomainsCard({ projectId, view, canEdit }: { projectId: string; view: SiteSources; canEdit: boolean }) {
  const act = useSiteSourcesActions(projectId)
  const [draft, setDraft] = useState('')
  const [error, setError] = useState<string | undefined>(undefined)
  function add() {
    setError(undefined)
    act.addDomain.mutate(draft, {
      onSuccess: () => setDraft(''),
      onError: (e) => {
        const message = fieldErrors(e).domain
        if (message !== undefined) setError(message)
        else fail('Ajout impossible')(e)
      },
    })
  }
  return (
    <Card title="Domaines du site" description="L’échéance est relevée chaque semaine. Les admins sont prévenus à 60, 30, 14, 7 et 1 jours, et si le relevé ne réussit plus.">
      {view.domains.length === 0 ? (
        <p className="px-1 text-[14px] text-[#73757c]">Aucun domaine suivi.</p>
      ) : (
        <ul className="flex flex-col">
          {view.domains.map((d) => (
            <li key={d.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-[#f3f4f4] py-2 last:border-b-0">
              <span className="min-w-0 basis-full break-words text-[14px] text-[#1b1b1b] sm:flex-1 sm:basis-0">{d.domain}</span>
              <span className={`text-[13px] ${d.failing || (d.days_left !== null && d.days_left < 30) ? 'text-[#9a6a00]' : 'text-[#73757c]'}`}>
                {d.failing
                  ? 'Relevé en échec'
                  : d.expires_at !== null
                    ? `Échéance le ${DAY.format(new Date(d.expires_at))}`
                    : 'Échéance relevée sous peu'}
              </span>
              {canEdit && (
                <Button type="button" variant="ghost" size="sm" aria-label={`Retirer ${d.domain}`} disabled={act.removeDomain.isPending} onClick={() => act.removeDomain.mutate(d.id, { onError: fail('Retrait impossible') })}>
                  Retirer
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
      {canEdit && (
        <form
          noValidate
          className="flex items-start gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            if (draft.trim() !== '') add()
          }}
        >
          <div className="min-w-0 flex-1">
            <Field label="Ajouter un domaine" {...(error !== undefined ? { error } : {})}>
              <Input className={CHAMP} placeholder="exemple.fr" value={draft} onChange={(e) => setDraft(e.target.value)} />
            </Field>
          </div>
          <Button type="submit" variant="outline" className="mt-[30px]" disabled={draft.trim() === '' || act.addDomain.isPending}>
            Ajouter
          </Button>
        </form>
      )}
    </Card>
  )
}
