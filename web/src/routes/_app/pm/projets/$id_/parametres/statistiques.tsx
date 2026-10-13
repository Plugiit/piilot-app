import { Cancel01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { SearchPicker } from '@/components/search-picker'
import { Card, SaveBar } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { analyticsWebsitesQuery, projectAnalyticsQuery, useSetProjectWebsites } from '@/features/analytics/api'
import { HttpError } from '@/lib/api'
import { can, sessionQuery } from '@/lib/auth'
import type { AnalyticsWebsite, ProjectAnalytics } from '@/types/api'

/**
 * Les sites Umami d'un projet. Facultatif. Le site de production est celui
 * que voit le client, sur la page « Statistiques » de son projet : un seul.
 */
export const Route = createFileRoute('/_app/pm/projets/$id_/parametres/statistiques')({
  loader: ({ context, params }) => context.queryClient.ensureQueryData(projectAnalyticsQuery(params.id)),
  component: StatisticsTab,
})

const ENVIRONMENT = { prod: 'Production', preprod: 'Préproduction' } as const
type Environment = keyof typeof ENVIRONMENT

interface Choice {
  website_id: string
  environment: Environment
  name: string
  domain: string
}

function choicesOf(view: ProjectAnalytics): Choice[] {
  return view.sites.map((s) => ({ website_id: s.website_id, environment: s.environment, name: s.name, domain: s.domain }))
}

function StatisticsTab() {
  const { id } = Route.useParams()
  const { data } = useQuery(projectAnalyticsQuery(id))
  const { data: session } = useQuery(sessionQuery)
  if (data === undefined) return null

  if (!data.module_enabled && data.sites.length === 0) {
    return (
      <div className="flex flex-col gap-3 p-4">
        <Card title="Statistiques" description="Reliez le projet à son site dans Umami : le client verra son audience depuis son espace.">
          <p className="px-1 text-[14px] text-[#73757c]">
            Le module de statistiques n’est pas activé.
            {can(session, 'users.write') && (
              <>
                {' '}
                <Link to="/parametres/modules/umami" className="text-[#1b1b1b] underline">
                  L’activer dans les paramètres
                </Link>
                .
              </>
            )}
          </p>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-3 p-4">
      <WebsitesForm key={data.sites.map((s) => s.website_id + s.environment).join()} projectId={id} view={data} canEdit={can(session, 'projects.write')} />
    </div>
  )
}

/** La recherche des sites, vingt resultats au plus. */
function useWebsiteSearch(text: string, enabled: boolean) {
  const { data, isFetching } = useQuery({ ...analyticsWebsitesQuery(text, 1, 20), enabled })
  return { items: data?.items ?? [], fetching: isFetching }
}

function WebsitesForm({ projectId, view, canEdit }: { projectId: string; view: ProjectAnalytics; canEdit: boolean }) {
  const save = useSetProjectWebsites(projectId)
  const initial = choicesOf(view)
  const [choices, setChoices] = useState(initial)
  const signature = (list: Choice[]) => JSON.stringify(list.map((c) => [c.website_id, c.environment]))
  const dirty = signature(choices) !== signature(initial)
  const hasProd = choices.some((c) => c.environment === 'prod')

  function add(w: AnalyticsWebsite) {
    // Le premier site est la production ; les suivants, la preproduction.
    setChoices((list) => [...list, { website_id: w.id, environment: list.some((c) => c.environment === 'prod') ? 'preprod' : 'prod', name: w.name, domain: w.domain }])
  }

  // Une seule production : en choisir une rend la precedente a la
  // preproduction.
  function setEnvironment(id: string, env: Environment) {
    setChoices((list) =>
      list.map((c) => (c.website_id === id ? { ...c, environment: env } : env === 'prod' && c.environment === 'prod' ? { ...c, environment: 'preprod' } : c)),
    )
  }

  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        save.mutate(
          choices.map((c) => ({ website_id: c.website_id, environment: c.environment })),
          {
            onSuccess: () => toast.success('Sites enregistrés'),
            onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
          },
        )
      }}
    >
      <Card
        title="Sites du projet"
        description="Choisissez parmi les sites qu’Umami mesure. Le site de production est celui que le client voit dans son espace ; la préproduction reste interne."
      >
        {canEdit && choices.length < 10 && (
          <SearchPicker
            placeholder="Rechercher un site à lier"
            useItems={useWebsiteSearch}
            keyOf={(w) => w.id}
            filter={(w) => w.present && !choices.some((c) => c.website_id === w.id)}
            onPick={add}
            render={(w) => (
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="truncate text-[14px]">{w.name}</span>
                <span className="truncate text-[12px] text-[#8d8d8d]">{w.domain}</span>
              </span>
            )}
          />
        )}
        {choices.length === 0 ? (
          <p className="px-1 text-[14px] text-[#73757c]">Aucun site lié pour l’instant.</p>
        ) : (
          <ul className="flex flex-col">
            {choices.map((c) => (
              <li key={c.website_id} className="flex flex-wrap items-center gap-3 border-b border-[#f3f4f4] py-2 last:border-b-0">
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="truncate text-[14px] text-[#1b1b1b]">{c.name}</span>
                  <span className="truncate text-[12px] text-[#8d8d8d]">
                    {c.domain}
                    {c.environment === 'prod' && ' · visible par le client'}
                  </span>
                </span>
                <Select value={c.environment} disabled={!canEdit} onValueChange={(v: Environment) => setEnvironment(c.website_id, v)}>
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
                    onClick={() => setChoices((list) => list.filter((x) => x.website_id !== c.website_id))}
                  >
                    <HugeiconsIcon icon={Cancel01Icon} size={16} strokeWidth={1.8} />
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
        {choices.length > 0 && !hasProd && (
          <p className="px-1 text-[13px] text-[#9a6a00]">Sans site de production, le client ne voit pas de statistiques.</p>
        )}
        {canEdit && <SaveBar dirty={dirty} pending={save.isPending} onCancel={() => setChoices(initial)} />}
      </Card>
    </form>
  )
}
