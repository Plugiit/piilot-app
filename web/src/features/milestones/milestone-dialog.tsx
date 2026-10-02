import { PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { useCreateMilestone, useUpdateMilestone } from '@/features/milestones/api'
import { HttpError } from '@/lib/api'
import type { Milestone } from '@/types/api'

/**
 * Creation ou modification d'un jalon.
 *
 * Trois champs : un titre, une echeance, et de quoi preciser. L'etat atteint
 * se coche depuis la liste, d'un geste, sans ouvrir ce dialogue.
 */
export function MilestoneDialog({
  projectId,
  milestone,
  trigger,
}: {
  projectId: string
  /** Absent, le dialogue cree un jalon. */
  milestone?: Milestone
  trigger?: ReactNode
}) {
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [due, setDue] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)

  const create = useCreateMilestone(projectId)
  const update = useUpdateMilestone(milestone?.id ?? '')
  const pending = create.isPending || update.isPending

  function reset() {
    setTitle(milestone?.title ?? '')
    setDue(milestone?.due_on ?? '')
    setDescription(milestone?.description ?? '')
    setError(null)
  }

  function submit() {
    if (title.trim() === '') {
      setError('Le titre est requis')
      return
    }

    const values = { title: title.trim(), description: description.trim(), due_on: due === '' ? null : due }
    const options = {
      onSuccess: () => {
        toast.success(milestone === undefined ? 'Jalon ajouté' : 'Jalon enregistré')
        setOpen(false)
      },
      onError: (err: Error) => toast.error(err instanceof HttpError ? err.message : 'Enregistrement impossible'),
    }

    if (milestone === undefined) create.mutate(values, options)
    else update.mutate(values, options)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (next) reset()
      }}
    >
      <DialogTrigger asChild>
        {trigger ?? (
          <Button size="lg" className="gap-1.5">
            <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
            Nouveau jalon
          </Button>
        )}
      </DialogTrigger>

      <DialogContent className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>{milestone === undefined ? 'Nouveau jalon' : 'Modifier le jalon'}</DialogTitle>
          <DialogDescription>Une étape datée du projet, à laquelle se rattachent des livrables.</DialogDescription>
        </DialogHeader>

        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault()
            submit()
          }}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="milestone-title">Titre</Label>
            <Input
              id="milestone-title"
              autoFocus
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              placeholder="Maquettes validées, mise en ligne…"
              aria-invalid={error !== null}
            />
            {error !== null && <p className="text-[12px] text-[#e5484d]">{error}</p>}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="milestone-due">Échéance</Label>
            <Input id="milestone-due" type="date" value={due} onChange={(event) => setDue(event.target.value)} />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="milestone-description">Description</Label>
            <Textarea
              id="milestone-description"
              rows={3}
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="Ce qui doit être vrai pour que le jalon soit tenu (facultatif)"
            />
          </div>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
              Annuler
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? 'Enregistrement…' : milestone === undefined ? 'Ajouter' : 'Enregistrer'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
