import { Cancel01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { Card, SaveBar } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { coolifyApplicationsQuery, projectCoolifyQuery, useSetProjectCoolify } from '@/features/coolify/api'
import { HttpError } from '@/lib/api'
import type { ProjectCoolify } from '@/types/api'

const ENVIRONMENT = { prod: 'Production', preprod: 'Préproduction' } as const
type Environment = keyof typeof ENVIRONMENT

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit' })

/** Une application choisie, avec ce qu'il faut pour l'afficher avant l'enregistrement. */
interface Choice {
  application_uuid: string
  environment: Environment
  name: string
  fqdn: string
}

function choicesOf(view: ProjectCoolify): Choice[] {
  return view.applications.map((a) => ({ application_uuid: a.application_uuid, environment: a.environment, name: a.name, fqdn: a.fqdn }))
}

/**
 * Les mises en ligne du projet, lues chez Coolify : l'application de
 * production et celle de preproduction, et les cinq derniers deploiements
 * releves. Rien ici n'appelle Coolify : la tache de fond relit, l'ecran lit
 * ce qu'elle a ecrit.
 */
export function CoolifyCard({ projectId, canEdit, canConfigure }: { projectId: string; canEdit: boolean; canConfigure: boolean }) {
  const { data, isError } = useQuery(projectCoolifyQuery(projectId))
  if (isError) return <p className="px-1 text-[14px] text-[#9a6a00]">Déploiements Coolify indisponibles pour le moment.</p>
  if (data === undefined) return <div className="h-[160px] animate-pulse rounded-[12px] bg-[#f3f4f4] motion-reduce:animate-none" />

  if (!data.module_enabled && data.applications.length === 0) {
    return (
      <Card title="Déploiements" description="Reliez le projet à ses applications Coolify : les mises en ligne du mois alimentent le rapport.">
        <p className="px-1 text-[14px] text-[#73757c]">
          Le module Coolify n’est pas activé.
          {canConfigure && (
            <>
              {' '}
              <Link to="/parametres/modules/coolify" className="text-[#1b1b1b] underline">
                L’activer dans les paramètres
              </Link>
              .
            </>
          )}
        </p>
      </Card>
    )
  }

  return <ApplicationsForm key={data.applications.map((a) => a.application_uuid + a.environment).join()} projectId={projectId} view={data} canEdit={canEdit} />
}

function ApplicationsForm({ projectId, view, canEdit }: { projectId: string; view: ProjectCoolify; canEdit: boolean }) {
  const save = useSetProjectCoolify(projectId)
  const initial = choicesOf(view)
  const [choices, setChoices] = useState(initial)
  const dirty = JSON.stringify(choices.map((c) => [c.application_uuid, c.environment])) !== JSON.stringify(initial.map((c) => [c.application_uuid, c.environment]))

  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        save.mutate(
          choices.map((c) => ({ application_uuid: c.application_uuid, environment: c.environment })),
          {
            onSuccess: () => toast.success('Applications enregistrées : relève demandée'),
            onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
          },
        )
      }}
    >
      <Card
        title="Déploiements"
        description="Les applications Coolify de ce projet. Seuls les déploiements réussis sont relevés ; avec elles, le rapport s’appuie sur Coolify plutôt que sur le webhook Git. Facultatif, jusqu’à dix applications."
      >
        {canEdit && choices.length < 10 && (
          <ApplicationPicker
            exclude={choices.map((c) => c.application_uuid)}
            onPick={(a) =>
              setChoices((list) => [
                ...list,
                // La premiere application est la production ; les suivantes, la preproduction.
                { application_uuid: a.uuid, environment: list.some((c) => c.environment === 'prod') ? 'preprod' : 'prod', name: a.name, fqdn: a.fqdn },
              ])
            }
          />
        )}
        {choices.length === 0 ? (
          <p className="px-1 text-[14px] text-[#73757c]">Aucune application liée pour l’instant.</p>
        ) : (
          <ul className="flex flex-col">
            {choices.map((c) => (
              <li key={c.application_uuid} className="flex flex-wrap items-center gap-3 border-b border-[#f3f4f4] py-2 last:border-b-0">
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="truncate text-[14px] text-[#1b1b1b]">{c.name}</span>
                  {c.fqdn !== '' && <span className="truncate text-[12px] text-[#8d8d8d]">{c.fqdn}</span>}
                </span>
                {view.applications.some((a) => a.application_uuid === c.application_uuid && !a.present) && (
                  <span className="text-[12px] text-[#a2a3a7]">retirée de Coolify</span>
                )}
                <Select
                  value={c.environment}
                  disabled={!canEdit}
                  onValueChange={(v: Environment) => setChoices((list) => list.map((x) => (x.application_uuid === c.application_uuid ? { ...x, environment: v } : x)))}
                >
                  <SelectTrigger size="sm" className="h-8 w-[150px] text-[13px]" aria-label={`Environnement de ${c.name}`}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent align="end">
                    <SelectItem value="prod">{ENVIRONMENT.prod}</SelectItem>
                    <SelectItem value="preprod">{ENVIRONMENT.preprod}</SelectItem>
                  </SelectContent>
                </Select>
                {canEdit && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Retirer ${c.name}`}
                    onClick={() => setChoices((list) => list.filter((x) => x.application_uuid !== c.application_uuid))}
                  >
                    <HugeiconsIcon icon={Cancel01Icon} size={16} strokeWidth={1.8} />
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
        {canEdit && <SaveBar dirty={dirty} pending={save.isPending} onCancel={() => setChoices(initial)} />}

        <Recent view={view} />
      </Card>
    </form>
  )
}

function Recent({ view }: { view: ProjectCoolify }) {
  if (view.applications.length === 0) return null
  return (
    <div className="flex flex-col gap-1">
      {view.last_poll_error !== null && <p className="px-1 text-[13px] text-[#9a6a00]">Dernière relève en erreur : {view.last_poll_error}</p>}
      {view.deployments.length === 0 ? (
        <p className="px-1 text-[13px] text-[#73757c]">
          {view.last_poll_at === null ? 'Pas encore relevé.' : 'Aucun déploiement réussi relevé pour l’instant.'}
        </p>
      ) : (
        <>
          <p className="px-1 text-[13px] font-medium text-[#1b1b1b]">Derniers déploiements</p>
          <ul className="flex flex-col">
            {view.deployments.map((d) => (
              <li key={d.deployed_at + d.name} className="flex flex-wrap items-baseline gap-x-3 border-b border-[#f3f4f4] py-2 text-[13px] last:border-b-0">
                <span className="text-[#73757c] tabular-nums">{WHEN.format(new Date(d.deployed_at))}</span>
                <span className="min-w-0 flex-1 truncate text-[#1b1b1b]">{d.name}</span>
                <span className="text-[#73757c]">{d.environment}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  )
}

/** Le choix d'une application parmi celles que la releve a vues. */
function ApplicationPicker({ exclude, onPick }: { exclude: string[]; onPick: (a: { uuid: string; name: string; fqdn: string }) => void }) {
  const { data, isError } = useQuery(coolifyApplicationsQuery)
  const options = (data?.items ?? []).filter((a) => a.present && !exclude.includes(a.uuid))

  if (isError) return <p className="px-1 text-[13px] text-[#9a6a00]">Liste des applications indisponible.</p>
  if (data !== undefined && options.length === 0) {
    return (
      <p className="px-1 text-[13px] text-[#73757c]">
        {data.items.length === 0 ? 'Aucune application relevée : testez la connexion dans les paramètres du module.' : 'Toutes les applications relevées sont déjà liées.'}
      </p>
    )
  }
  return (
    // La valeur reste vide : le choix s'ajoute a la liste, puis le menu se referme.
    <Select
      value=""
      onValueChange={(uuid) => {
        const a = options.find((o) => o.uuid === uuid)
        if (a !== undefined) onPick(a)
      }}
    >
      <SelectTrigger className="w-full" aria-label="Application à lier">
        <SelectValue placeholder="Choisir une application à lier" />
      </SelectTrigger>
      <SelectContent>
        {options.map((a) => (
          <SelectItem key={a.uuid} value={a.uuid}>
            {a.project_name !== '' ? `${a.project_name} · ` : ''}
            {a.name}
            {a.environment_name !== '' ? ` (${a.environment_name})` : ''}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
