import { RefreshIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'

import { Card } from '@/components/settings-ui'
import { Button } from '@/components/ui/button'
import { gitSyncQuery, useRequestGitSync } from '@/features/git-sync/api'
import { HttpError } from '@/lib/api'

const WHEN = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric', hour: '2-digit', minute: '2-digit' })

/**
 * Le rattrapage des pull requests fusionnees : une fois par jour, et le 1er
 * avant le rapport, Piilot relit GitHub pour ce que le webhook a manque. Rien
 * ici n'appelle GitHub : « Synchroniser » le demande a la tache de fond.
 * Sans depot GitHub, la carte n'a rien a dire.
 */
export function GitSyncCard({ projectId, canSync }: { projectId: string; canSync: boolean }) {
  const { data } = useQuery(gitSyncQuery(projectId))
  const request = useRequestGitSync(projectId)
  if (data === undefined || !data.eligible) return null

  const pending = request.isPending || data.pending

  return (
    <Card
      title="Pull requests fusionnées"
      description="Le webhook Git alimente les mises à jour du rapport. Piilot relit aussi GitHub chaque jour, et le 1er avant de créer le rapport, pour rattraper ce qu’une coupure du webhook a fait manquer. Le jeton GitHub de la carte « Mises à jour » doit avoir la permission « Pull requests : lecture »."
    >
      {!data.token_configured && <p className="px-1 text-[14px] text-[#9a6a00]">Aucun jeton GitHub enregistré : le rattrapage ne peut pas lire GitHub.</p>}
      <p className="px-1 text-[14px] text-[#1b1b1b]">
        {data.last_synced_at === null ? 'Jamais synchronisé avec succès.' : `Dernière synchronisation réussie le ${WHEN.format(new Date(data.last_synced_at))}.`}
      </p>
      {data.last_error !== null && (
        <p className="px-1 text-[14px] text-[#a30f2c]">
          Dernier essai en échec{data.last_attempt_at !== null && ` (${WHEN.format(new Date(data.last_attempt_at))})`} : {data.last_error}
        </p>
      )}
      {canSync && (
        <div>
          <Button
            type="button"
            variant="outline"
            className="gap-1.5"
            disabled={pending}
            onClick={() => request.mutate(undefined, { onError: (e) => toast.error(e instanceof HttpError ? e.message : 'Demande impossible') })}
          >
            <HugeiconsIcon icon={RefreshIcon} size={14} strokeWidth={1.8} className={pending ? 'animate-spin' : undefined} />
            {pending ? 'Synchronisation en cours…' : 'Synchroniser maintenant'}
          </Button>
        </div>
      )}
    </Card>
  )
}
