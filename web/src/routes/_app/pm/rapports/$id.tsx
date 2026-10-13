import { RefreshIcon, SentIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { monthLabel } from '@/features/maintenance/format'
import { StatusPill } from '@/features/projects/ui'
import { reportQuery, useReportAction } from '@/features/reports/api'
import { REPORT_STATUS } from '@/features/reports/format'
import { ReportView } from '@/features/reports/report-view'
import { HttpError } from '@/lib/api'
import type { Report } from '@/types/api'

/**
 * La relecture d'un rapport : l'apercu tel que le client le lira, et a cote
 * le mot du mois, les gestes et les destinataires. Brouillon → valide →
 * envoye ; envoye, il ne change plus.
 */
export const Route = createFileRoute('/_app/pm/rapports/$id')({
  loader: ({ context, params }) => context.queryClient.ensureQueryData(reportQuery(params.id)),
  component: ReportPage,
})

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })

function ReportPage() {
  const { id } = Route.useParams()
  const { data } = useQuery(reportQuery(id))
  if (data === undefined) return null
  return (
    <PageFrame title={data.project.name} trail={[{ label: 'Rapports mensuels', to: '/pm/rapports', search: { mois: data.month } }]}>
      <div className="grid grid-cols-1 gap-4 p-4 xl:grid-cols-[minmax(0,1fr)_340px]">
        <ReportView data={data.data} editorial={data.editorial} />
        <aside className="flex flex-col gap-3 xl:sticky xl:top-4 xl:self-start">
          <Actions report={data} />
          <Editorial key={data.editorial + data.status} report={data} />
          <Recipients report={data} />
        </aside>
      </div>
    </PageFrame>
  )
}

function Box({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-3 rounded-[12px] border border-[#e8e8e9] bg-white p-4">
      <h2 className="text-[14px] font-medium text-[#1b1b1b]">{title}</h2>
      {children}
    </section>
  )
}

function fail(e: unknown) {
  toast.error(e instanceof HttpError ? e.message : 'Action impossible')
}

function Actions({ report }: { report: Report }) {
  const act = useReportAction(report.id)
  const busy = Object.values(act).some((m) => m.isPending)
  return (
    <Box title={`Rapport de ${monthLabel(report.month).toLowerCase()}`}>
      <div className="flex items-center gap-2">
        <StatusPill {...REPORT_STATUS[report.status]} />
        <span className="text-[12px] text-[#73757c]">
          {report.status === 'sent' && report.sent_at !== null
            ? `le ${WHEN.format(new Date(report.sent_at))}`
            : `généré le ${WHEN.format(new Date(report.generated_at))}`}
        </span>
      </div>
      {report.status === 'draft' && (
        <div className="flex flex-col gap-2">
          <Button type="button" disabled={busy} onClick={() => act.validate.mutate(undefined, { onSuccess: () => toast.success('Rapport validé'), onError: fail })}>
            Valider le rapport
          </Button>
          <Button
            type="button"
            variant="outline"
            className="gap-1.5"
            disabled={busy}
            onClick={() => act.regenerate.mutate(undefined, { onSuccess: () => toast.success('Données régénérées'), onError: fail })}
          >
            <HugeiconsIcon icon={RefreshIcon} size={14} strokeWidth={1.8} />
            Régénérer les données
          </Button>
          <p className="text-[12px] text-[#8d8d8d]">Pour une intervention saisie en retard, ou des chiffres d’audience arrivés depuis.</p>
        </div>
      )}
      {report.status === 'validated' && (
        <div className="flex flex-col gap-2">
          <Button
            type="button"
            className="gap-1.5"
            disabled={busy || report.recipients.length === 0}
            onClick={() => act.send.mutate(undefined, { onSuccess: (r) => toast.success(`Rapport envoyé à ${r.recipients.length} destinataire${r.recipients.length > 1 ? 's' : ''}`), onError: fail })}
          >
            <HugeiconsIcon icon={SentIcon} size={14} strokeWidth={1.8} />
            Envoyer au client
          </Button>
          <Button type="button" variant="outline" disabled={busy} onClick={() => act.unvalidate.mutate(undefined, { onError: fail })}>
            Repasser en brouillon
          </Button>
          <p className="text-[12px] text-[#8d8d8d]">Une fois envoyé, le rapport ne change plus.</p>
        </div>
      )}
      {report.status === 'sent' && <p className="text-[13px] text-[#73757c]">Envoyé : il est figé, et le client le retrouve dans son espace.</p>}
    </Box>
  )
}

function Editorial({ report }: { report: Report }) {
  const act = useReportAction(report.id)
  const [text, setText] = useState(report.editorial)
  const editable = report.status === 'draft'
  return (
    <Box title="Le mot du mois">
      {editable ? (
        <>
          <Textarea
            value={text}
            rows={6}
            maxLength={4000}
            placeholder="Ce qui a marqué le mois, et nos recommandations pour le suivant."
            onChange={(e) => setText(e.target.value)}
          />
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="self-end"
            disabled={text === report.editorial || act.editorial.isPending}
            onClick={() => act.editorial.mutate(text, { onSuccess: () => toast.success('Mot du mois enregistré'), onError: fail })}
          >
            Enregistrer
          </Button>
        </>
      ) : (
        <p className="text-[13px] whitespace-pre-line text-[#4b4b4f]">{report.editorial === '' ? 'Aucun.' : report.editorial}</p>
      )}
    </Box>
  )
}

function Recipients({ report }: { report: Report }) {
  return (
    <Box title={report.status === 'sent' ? 'Envoyé à' : 'Destinataires'}>
      {report.recipients.length === 0 ? (
        <p className="text-[13px] text-[#9a6a00]">Aucun destinataire : invitez un contact au portail ou ajoutez une adresse au projet.</p>
      ) : (
        <ul className="flex flex-col gap-1">
          {report.recipients.map((r) => (
            <li key={r} className="truncate text-[13px] text-[#1b1b1b]">
              {r}
            </li>
          ))}
        </ul>
      )}
      {report.status !== 'sent' && (
        <Link to="/pm/projets/$id/parametres/maintenance" params={{ id: report.project.id }} className="text-[13px] text-[#73757c] underline hover:text-[#1b1b1b]">
          Gérer les destinataires du projet
        </Link>
      )}
    </Box>
  )
}
