import { useQuery } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { z } from 'zod'

import { PageFrame, type Crumb } from '@/components/layout/page-frame'
import { Card } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { importQuery, importRowsQuery, useRunImport } from '@/features/data-import/api'
import {
  entityLabel,
  failureLabel,
  IMPORT_ORDER,
  isActive,
  STEP_LABEL,
  STEPS,
  type RowOutcome,
  type Step,
} from '@/features/data-import/entities'
import { cleanMapping, missingRequired, type Mapping } from '@/features/data-import/mapping'
import { MappingStep } from '@/features/data-import/mapping-step'
import { Report } from '@/features/data-import/report'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { ImportDetail } from '@/types/api'

/**
 * Assistant d'import en cinq étapes : aperçu, correspondance des colonnes,
 * simulation, import, rapport. L'étape et les filtres du rapport vivent dans
 * l'URL : on peut partager ou recharger sans perdre sa place.
 *
 * Le serveur borne ce qui est possible (une simulation terminée peut être
 * relancée ou transformée en import, un import réel est définitif) ; l'écran
 * en déduit l'étape réellement affichée, voir `effectiveStep`.
 */
export const Route = createFileRoute('/_app/parametres/import/$id')({
  validateSearch: z.object({
    step: z.enum(STEPS).catch('apercu'),
    outcome: z.enum(['created', 'skipped', 'error']).optional().catch(undefined),
    page: z.number().int().min(1).catch(1),
  }),
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'data.import')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  loader: ({ context, params }) => context.queryClient.query({ ...importQuery(params.id), staleTime: 0 }),
  component: ImportWizard,
})

const TRAIL: Crumb = { label: 'Import et export', to: '/parametres/import', search: { page: 1 } }

/**
 * Étape réellement montrée. Un import en cours n'a qu'un écran : la progression
 * de la simulation ou de l'import. Un import réel terminé ne mène qu'à son
 * rapport. Un import échoué ou jamais lancé n'a pas de rapport à lire.
 */
function effectiveStep(item: ImportDetail, step: Step): Step {
  if (isActive(item.status)) return item.dry_run ? 'simulation' : 'import'
  if (item.status === 'done' && !item.dry_run) return 'rapport'
  if (item.status === 'uploaded' && (step === 'import' || step === 'rapport')) return 'simulation'

  return step
}

