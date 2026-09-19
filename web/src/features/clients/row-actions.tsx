import { Delete02Icon, MoreHorizontalIcon, PencilEdit02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useDeleteClient, useUpdateClient, valuesOfClient } from '@/features/clients/api'
import { HttpError } from '@/lib/api'
import type { CrmClient } from '@/types/api'

/**
 * Actions d'une ligne de client : renommer, supprimer.
 *
 * Les deux vivent dans un menu plutot qu'en boutons visibles : une ligne de
 * tableau en compte deja assez, et supprimer n'est pas un geste qu'on veut a
 * portee de clic.
 *
 * `onDeleted` permet a la fiche de quitter la page apres suppression ; la
 * liste, elle, n'a rien a faire de plus que se rafraichir.
 */
export function ClientRowActions({
  client,
  onDeleted,
}: {
  client: CrmClient
  onDeleted?: () => void
}) {
  const [renaming, setRenaming] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [name, setName] = useState(client.name)
  const [error, setError] = useState<string | null>(null)

  const update = useUpdateClient()
  const remove = useDeleteClient()

  function rename() {
    setError(null)

    // Les autres champs repartent inchanges : la requete les ecrit tous.
    update.mutate(
      { id: client.id, values: { ...valuesOfClient(client), name } },
      {
        onSuccess: () => {
          setRenaming(false)
          toast.success('Client renommé')
        },
        onError: (err) => {
          if (err instanceof HttpError) {
            setError(typeof err.details.name === 'string' ? err.details.name : err.message)

            return
          }

          setError('Modification impossible')
        },
      },
    )
  }

  function destroy() {
    remove.mutate(client.id, {
      onSuccess: () => {
        setConfirming(false)
        toast.success(`« ${client.name} » supprimé`)
        onDeleted?.()
      },
      onError: (err) => {
        // Le refus porte une raison utile — des projets, des comptes — qui
        // merite d'etre lue en entier plutot que resumee.
        const detail =
          err instanceof HttpError ? Object.values(err.details).map(String).join(' ') : ''

        toast.error(detail !== '' ? detail : 'Suppression impossible')
        setConfirming(false)
      },
    })
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label={`Actions sur ${client.name}`}
            className="flex size-7 cursor-pointer items-center justify-center rounded-[8px] hover:bg-[#f3f4f4]"
          >
            <HugeiconsIcon
              icon={MoreHorizontalIcon}
              size={16}
              strokeWidth={1.8}
              className="text-[#73757c]"
            />
          </button>
        </DropdownMenuTrigger>

        <DropdownMenuContent align="end">
          <DropdownMenuItem
            onSelect={() => {
              setName(client.name)
              setError(null)
              setRenaming(true)
            }}
          >
            <HugeiconsIcon icon={PencilEdit02Icon} size={16} strokeWidth={1.6} />
            Renommer
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          <DropdownMenuItem variant="destructive" onSelect={() => setConfirming(true)}>
            <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.6} />
            Supprimer
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={renaming} onOpenChange={setRenaming}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Renommer le client</DialogTitle>
            <DialogDescription>
              Le nom est unique, casse ignorée. Il s'affiche partout où le client apparaît.
            </DialogDescription>
          </DialogHeader>

          <div className="flex flex-col gap-2">
            <Label htmlFor="client-name">Nom du client</Label>
            <Input
              id="client-name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') rename()
              }}
            />
            {error !== null && <p className="text-[13px] text-destructive">{error}</p>}
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setRenaming(false)}>
              Annuler
            </Button>
            <Button
              type="button"
              disabled={update.isPending || name.trim() === ''}
              onClick={rename}
            >
              {update.isPending ? 'Enregistrement…' : 'Enregistrer'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Supprimer « {client.name} » ?</DialogTitle>
            <DialogDescription>
              Ses {client.contacts_count} contact{client.contacts_count > 1 ? 's' : ''} seront
              supprimés avec lui. Un client qui porte des projets ou des comptes de portail ne peut
              pas être supprimé.
            </DialogDescription>
          </DialogHeader>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirming(false)}>
              Annuler
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={remove.isPending}
              onClick={destroy}
            >
              {remove.isPending ? 'Suppression…' : 'Supprimer'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
