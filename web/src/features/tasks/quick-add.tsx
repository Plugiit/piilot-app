import { PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'

import { useCreateTask } from '@/features/tasks/api'
import { parseQuickTask } from '@/features/tasks/quick-task'
import { HttpError } from '@/lib/api'
import { sessionQuery } from '@/lib/auth'
import { userInitials } from '@/lib/initials'
import { cn } from '@/lib/utils'

/**
 * Ligne d'ajout rapide, en tete d'une liste de taches.
 *
 * On tape, on fait Entree, la tache existe et le champ attend la suivante :
 * dix taches se saisissent en dix lignes. Le libelle porte ses raccourcis
 * (« demain », « lundi », « 12/10 », « !haute ») et la tache est assignee a
 * celui qui la cree, sauf s'il decoche la pastille.
 */
export function QuickAddTask({ projectId, className }: { projectId: string; className?: string }) {
  const { data: me } = useQuery(sessionQuery)
  const create = useCreateTask(projectId)
  const [title, setTitle] = useState('')
  const [mine, setMine] = useState(true)

  const parsed = parseQuickTask(title)

  function submit() {
    if (parsed.title === '' || create.isPending) return

    create.mutate(
      {
        title: parsed.title,
        status: 'todo',
        priority: parsed.priority ?? 'medium',
        due_on: parsed.dueOn ?? null,
        assignee_ids: mine && me !== undefined ? [me.id] : [],
      },
      {
        onSuccess: () => setTitle(''),
        onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Création impossible'),
      },
    )
  }

  return (
    <div className={cn('flex items-center gap-2 border-b border-[#e8e8e9] bg-[#fafafa] px-4 py-2', className)}>
      <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} className="shrink-0 text-[#8d8d8d]" />
      <input
        value={title}
        onChange={(event) => setTitle(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault()
            submit()
          }
          if (event.key === 'Escape') {
            event.stopPropagation()
            setTitle('')
          }
        }}
        placeholder="Ajouter une tâche et appuyer sur Entrée · « demain », « lundi », « 12/10 », « !haute »"
        aria-label="Nouvelle tâche"
        className="h-8 min-w-0 flex-1 bg-transparent text-[14px] text-[#1b1b1b] placeholder:text-[#a2a3a7] focus:outline-none"
      />

      {/* Ce que la ligne a compris, avant qu'on valide. */}
      {(parsed.priority !== undefined || parsed.dueOn !== undefined) && (
        <span className="hidden shrink-0 items-center gap-1.5 text-[12px] text-[#73757c] sm:flex">
          {parsed.dueOn !== undefined && <span>{formatDay(parsed.dueOn)}</span>}
          {parsed.priority !== undefined && (
            <span className="rounded-full bg-[#f3f4f4] px-1.5 py-0.5">
              {{ high: 'Haute', medium: 'Moyenne', low: 'Basse' }[parsed.priority]}
            </span>
          )}
        </span>
      )}

      {me !== undefined && (
        <button
          type="button"
          role="checkbox"
          aria-checked={mine}
          aria-label={mine ? 'Assignée à moi' : 'Non assignée'}
          title={mine ? 'Assignée à moi — cliquer pour ne pas assigner' : 'Non assignée — cliquer pour me l’assigner'}
          onClick={() => setMine((v) => !v)}
          className={cn(
            'flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-full text-[11px] font-medium transition-colors',
            mine ? 'bg-brand text-white' : 'border border-dashed border-[#c4c4c4] text-[#a2a3a7]',
          )}
        >
          {userInitials(me)}
        </button>
      )}
    </div>
  )
}

function formatDay(iso: string): string {
  const [y, m, d] = iso.split('-').map(Number)
  return new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'short' }).format(new Date(y!, m! - 1, d))
}