function ImportWizard() {
  const { id } = Route.useParams()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const { data: item, isError, error } = useQuery(importQuery(id))
  const [mapping, setMapping] = useState<Mapping | undefined>()

  const step = item === undefined ? search.step : effectiveStep(item, search.step)

  // L'URL suit l'étape réellement affichée (fin d'import, lien périmé).
  useEffect(() => {
    if (step !== search.step) void navigate({ search: (prev) => ({ ...prev, step, page: 1 }), replace: true })
  }, [step, search.step, navigate])

  if (isError || item === undefined) {
    return (
      <PageFrame title="Import" trail={[TRAIL]}>
        <p role="alert" className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
          {error instanceof HttpError ? error.message : 'Chargement impossible'}
        </p>
      </PageFrame>
    )
  }

  const current: Mapping = mapping ?? (Object.keys(item.mapping).length > 0 && item.status !== 'uploaded' ? item.mapping : item.suggested_mapping)
  const reviewable = item.status === 'uploaded' || (item.status === 'done' && item.dry_run)
  const hasReport = item.status === 'done' || item.status === 'failed'

  const enabled: Record<Step, boolean> = {
    apercu: reviewable,
    colonnes: reviewable,
    simulation: true,
    import: item.status === 'done' && item.dry_run,
    rapport: hasReport,
  }

  function go(next: Step) {
    void navigate({ search: (prev) => ({ ...prev, step: next, outcome: undefined, page: 1 }) })
  }

  return (
    <PageFrame title={`Import · ${item.file_name}`} trail={[TRAIL]}>
      <div className="flex flex-col gap-3 p-4">
        <nav aria-label="Étapes de l’import">
          <ol className="flex flex-wrap gap-2">
            {STEPS.map((s, index) => (
              <li key={s}>
                <button
                  type="button"
                  disabled={!enabled[s] && s !== step}
                  aria-current={s === step ? 'step' : undefined}
                  onClick={() => go(s)}
                  className={cn(
                    'rounded-[8px] border px-3 py-1.5 text-[13px] disabled:cursor-not-allowed disabled:opacity-50',
                    s === step ? 'border-[#1b1b1b] bg-[#1b1b1b] text-white' : 'border-[#ebebeb] bg-white text-[#1b1b1b]',
                  )}
                >
                  {index + 1}. {STEP_LABEL[s]}
                </button>
              </li>
            ))}
          </ol>
        </nav>

        <p className="text-[13px] text-[#73757c]">
          {entityLabel(item.entity)} · {item.row_count} ligne{item.row_count > 1 ? 's' : ''} · ordre d’import recommandé :{' '}
          {IMPORT_ORDER}.
        </p>

        {step === 'apercu' && <Preview item={item} onNext={() => go('colonnes')} />}

        {step === 'colonnes' && (
          <Card title="Correspondance des colonnes" description="Pour chaque champ de Piilot, la colonne de votre fichier. Les champs marqués d’une étoile sont obligatoires.">
            <MappingStep fields={item.fields} headers={item.headers} mapping={current} onChange={setMapping} />
            <div className="flex justify-end gap-2">
              <Button variant="outline" size="lg" onClick={() => go('apercu')}>Retour</Button>
              <Button size="lg" disabled={missingRequired(item.fields, current).length > 0} onClick={() => go('simulation')}>
                Continuer
              </Button>
            </div>
          </Card>
        )}

        {step === 'simulation' && <Simulation item={item} mapping={current} onMapping={() => go('colonnes')} onNext={() => go('import')} outcome={search.outcome} page={search.page} setFilter={(outcome, page) => void navigate({ search: (prev) => ({ ...prev, outcome, page }) })} />}

        {step === 'import' && <RealRun item={item} />}

        {step === 'rapport' && (
          <Card title={item.dry_run ? 'Rapport de la simulation' : 'Rapport de l’import'} description={item.dry_run ? 'Simulation : rien n’a été enregistré.' : undefined}>
            {item.status === 'failed' && <p role="alert" className="text-[13px] text-[#e5484d]">{failureLabel(item.error)}</p>}
            <ReportPanel
              item={item}
              outcome={search.outcome}
              page={search.page}
              setFilter={(outcome, page) => void navigate({ search: (prev) => ({ ...prev, outcome, page }) })}
            />
          </Card>
        )}
      </div>
    </PageFrame>
  )
}

