import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import {
  clientListQuery,
  peopleQuery,
  projectDetailQuery,
  useSetTeam,
  useUpdateProject,
} from '@/features/projects/api'
import { Avatars, TeamPicker } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import type { ProjectDetail } from '@/types/api'

import { Card, Field, SaveBar } from '@/components/settings-ui'

export const Route = createFileRoute('/_app/pm/projets/$id_/parametres/equipe')({
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.query({ ...peopleQuery, staleTime: 'static' }),
      context.queryClient.query({ ...clientListQuery(), staleTime: 'static' }),
    ]),
  component: TeamPage,
})

function TeamPage() {
  const { id } = Route.useParams()
  const { data: project } = useQuery(projectDetailQuery(id))

  if (project === undefined) return null

  return <TeamForm key={project.id} project={project} />
}

/**
 * Client et equipe.
 *
 * Deux ecritures distinctes derriere un seul bouton : le client passe par la
 * modification du projet, l'equipe par son endpoint propre, qui prend la liste
 * entiere. On n'envoie que ce qui a bouge.
 */
function TeamForm({ project }: { project: ProjectDetail }) {
  const { data: people } = useQuery(peopleQuery)
  const { data: clients } = useQuery(clientListQuery())
  const update = useUpdateProject(project.id)
  const setTeam = useSetTeam(project.id)

  const initiale = project.team.map((member) => member.id)
  const [clientID, setClientID] = useState(project.client_id)
  const [membres, setMembres] = useState<ReadonlySet<string>>(() => new Set(initiale))

  const clientModifie = clientID !== project.client_id
  const equipeModifiee =
    membres.size !== initiale.length || initiale.some((memberID) => !membres.has(memberID))
  const modifie = clientModifie || equipeModifiee
  const enCours = update.isPending || setTeam.isPending

  function bascule(memberID: string) {
    setMembres((prev) => {
      const next = new Set(prev)

      if (next.has(memberID)) next.delete(memberID)
      else next.add(memberID)

      return next
    })
  }

  function annuler() {
    setClientID(project.client_id)
    setMembres(new Set(initiale))
  }

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()

    try {
      if (clientModifie) await update.mutateAsync({ client_id: clientID })
      if (equipeModifiee) await setTeam.mutateAsync([...membres])

      toast.success('Équipe enregistrée')
    } catch (error) {
      toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible')
    }
  }

  const choisis = (people?.items ?? []).filter((person) => membres.has(person.id))

  return (
    <form noValidate onSubmit={onSubmit} className="flex flex-col gap-3 p-4">
      <Card title="Client" description="À qui ce projet est vendu.">
        <Field label="Client">
          <select
            value={clientID}
            onChange={(event) => setClientID(event.target.value)}
            className="h-auto w-full cursor-pointer rounded-[12px] border border-[#e8e8e9] bg-white p-3 text-[16px] leading-[1.5] text-[#1b1b1b]"
          >
            {(clients?.items ?? []).map((client) => (
              <option key={client.id} value={client.id}>
                {client.name}
              </option>
            ))}
          </select>
        </Field>
      </Card>

      <Card
        title="Équipe affectée"
        description="Les comptes internes qui travaillent sur ce projet."
      >
        <Field label={`Membres (${membres.size})`}>
          {choisis.length > 0 && (
            <div className="mb-1 flex items-center gap-2">
              <Avatars people={choisis} max={8} size={28} />
            </div>
          )}

          <TeamPicker people={people?.items ?? []} selected={membres} onToggle={bascule} />
        </Field>
      </Card>

      <SaveBar dirty={modifie} pending={enCours} onCancel={annuler} />
    </form>
  )
}
