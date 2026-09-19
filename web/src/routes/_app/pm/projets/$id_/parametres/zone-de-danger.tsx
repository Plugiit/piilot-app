import { Alert02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { projectDetailQuery, useDeleteProject } from '@/features/projects/api'
import { HttpError } from '@/lib/api'
import type { ProjectDetail } from '@/types/api'

import { CHAMP, Card, Field } from '@/components/settings-ui'

export const Route = createFileRoute('/_app/pm/projets/$id_/parametres/zone-de-danger')({
  component: DangerPage,
})

function DangerPage() {
  const { id } = Route.useParams()
  const { data: project } = useQuery(projectDetailQuery(id))

  if (project === undefined) return null

  return <DangerForm key={project.id} project={project} />
}

/**
 * Suppression du projet.
 *
 * Recopier le nom plutot que cocher une case : c'est le seul garde-fou qui
 * resiste au clic machinal, et il oblige a lire ce qu'on s'apprete a effacer.
 *
 * La suppression est logique cote serveur — la ligne porte une date de
 * suppression, elle ne part pas de la base. C'est dit ici, parce qu'un ecran
 * qui annonce « definitif » quand ca ne l'est pas se trompe dans les deux sens.
 */
function DangerForm({ project }: { project: ProjectDetail }) {
  const navigate = useNavigate()
  const remove = useDeleteProject()
  const [saisie, setSaisie] = useState('')

  const confirme = saisie.trim() === project.name

  function supprimer() {
    remove.mutate(project.id, {
      onSuccess: () => {
        toast.success(`« ${project.name} » a été supprimé`)
        void navigate({ to: '/pm/projets', search: { page: 1, sort: 'due', dir: 'asc' } })
      },
      onError: (error) =>
        toast.error(error instanceof HttpError ? error.message : 'Suppression impossible'),
    })
  }

  return (
    <div className="flex flex-col gap-3 p-4">
      <section className="flex w-full flex-col gap-1 rounded-[12px] bg-[#fdf3f3] p-1">
        <header className="flex items-start gap-2 p-2">
          <HugeiconsIcon
            icon={Alert02Icon}
            size={20}
            strokeWidth={1.8}
            className="mt-0.5 shrink-0 text-[#e5484d]"
          />
          <div className="flex flex-col gap-0.5">
            <h2 className="text-[18px] leading-[1.5] font-medium tracking-[-0.18px] text-[#e5484d]">
              Supprimer le projet
            </h2>
            <p className="text-[14px] leading-[1.5] text-[#8a5a5a]">
              Ses tâches, ses pièces jointes et son historique cessent d’être accessibles.
            </p>
          </div>
        </header>

        <div className="flex w-full flex-col gap-3 rounded-[10px] bg-white p-2">
          <Field
            label="Confirmation"
            hint={`Recopiez « ${project.name} » pour débloquer la suppression.`}
          >
            <Input
              value={saisie}
              onChange={(event) => setSaisie(event.target.value)}
              placeholder={project.name}
              aria-label="Nom du projet à recopier"
              className={CHAMP}
            />
          </Field>

          <div className="flex items-center justify-between gap-4">
            <p className="text-[13px] text-[#73757c]">
              La suppression est réversible en base : la ligne est marquée, pas effacée. Aucun écran
              ne permet encore de la rétablir.
            </p>

            <Button
              type="button"
              variant="destructive"
              size="lg"
              disabled={!confirme || remove.isPending}
              onClick={supprimer}
              className="shrink-0"
            >
              {remove.isPending ? 'Suppression…' : 'Supprimer ce projet'}
            </Button>
          </div>
        </div>
      </section>

      <Card title="Archiver" description="Bientôt : sortir le projet des listes sans le supprimer.">
        <p className="text-[14px] text-[#73757c]">
          L’archivage n’existe pas encore côté API — il demanderait une colonne distincte de la
          suppression. En attendant, le statut « Livré » sort un projet du flux de travail.
        </p>
      </Card>
    </div>
  )
}