function Preview({ item, onNext }: { item: ImportDetail; onNext: () => void }) {
  return (
    <Card title="Aperçu du fichier" description={`${item.headers.length} colonnes · séparateur « ${item.delimiter} » · ${item.encoding}`}>
      {item.preview.length === 0 ? (
        <p className="text-[13px] text-[#73757c]">Le fichier n’est plus conservé : aucun aperçu.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-[13px]">
            <thead className="bg-[#f3f4f4] text-[#73757c]">
              <tr>
                {item.headers.map((h, i) => (
                  <th key={`${h}-${i}`} className="px-3 py-2 font-normal whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {item.preview.map((row, r) => (
                <tr key={r} className="border-b border-[#f3f4f4]">
                  {item.headers.map((_, c) => (
                    <td key={c} className="max-w-[220px] truncate px-3 py-2 text-[#1b1b1b]">{row[c] ?? ''}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <div className="flex justify-end">
        <Button size="lg" onClick={onNext}>Choisir les colonnes</Button>
      </div>
    </Card>
  )
}

function Progress({ item }: { item: ImportDetail }) {
  return (
    <p role="status" className="text-[14px] text-[#1b1b1b]">
      {item.status === 'queued' ? 'En attente de traitement…' : 'Traitement en cours…'}{' '}
      <span className="text-[#73757c] tabular-nums">
        {item.created_n} créées · {item.skipped_n} ignorées · {item.error_n} en erreur
      </span>
    </p>
  )
}

function Simulation({
  item,
  mapping,
  outcome,
  page,
  setFilter,
  onMapping,
  onNext,
}: {
  item: ImportDetail
  mapping: Mapping
  outcome: RowOutcome | undefined
  page: number
  setFilter: (outcome: RowOutcome | undefined, page: number) => void
  onMapping: () => void
  onNext: () => void
}) {
  const run = useRunImport(item.id)
  const simulated = item.status === 'done' && item.dry_run

  function launch() {
    run.mutate(
      { mapping: cleanMapping(mapping, item.headers), dryRun: true },
      { onError: (e) => toast.error(e instanceof HttpError ? e.message : 'Lancement impossible') },
    )
  }

  return (
    <Card title="Simulation" description="Le fichier est traité comme pour un vrai import, puis tout est annulé : rien n’est enregistré.">
      {isActive(item.status) ? (
        <Progress item={item} />
      ) : (
        <>
          {item.status === 'failed' && <p role="alert" className="text-[13px] text-[#e5484d]">{failureLabel(item.error)}</p>}
          {simulated && <ReportPanel item={item} outcome={outcome} page={page} setFilter={setFilter} />}
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="lg" onClick={onMapping}>Modifier les colonnes</Button>
            {item.status !== 'failed' && (
              <Button variant={simulated ? 'outline' : 'default'} size="lg" disabled={run.isPending || missingRequired(item.fields, mapping).length > 0} onClick={launch}>
                {simulated ? 'Relancer la simulation' : 'Lancer la simulation'}
              </Button>
            )}
            {simulated && <Button size="lg" onClick={onNext}>Continuer vers l’import</Button>}
          </div>
        </>
      )}
    </Card>
  )
}

function RealRun({ item }: { item: ImportDetail }) {
  const run = useRunImport(item.id)

  function launch() {
    // Le mapping simulé, pas celui du brouillon : on importe ce qu'on a vu.
    run.mutate(
      { mapping: item.mapping, dryRun: false },
      { onError: (e) => toast.error(e instanceof HttpError ? e.message : 'Lancement impossible') },
    )
  }

  return (
    <Card title="Import" description="Les lignes valides sont créées dans Piilot.">
      {isActive(item.status) ? (
        <Progress item={item} />
      ) : (
        <>
          <p className="text-[14px] text-[#1b1b1b]">
            La simulation donne <strong>{item.created_n}</strong> ligne{item.created_n > 1 ? 's' : ''} à créer,{' '}
            {item.skipped_n} ignorée{item.skipped_n > 1 ? 's' : ''} et {item.error_n} en erreur. Les lignes en erreur
            ne sont pas importées.
          </p>
          <p className="text-[13px] text-[#a05a00]">
            L’import est définitif : il ne se défait pas d’un clic. Rejouer le même fichier ne crée pas de doublons.
          </p>
          <div className="flex justify-end">
            <Button size="lg" disabled={run.isPending} onClick={launch}>Lancer l’import</Button>
          </div>
        </>
      )}
    </Card>
  )
}

function ReportPanel({
  item,
  outcome,
  page,
  setFilter,
}: {
  item: ImportDetail
  outcome: RowOutcome | undefined
  page: number
  setFilter: (outcome: RowOutcome | undefined, page: number) => void
}) {
  const { data } = useQuery(importRowsQuery(item.id, outcome, page))

  return <Report item={item} rows={data} outcome={outcome} page={page} onOutcome={(o) => setFilter(o, 1)} onPage={(p) => setFilter(outcome, p)} />
}
