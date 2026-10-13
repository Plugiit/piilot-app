import { ArrowLeft01Icon, PrinterIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'

import { Button } from '@/components/ui/button'
import { portalReportQuery } from '@/features/reports/api'
import { ReportView } from '@/features/reports/report-view'
import { HttpError } from '@/lib/api'

/**
 * Un rapport mensuel, pour le client. Le lien de l'e-mail mene ici, apres
 * connexion. « Telecharger en PDF » passe par l'impression du navigateur :
 * la feuille d'impression ne garde que le rapport.
 */
export const Route = createFileRoute('/client/rapports/$id')({
  loader: ({ context, params }) => context.queryClient.ensureQueryData(portalReportQuery(params.id)),
  component: ReportPage,
  errorComponent: ({ error }) => (
    <p className="rounded-[14px] border border-[#e8e8e9] bg-white p-6 text-[15px] text-[#4b4b4f]">
      {error instanceof HttpError && error.status === 404 ? 'Ce rapport est introuvable, ou il ne fait pas partie de votre espace.' : error.message}
    </p>
  ),
})

function ReportPage() {
  const { id } = Route.useParams()
  const { data } = useQuery(portalReportQuery(id))
  if (data === undefined) return null
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3 print:hidden">
        <Link to="/client/projets/$id/maintenance" params={{ id: data.project.id }} className="flex items-center gap-1 text-[14px] text-[#73757c] hover:text-[#1b1b1b]">
          <HugeiconsIcon icon={ArrowLeft01Icon} size={16} strokeWidth={1.8} />
          {data.project.name}
        </Link>
        <Button type="button" variant="outline" size="sm" className="gap-1.5" onClick={() => window.print()}>
          <HugeiconsIcon icon={PrinterIcon} size={14} strokeWidth={1.8} />
          Télécharger en PDF
        </Button>
      </div>
      <ReportView data={data.data} editorial={data.editorial} />
    </div>
  )
}
