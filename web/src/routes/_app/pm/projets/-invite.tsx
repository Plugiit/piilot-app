import { PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
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
import { peopleQuery, useSetTeam } from '@/features/projects/api'
import { Avatars, TeamPicker } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import type { ProjectDetail } from '@/types/api'

/**
 * Invitation au projet, depuis son en-tete.
 *
 * L'affectation n'est pas qu'un affichage d'avatars : un compte hors de
 * l'equipe n'a pas acces au projet. Elle ne pouvait se faire que dans les
 * parametres, deux ecrans plus loin, alors que la question se pose la ou l'on
 * voit qui travaille dessus. Le meme geste est donc offert aux deux endroits,
 * et les deux ecrivent par le meme endpoint.
 */
export function InviteDialog({ project }: { project: ProjectDetail }) {
  const [open, setOpen] = useState(false)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" size="lg" className="gap-2">
          <HugeiconsIcon icon={PlusSignIcon} size={20} strokeWidth={1.8} />
          Inviter
        </Button>
      </DialogTrigger>

      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>Inviter au projet</DialogTitle>
          <DialogDescription>
            Seuls les comptes cochés ont accès au projet.
          </DialogDescription>
        </DialogHeader>

        {/* Monte a l'ouverture, demonte a la fermeture : la liste des comptes
            n'est demandee que si la modale sert, et la selection repart de
            l'equipe reelle a chaque ouverture plutot que de garder celle d'une
            hesitation precedente. */}
        {open && <InviteForm project={project} onDone={() => setOpen(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function InviteForm({ project, onDone }: { project: ProjectDetail; onDone: () => void }) {
  const { data: people, isPending } = useQuery(peopleQuery)
  const setTeam = useSetTeam(project.id)

  const initiale = project.team.map((member) => member.id)
  const [membres, setMembres] = useState<ReadonlySet<string>>(() => new Set(initiale))

  const modifie =
    membres.size !== initiale.length || initiale.some((memberID) => !membres.has(memberID))

  function bascule(personID: string) {
    setMembres((prev) => {
      const next = new Set(prev)

      if (next.has(personID)) next.delete(personID)
      else next.add(personID)

      return next
    })
  }

  // L'endpoint prend la liste entiere : on envoie l'etat coche, pas un delta.
  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()

    try {
      await setTeam.mutateAsync([...membres])
      toast.success('Équipe mise à jour')
      onDone()
    } catch (error) {
      toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible')
    }
  }

  const choisis = (people?.items ?? []).filter((person) => membres.has(person.id))

  return (
    <form noValidate onSubmit={onSubmit} className="flex min-h-0 flex-col gap-4">
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <p className="text-[14px] text-[#73757c]">Membres ({membres.size})</p>
          {choisis.length > 0 && <Avatars people={choisis} max={8} size={24} />}
        </div>

        {/* La liste des comptes internes n'a pas de plafond : elle defile dans
            la modale plutot que de la pousser hors de l'ecran. */}
        <div className="max-h-[320px] overflow-y-auto">
          {isPending ? (
            <p className="text-[14px] text-[#73757c]">Chargement…</p>
          ) : (
            <TeamPicker people={people?.items ?? []} selected={membres} onToggle={bascule} />
          )}
        </div>
      </div>

      <DialogFooter>
        <Button type="button" variant="ghost" size="lg" onClick={onDone}>
          Annuler
        </Button>
        <Button type="submit" size="lg" disabled={!modifie || setTeam.isPending}>
          {setTeam.isPending ? 'Enregistrement…' : 'Enregistrer'}
        </Button>
      </DialogFooter>
    </form>
  )
}
