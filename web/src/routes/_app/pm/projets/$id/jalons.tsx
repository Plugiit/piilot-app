import {
  Delete02Icon,
  Flag02Icon,
  Link01Icon,
  MoreHorizontalIcon,
  PencilEdit02Icon,
  Unlink02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { toast } from 'sonner'

import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { deliverableListQuery } from '@/features/deliverables/api'
import { DELIVERABLE_STATUS } from '@/features/deliverables/format'
import {
  milestoneListQuery,
  useAttachDeliverable,
  useDeleteMilestone,
  useUpdateMilestone,
} from '@/features/milestones/api'
import { MILESTONE_STATE } from '@/features/milestones/format'
import { MilestoneDialog } from '@/features/milestones/milestone-dialog'
import { DONE_COLOR, parseApiDate, PROGRESS_COLOR } from '@/features/projects/format'
import { Meter, StatusPill } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { can, sessionQuery } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { Milestone } from '@/types/api'

/**
 * Les jalons du projet : ses etapes datees, et les livrables qui les tiennent.
 *
 * Une frise verticale plutot qu'un tableau : un projet en compte une poignee,
 * et ce qu'on lit est une suite — ce qui est passe, ce qui vient.
 */
export const Route = createFileRoute('/_app/pm/projets/$id/jalons')({
  loader: ({ context, params }) =>
    context.queryClient.query({ ...milestoneListQuery(params.id), staleTime: 'static' }),
  component: ProjectMilestonesPage,
})

const LONG = new Intl.DateTimeFormat('fr-FR', { weekday: 'short', day: 'numeric', month: 'long', year: 'numeric' })
const ERROR_TOAST = (error: Error) => toast.error(error instanceof HttpError ? error.message : 'Action impossible')

function ProjectMilestonesPage() {
  const { id } = Route.useParams()
  const { data, isError, error } = useQuery(milestoneListQuery(id))
  const { data: session } = useQuery(sessionQuery)
  const canEdit = can(session, 'projects.write')

  if (isError) {
    return (
      <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
        {error instanceof HttpError ? error.message : 'Chargement impossible'}
      </p>
    )
  }

  const items = data?.items ?? []

  if (items.length === 0) {
    return (
      <div className="flex flex-col items-center gap-3 p-10 text-center">
        <span className="flex size-11 items-center justify-center rounded-full bg-[#f3f4f4] text-[#73757c]">
          <HugeiconsIcon icon={Flag02Icon} size={20} strokeWidth={1.6} />
        </span>
        <p className="text-[15px] font-medium text-[#1b1b1b]">Aucun jalon pour ce projet</p>
        <p className="max-w-[420px] text-[13px] text-[#73757c]">
          Les jalons découpent le projet en étapes datées — cadrage, maquettes validées, mise en ligne — et
          rassemblent les livrables qui les tiennent. Ils apparaissent dans le planning.
        </p>
        {canEdit && <MilestoneDialog projectId={id} />}
      </div>
    )
  }

  return (
    <ol className="flex flex-col p-4">
      {items.map((milestone, index) => (
        <MilestoneRow
          key={milestone.id}
          projectId={id}
          milestone={milestone}
          last={index === items.length - 1}
          canEdit={canEdit}
        />
      ))}
    </ol>
  )
}

function MilestoneRow({
  projectId,
  milestone,
  last,
  canEdit,
}: {
  projectId: string
  milestone: Milestone
  last: boolean
  canEdit: boolean
}) {
  const state = MILESTONE_STATE[milestone.state]
  const update = useUpdateMilestone(milestone.id)
  const remove = useDeleteMilestone(milestone.id)
  const due = parseApiDate(milestone.due_on)
  const ratio =
    milestone.deliverables_total === 0
      ? 0
      : (milestone.deliverables_validated / milestone.deliverables_total) * 100

  return (
    <li className="relative flex gap-4">
      {/* La frise : un point par jalon, relie au suivant. Le point prend la
          couleur de l'etat, le trait reste neutre. */}
      <div className="flex w-5 shrink-0 flex-col items-center pt-[18px]">
        <span
          aria-hidden
          className="size-3 shrink-0 rounded-full ring-4 ring-white"
          style={{ backgroundColor: state.color }}
        />
        {!last && <span aria-hidden className="mt-1 w-px flex-1 bg-[#e8e8e9]" />}
      </div>

      <article className="mb-3 flex min-w-0 flex-1 flex-col gap-3 rounded-[12px] border border-[#e8e8e9] bg-white p-3">
        <header className="flex items-start gap-3">
          {canEdit && (
            <Checkbox
              checked={milestone.state === 'done'}
              disabled={update.isPending}
              aria-label={milestone.state === 'done' ? 'Rouvrir le jalon' : 'Marquer le jalon atteint'}
              onCheckedChange={(value) => update.mutate({ done: value === true }, { onError: ERROR_TOAST })}
              className="mt-0.5 size-5 rounded-[6px]"
            />
          )}

          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <h3
              className={cn(
                'text-[15px] font-medium text-[#1b1b1b]',
                milestone.state === 'done' && 'text-[#73757c] line-through decoration-[#c4c4c4]',
              )}
            >
              {milestone.title}
            </h3>
            <p className={cn('text-[13px] first-letter:uppercase', milestone.state === 'late' ? 'text-[#e5484d]' : 'text-[#73757c]')}>
              {due === null ? 'Sans échéance' : LONG.format(due)}
            </p>
            {milestone.description !== '' && (
              <p className="pt-1 text-[13px] leading-[1.5] text-[#4b4b4f]">{milestone.description}</p>
            )}
          </div>

          <StatusPill label={state.label} color={state.color} pill={state.pill} />

          {canEdit && (
            <DropdownMenu>
              <DropdownMenuTrigger
                aria-label={`Actions sur ${milestone.title}`}
                className="flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-[8px] text-[#73757c] hover:bg-[#f3f4f4]"
              >
                <HugeiconsIcon icon={MoreHorizontalIcon} size={16} strokeWidth={1.8} />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-44">
                <MilestoneDialog
                  projectId={projectId}
                  milestone={milestone}
                  trigger={
                    <DropdownMenuItem onSelect={(event) => event.preventDefault()}>
                      <HugeiconsIcon icon={PencilEdit02Icon} size={14} strokeWidth={1.8} />
                      Modifier
                    </DropdownMenuItem>
                  }
                />
                <DropdownMenuItem
                  variant="destructive"
                  onSelect={() =>
                    remove.mutate(undefined, {
                      onSuccess: () => toast.success('Jalon supprimé. Ses livrables restent dans le projet.'),
                      onError: ERROR_TOAST,
                    })
                  }
                >
                  <HugeiconsIcon icon={Delete02Icon} size={14} strokeWidth={1.8} />
                  Supprimer
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </header>

        <div className="flex flex-col gap-2 rounded-[10px] bg-[#fafafa] p-2.5">
          <div className="flex items-center gap-3">
            <span className="shrink-0 text-[12px] text-[#73757c] tabular-nums">
              {milestone.deliverables_total === 0
                ? 'Aucun livrable rattaché'
                : `${milestone.deliverables_validated} validé${milestone.deliverables_validated > 1 ? 's' : ''} sur ${milestone.deliverables_total} livrable${milestone.deliverables_total > 1 ? 's' : ''}`}
            </span>
            {milestone.deliverables_total > 0 && (
              <Meter ratio={ratio} color={ratio >= 100 ? DONE_COLOR : PROGRESS_COLOR} className="max-w-[220px]" />
            )}
          </div>

          {milestone.deliverables.length > 0 && (
            <ul className="flex flex-col">
              {milestone.deliverables.map((deliverable) => {
                const status = DELIVERABLE_STATUS[deliverable.status]

                return (
                  <li key={deliverable.id} className="group flex items-center gap-2 py-1">
                    <span className="min-w-0 flex-1 truncate text-[13px] text-[#1b1b1b]">{deliverable.title}</span>
                    <StatusPill label={status.label} color={status.pill.text} pill={status.pill} />
                    {canEdit && <DetachButton deliverableId={deliverable.id} title={deliverable.title} />}
                  </li>
                )
              })}
            </ul>
          )}

          {canEdit && <AttachPicker projectId={projectId} milestone={milestone} />}
        </div>
      </article>
    </li>
  )
}

function DetachButton({ deliverableId, title }: { deliverableId: string; title: string }) {
  const attach = useAttachDeliverable()

  return (
    <button
      type="button"
      aria-label={`Détacher « ${title} »`}
      title="Détacher du jalon"
      disabled={attach.isPending}
      onClick={() => attach.mutate({ deliverableId, milestoneId: null }, { onError: ERROR_TOAST })}
      className="cursor-pointer text-[#a2a3a7] opacity-0 transition-opacity group-hover:opacity-100 hover:text-[#1b1b1b] focus-visible:opacity-100"
    >
      <HugeiconsIcon icon={Unlink02Icon} size={14} strokeWidth={1.8} />
    </button>
  )
}

/**
 * Rattache un livrable du projet au jalon. La liste propose ceux qui n'ont pas
 * encore de jalon, puis ceux d'un autre — choisir l'un d'eux le deplace.
 */
function AttachPicker({ projectId, milestone }: { projectId: string; milestone: Milestone }) {
  const { data } = useQuery(deliverableListQuery({ projectId }, 1))
  const attach = useAttachDeliverable()

  const candidates = (data?.items ?? [])
    .filter((item) => item.milestone?.id !== milestone.id)
    .sort((a, b) => Number(a.milestone !== null) - Number(b.milestone !== null))

  if (candidates.length === 0) return null

  return (
    <Select
      value=""
      onValueChange={(deliverableId) =>
        attach.mutate(
          { deliverableId, milestoneId: milestone.id },
          { onSuccess: () => toast.success('Livrable rattaché'), onError: ERROR_TOAST },
        )
      }
    >
      <SelectTrigger className="h-8 w-fit gap-1.5 border-dashed bg-white text-[12px] text-[#73757c]">
        <HugeiconsIcon icon={Link01Icon} size={14} strokeWidth={1.8} />
        <SelectValue placeholder="Rattacher un livrable" />
      </SelectTrigger>
      <SelectContent>
        {candidates.map((item) => (
          <SelectItem key={item.id} value={item.id}>
            {item.title}
            {item.milestone !== null && (
              <span className="text-[#a2a3a7]"> — depuis « {item.milestone.name} »</span>
            )}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
