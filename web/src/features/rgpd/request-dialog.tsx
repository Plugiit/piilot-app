import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
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
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { useRequestErasure, settingsQuery } from './api'
import { reportError, useExportAction, type RgpdSubject } from './actions'
import { confirmMatches, formatDay, graceLabel, plannedDate, type SubjectType } from './labels'

/** Ce que l'effacement fait, dans les mots du sujet. */
function Consequences({ type }: { type: SubjectType }) {
  return type === 'client' ? (
    <ul className="list-disc space-y-1 pl-5 text-[13px] text-[#4b4b4f]">
      <li>
        <strong className="font-medium text-[#1b1b1b]">Supprimés</strong> : le client, ses contacts, ses projets avec
        leurs tâches, tickets, livrables et rapports, le temps saisi sur ces projets, ses e-mails et tous ses fichiers.
      </li>
      <li>
        <strong className="font-medium text-[#1b1b1b]">Anonymisés</strong> : ses comptes du portail, désactivés dès
        maintenant.
      </li>
      <li>
        <strong className="font-medium text-[#1b1b1b]">Conservés</strong> : le journal d’audit (jusqu’à son expiration)
        et la trace de la demande.
      </li>
    </ul>
  ) : (
    <ul className="list-disc space-y-1 pl-5 text-[13px] text-[#4b4b4f]">
      <li>
        <strong className="font-medium text-[#1b1b1b]">Anonymisés</strong> : nom, adresse e-mail, téléphone, avatar,
        connexions, notifications et appartenances aux projets. Le compte est désactivé dès maintenant.
      </li>
      <li>
        <strong className="font-medium text-[#1b1b1b]">Conservés</strong> : le temps saisi (sans lien avec une
        personne), le texte libre de l’équipe et le journal d’audit.
      </li>
    </ul>
  )
}

/**
 * Demande d'effacement définitif.
 *
 * Dit ce qui part, quand, et laisse le temps de se raviser : l'effacement n'a
 * lieu qu'à la fin du délai de rétractation, et peut être annulé jusque-là.
 * Le bouton ne s'active qu'une fois le nom du sujet retapé.
 */
export function ErasureDialog({
  subject,
  open,
  onOpenChange,
}: {
  subject: RgpdSubject
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const [typed, setTyped] = useState('')
  const { data: settings } = useQuery({ ...settingsQuery, enabled: open })
  const request = useRequestErasure()
  const exporter = useExportAction(subject)

  const inputId = `rgpd-confirm-${subject.id}`
  const matches = confirmMatches(typed, subject.name)

  function close(next: boolean) {
    if (!next) setTyped('')
    onOpenChange(next)
  }

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Supprimer définitivement {subject.name} ?</DialogTitle>
          <DialogDescription>
            {settings === undefined
              ? 'Calcul de la date d’effacement…'
              : `Effacement prévu le ${formatDay(plannedDate(new Date(), settings.erasure_grace_hours))}, soit ${graceLabel(settings.erasure_grace_hours)} après la demande. Jusque-là, vous pouvez l’annuler.`}
          </DialogDescription>
        </DialogHeader>

        <Consequences type={subject.type} />

        <p className="rounded-[10px] bg-[#f3f4f4] px-3 py-2.5 text-[13px] text-[#4b4b4f]">
          L’effacement est irréversible une fois exécuté.{' '}
          {subject.type === 'client' ? 'Proposez d’abord son export à la personne concernée. ' : 'Pensez à exporter ses données d’abord. '}
          <Button type="button" variant="link" size="sm" className="h-auto p-0" disabled={exporter.pending} onClick={exporter.run}>
            Exporter les données
          </Button>
          {' · '}
          <Link to="/parametres/rgpd" search={{ tab: 'exports', page: 1 }} className="underline">
            Voir les exports
          </Link>
        </p>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor={inputId}>
            Pour confirmer, retapez <strong className="font-medium">{subject.name}</strong>
          </Label>
          <Input
            id={inputId}
            value={typed}
            autoComplete="off"
            spellCheck={false}
            onChange={(event) => setTyped(event.target.value)}
          />
        </div>

        <DialogFooter>
          <Button variant="ghost" size="lg" onClick={() => close(false)}>
            Annuler
          </Button>
          <Button
            variant="destructive"
            size="lg"
            disabled={!matches || request.isPending || settings === undefined}
            onClick={() =>
              request.mutate(subject, {
                onSuccess: (item) => {
                  close(false)
                  toast.success(`Suppression planifiée le ${formatDay(item.scheduled_for)}`)
                },
                onError: (e) => reportError(e, 'Demande impossible'),
              })
            }
          >
            Planifier la suppression
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
