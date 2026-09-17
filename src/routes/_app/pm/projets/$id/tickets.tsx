import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import {
  TICKETS_PAGE_SIZE,
  projectTicketsBoardQuery,
  projectTicketsQuery,
} from '@/features/tickets/api'
import { TicketBoard, type BoardColumn } from '@/features/tickets/board'
import { TICKET_STATUS, TICKET_STATUS_ORDER } from '@/features/tickets/format'
import { ProjectTicketTable } from '@/features/tickets/list'
import { HttpError } from '@/lib/api'

/**
 * Les tickets du projet, en tableau ou en kanban.
 *
 * Tous les tickets du projet et non les seuls miens, contrairement a l'ecran
 * « Tickets » du module : devant un projet, la question est « qui a quoi ».
 *
 * `?page` ne sert qu'au tableau — le kanban est borne, pas pagine. Il reste
 * declare ici pour les deux : le sortir du schema le ferait disparaitre de
 * l'adresse en passant au kanban, et revenir au tableau ramenerait page 1.
 */
const searchSchema = z.object({
  page: z.number().int().min(1).catch(1),
})

export const Route = createFileRoute('/_app/pm/projets/$id/tickets')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ page: search.page, vue: search.vue }),
  // Seule la vue ouverte est prechargee : le kanban et le tableau lisent deux
  // endpoints, et charger les deux ferait deux requetes pour un seul ecran.
  loader: async ({ context, params, deps }) => {
    if (deps.vue === 'kanban') {
      await context.queryClient.query({
        ...projectTicketsBoardQuery(params.id),
        staleTime: 'static',
      })

      return
    }

    await context.queryClient.query({
      ...projectTicketsQuery(params.id, deps.page),
      staleTime: 'static',
    })
  },
  component: ProjectTicketsPage,
})

/** Ce que les deux vues disent quand le projet n'a pas encore de ticket. */
const VIDE = 'Aucun ticket sur ce projet. Déposez le premier depuis « Déposer un ticket ».'

function ProjectTicketsPage() {
  const { vue } = Route.useSearch()

  return vue === 'kanban' ? <KanbanView /> : <TableView />
}

function TableView() {
  const { id } = Route.useParams()
  const { page } = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data, isError, error } = useQuery(projectTicketsQuery(id, page))

  if (isError) {
    return <Erreur error={error} />
  }

  const total = data?.total ?? 0
  const current = data?.page ?? page
  const totalPages = Math.max(1, Math.ceil(total / TICKETS_PAGE_SIZE))

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <ProjectTicketTable tickets={data?.items ?? []} empty={VIDE} />

      <div className="flex flex-wrap items-center justify-between gap-2 p-4">
        <p className="text-[12px] text-[#777]">
          {total} ticket{total > 1 ? 's' : ''} · page {current} sur {totalPages}
        </p>

        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={current <= 1}
            onClick={() => void navigate({ search: (prev) => ({ ...prev, page: current - 1 }) })}
          >
            Précédent
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={current >= totalPages}
            onClick={() => void navigate({ search: (prev) => ({ ...prev, page: current + 1 }) })}
          >
            Suivant
          </Button>
        </div>
      </div>
    </div>
  )
}

/**
 * Kanban par statut.
 *
 * Les sept colonnes sont toujours la, meme vides : ce sont les etapes du cycle,
 * pas un resume des donnees. Pas de regroupement par projet ici — dans un
 * projet, il ne ferait qu'une colonne.
 */
function KanbanView() {
  const { id } = Route.useParams()
  const { data, isError, error } = useQuery(projectTicketsBoardQuery(id))

  if (isError) {
    return <Erreur error={error} />
  }

  const tickets = data?.items ?? []

  const columns: BoardColumn[] = TICKET_STATUS_ORDER.map((status) => ({
    key: status,
    label: TICKET_STATUS[status]!.label,
    color: TICKET_STATUS[status]!.pill.text,
    tickets: tickets.filter((ticket) => ticket.status === status),
  }))

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {data?.truncated === true && (
        <p className="px-4 pt-4 text-[12px] text-[#73757c]">
          Seules les 300 premières cartes sont affichées. Le tableau les montre toutes.
        </p>
      )}

      {/* Pas de `showProject` : la fiche du projet le dit deja, et le repeter
          sur chaque carte prendrait la place du sujet. */}
      <TicketBoard columns={columns} empty={VIDE} />
    </div>
  )
}

function Erreur({ error }: { error: unknown }) {
  return (
    <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
      {error instanceof HttpError ? error.message : 'Chargement impossible'}
    </p>
  )
}
