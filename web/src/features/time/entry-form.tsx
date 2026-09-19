import { PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { projectListQuery } from '@/features/projects/api'
import { taskBoardQuery } from '@/features/tasks/api'
import { serviceOptionsQuery } from '@/features/services/api'
import { useCreateTimeEntry } from '@/features/time/api'
import { parseDuration } from '@/features/time/format'
import { HttpError } from '@/lib/api'

/** Valeur du choix « aucun » : Radix refuse la chaine vide comme option. */
const AUCUN = 'aucun'

/**
 * Saisie rapide d'une ligne de temps.
 *
 * Une rangee et non un dialogue : pointer est un geste qu'on repete cinq fois
 * de suite le soir, et rouvrir une fenetre a chaque ligne le rendrait penible.
 * Le champ de duree garde le foyer apres l'envoi, pour enchainer.
 *
 * La tache et le service suivent le projet choisi : une tache appartient a un
 * projet, et proposer celles des autres ferait chercher dans une liste ou la
 * bonne reponse ne peut pas se trouver.
 */
export function TimeEntryForm({ day }: { day: string }) {
  const [projectId, setProjectId] = useState('')
  const [taskId, setTaskId] = useState(AUCUN)
  const [serviceId, setServiceId] = useState(AUCUN)
  const [duration, setDuration] = useState('')
  const [note, setNote] = useState('')
  const [erreur, setErreur] = useState<string | null>(null)

  const create = useCreateTimeEntry()

  const { data: projects } = useQuery(
    projectListQuery({ page: 1, pageSize: 100, sort: 'name', dir: 'asc' }),
  )

  // Les taches ne partent qu'une fois le projet connu : sans lui, la requete
  // n'aurait pas de quoi choisir.
  const { data: board } = useQuery({ ...taskBoardQuery(projectId), enabled: projectId !== '' })
  const { data: services } = useQuery(serviceOptionsQuery())

  function submit() {
    if (projectId === '') {
      setErreur('Choisissez un projet')

      return
    }

    const minutes = parseDuration(duration)
    if (minutes === null || minutes <= 0) {
      setErreur('Durée attendue : 90, 1h30 ou 1,5')

      return
    }

    setErreur(null)

    create.mutate(
      {
        project_id: projectId,
        task_id: taskId === AUCUN ? null : taskId,
        service_id: serviceId === AUCUN ? null : serviceId,
        spent_on: day,
        minutes,
        note: note.trim(),
      },
      {
        onSuccess: () => {
          // Le projet et le service restent : on pointe souvent plusieurs
          // lignes de suite sur le meme chantier.
          setDuration('')
          setNote('')
          setTaskId(AUCUN)
        },
        onError: (error) =>
          toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
      },
    )
  }

  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        submit()
      }}
      className="flex flex-col gap-2 rounded-[12px] border border-[#e8e8e9] bg-white p-3"
    >
      <div className="flex flex-wrap items-center gap-2">
        <Select value={projectId} onValueChange={(value) => { setProjectId(value); setTaskId(AUCUN) }}>
          <SelectTrigger className="h-9 min-w-[180px] flex-[2_1_180px] text-[13px]">
            <SelectValue placeholder="Projet" />
          </SelectTrigger>
          <SelectContent>
            {(projects?.items ?? []).map((project) => (
              <SelectItem key={project.id} value={project.id}>
                {project.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={taskId} onValueChange={setTaskId} disabled={projectId === ''}>
          <SelectTrigger className="h-9 min-w-[160px] flex-[2_1_160px] text-[13px]">
            <SelectValue placeholder="Tâche" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={AUCUN}>
              <span className="text-[#73757c]">Sans tâche</span>
            </SelectItem>
            {(board?.items ?? []).map((task) => (
              <SelectItem key={task.id} value={task.id}>
                {task.title}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={serviceId} onValueChange={setServiceId}>
          <SelectTrigger className="h-9 min-w-[150px] flex-[1_1_150px] text-[13px]">
            <SelectValue placeholder="Service" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={AUCUN}>
              <span className="text-[#73757c]">Sans service</span>
            </SelectItem>
            {(services?.items ?? []).map((service) => (
              <SelectItem key={service.id} value={service.id}>
                <span
                  aria-hidden
                  className="size-2 shrink-0 rounded-full"
                  style={{ backgroundColor: service.color }}
                />
                {service.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Input
          value={duration}
          onChange={(event) => setDuration(event.target.value)}
          placeholder="1h30"
          aria-label="Durée"
          className="h-9 w-[90px] shrink-0 text-center text-[13px] tabular-nums"
        />

        <Button type="submit" size="lg" disabled={create.isPending} className="gap-1.5">
          <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
          {create.isPending ? 'Ajout…' : 'Pointer'}
        </Button>
      </div>

      <Input
        value={note}
        onChange={(event) => setNote(event.target.value)}
        placeholder="Ce qui a été fait (facultatif)"
        aria-label="Note"
        className="h-9 text-[13px]"
      />

      {erreur !== null && <p className="text-[12px] text-[#e5484d]">{erreur}</p>}
    </form>
  )
}
