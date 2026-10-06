import { ArrowUpRight01Icon, Loading03Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
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
import { updateStatusQuery, useRequestUpdate } from '@/features/system/api'
import { HttpError } from '@/lib/api'
import { sessionQuery } from '@/lib/auth'
import type { UpdateStatus } from '@/types/api'

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', { day: 'numeric', month: 'long', year: 'numeric' })

/**
 * Etat de la mise a jour, pour les seuls comptes qui portent la permission
 * `system.update` — les admins. Nul tant qu'il n'y a rien a installer ni
 * installation en cours : un rappel toujours visible deviendrait un decor
 * qu'on ne regarde plus le jour ou il compte.
 */
function usePendingUpdate(): UpdateStatus | null {
  const { data: session } = useQuery(sessionQuery)
  const allowed = session?.permissions.includes('system.update') === true

  // La requete ne part que pour un admin : pour les autres, l'API repondrait
  // 403 a chaque relevé.
  const { data: status } = useQuery({ ...updateStatusQuery, enabled: allowed })

  if (!allowed || status === undefined) return null
  if (!status.update_available && !status.in_progress) return null

  return status
}

const targetOf = (status: UpdateStatus) => status.latest?.version ?? status.last_request?.target_version ?? ''

/**
 * Carte « nouvelle version », au pied du panneau de navigation.
 *
 * C'est la seule annonce dans l'interface : au pied du panneau, elle se voit
 * depuis tous les ecrans du back-office sans prendre la place d'un contenu.
 */
export function UpdateCard() {
  const status = usePendingUpdate()
  if (status === null) return null

  const target = targetOf(status)

  return (
    <UpdateDialog status={status}>
      <div
        role="region"
        aria-label="Nouvelle version"
        className="flex w-full flex-col gap-2.5 rounded-[10px] border border-[#e4e4e4] bg-white p-3"
      >
        <div className="flex flex-col gap-0.5">
          <span className="flex items-center gap-2 text-[13px] leading-[1.4] font-medium text-[#1b1b1b]">
            {status.in_progress ? (
              <HugeiconsIcon icon={Loading03Icon} size={13} strokeWidth={2} className="animate-spin text-[#73757c]" />
            ) : (
              <span aria-hidden className="size-1.5 shrink-0 rounded-full bg-brand" />
            )}
            {status.in_progress ? `Installation de la ${target}` : `Version ${target} disponible`}
          </span>
          <span className="text-[12px] leading-[1.4] text-[#73757c]">
            {status.in_progress
              ? 'L’application redémarre dans une à deux minutes.'
              : `Version installée : ${status.current_version}`}
          </span>
        </div>

        <DialogTrigger asChild>
          <Button variant="outline" size="sm" className="w-full">
            {status.in_progress ? 'Suivre l’installation' : 'Installer'}
          </Button>
        </DialogTrigger>
      </div>
    </UpdateDialog>
  )
}

function UpdateDialog({ status, children }: { status: UpdateStatus; children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const target = targetOf(status)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {children}

      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Mettre à jour Piilot</DialogTitle>
          <DialogDescription>
            Version installée <strong className="text-[#1b1b1b]">{status.current_version}</strong>
            {target !== '' && (
              <>
                {' '}
                → <strong className="text-[#1b1b1b]">{target}</strong>
              </>
            )}
          </DialogDescription>
        </DialogHeader>

        {open && <UpdateBody status={status} onDone={() => setOpen(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function UpdateBody({ status, onDone }: { status: UpdateStatus; onDone: () => void }) {
  const request = useRequestUpdate()
  const last = status.last_request

  async function launch() {
    try {
      await request.mutateAsync()
      toast.success('Mise à jour lancée')
    } catch (error) {
      toast.error(error instanceof HttpError ? error.message : 'Lancement impossible')
    }
  }

  return (
    <div className="flex flex-col gap-4 text-[14px] text-[#1b1b1b]">
      {status.latest !== null && (
        <a
          href={status.latest.url}
          target="_blank"
          rel="noreferrer"
          className="flex items-center gap-1 self-start font-medium underline-offset-2 hover:underline"
        >
          {status.latest.name !== '' ? status.latest.name : `Version ${status.latest.version}`}
          {status.latest.published_at !== null &&
            `, publiée le ${DATE_FORMAT.format(new Date(status.latest.published_at))}`}
          <HugeiconsIcon icon={ArrowUpRight01Icon} size={14} strokeWidth={1.8} />
        </a>
      )}

      {status.in_progress && last !== null && (
        <div className="flex flex-col gap-1 rounded-[10px] bg-[#f3f4f4] p-3 text-[13px] text-[#4b4b4f]">
          <p className="font-medium text-[#1b1b1b]">
            {last.status === 'pending' ? 'Demande enregistrée' : last.step}
          </p>
          <p>
            {last.status === 'pending'
              ? 'Le service de mise à jour la prend dans quelques secondes.'
              : 'L’application redémarre sur la nouvelle version. Comptez une à deux minutes ; un bandeau proposera de recharger la page une fois la nouvelle version en ligne.'}
            {last.requested_by !== '' && ` Lancée par ${last.requested_by}.`}
          </p>
        </div>
      )}

      {!status.in_progress && last?.status === 'failed' && (
        <p className="rounded-[10px] bg-[#fdf3f3] p-3 text-[13px] text-[#c4333a]">
          La dernière tentative a échoué : {last.error}
        </p>
      )}

      {!status.in_progress && status.can_update && (
        <ul className="flex list-disc flex-col gap-1.5 pl-5 text-[13px] text-[#4b4b4f]">
          <li>L’application est indisponible quelques instants pour tout le monde pendant le redémarrage.</li>
          <li>
            Les migrations de base de la nouvelle version s’appliquent au démarrage et ne se défont
            pas. Assurez-vous d’avoir une sauvegarde récente de la base.
          </li>
          <li>Lisez les notes de version : elles signalent les changements à connaître.</li>
        </ul>
      )}

      {!status.in_progress && !status.can_update && (
        <p className="rounded-[10px] bg-[#f3f4f4] p-3 text-[13px] text-[#4b4b4f]">
          {status.updater_error !== ''
            ? `Le service de mise à jour ne peut pas travailler : ${status.updater_error}.`
            : 'Le service de mise à jour (le conteneur « updater ») ne répond pas sur cette instance.'}{' '}
          Mettez à jour depuis le serveur, ou voyez la section « Mise à jour depuis l’interface » du
          README.
        </p>
      )}

      <DialogFooter>
        <Button type="button" variant="ghost" size="lg" onClick={onDone}>
          {status.in_progress || !status.can_update ? 'Fermer' : 'Annuler'}
        </Button>
        {!status.in_progress && status.can_update && (
          <Button type="button" size="lg" disabled={request.isPending} onClick={() => void launch()}>
            {request.isPending ? 'Lancement…' : 'Mettre à jour maintenant'}
          </Button>
        )}
      </DialogFooter>
    </div>
  )
}
