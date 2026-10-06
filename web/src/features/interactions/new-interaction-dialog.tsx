import { ArrowLeft01Icon, Building03Icon, PlusSignIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'framer-motion'
import { useDeferredValue, useState } from 'react'

import { AutoHeight } from '@/components/auto-height'
import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { InteractionForm } from '@/features/interactions/journal'
import { clientListQuery } from '@/features/projects/api'


interface ChosenClient {
  id: string
  name: string
}

/**
 * Ajout d'une interaction depuis l'ecran du CRM.
 *
 * En deux temps dans la meme fenetre : d'abord le client, cherche par son nom,
 * puis ce qui s'est dit. Le choix du client ne reste pas affiche en champ une
 * fois fait — il devient le sous-titre, avec de quoi revenir en arriere — pour
 * que le formulaire ne montre que ce qu'il reste a remplir.
 *
 * Volontairement a l'ecart des filtres : filtrer le journal et y ecrire sont
 * deux gestes, et un formulaire qui apparaissait selon un filtre les melait.
 */
export function NewInteractionDialog() {
  const [open, setOpen] = useState(false)
  const [client, setClient] = useState<ChosenClient | null>(null)

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) setClient(null)
      }}
    >
      <DialogTrigger asChild>
        <Button size="lg" className="gap-1.5">
          <HugeiconsIcon icon={PlusSignIcon} size={16} strokeWidth={2} />
          Ajouter une interaction
        </Button>
      </DialogTrigger>

      <DialogContent className="overflow-hidden sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>Ajouter une interaction</DialogTitle>
          <DialogDescription asChild>
            {client === null ? (
              <p>Avec quel client ?</p>
            ) : (
              <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                <span className="flex items-center gap-1.5 text-[#1b1b1b]">
                  <HugeiconsIcon icon={Building03Icon} size={14} strokeWidth={1.8} className="text-[#73757c]" />
                  {client.name}
                </span>
                <button
                  type="button"
                  onClick={() => setClient(null)}
                  className="flex cursor-pointer items-center gap-0.5 text-[13px] text-[#73757c] underline-offset-2 hover:text-[#1b1b1b] hover:underline"
                >
                  <HugeiconsIcon icon={ArrowLeft01Icon} size={12} strokeWidth={2} />
                  Changer de client
                </button>
              </div>
            )}
          </DialogDescription>
        </DialogHeader>

        <AutoHeight>
          <AnimatePresence mode="popLayout" initial={false}>
            <motion.div
              key={client === null ? 'client' : `form-${client.id}`}
              initial={{ opacity: 0, x: client === null ? -16 : 16 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: client === null ? 16 : -16 }}
              transition={{ duration: 0.18 }}
            >
              {client === null ? (
                <ClientPicker onPick={setClient} />
              ) : (
                <InteractionForm
                  clientId={client.id}
                  autoFocus
                  onSaved={() => {
                    setOpen(false)
                    setClient(null)
                  }}
                />
              )}
            </motion.div>
          </AnimatePresence>
        </AutoHeight>
      </DialogContent>
    </Dialog>
  )
}

/** Choix du client, avec recherche sur le serveur. */
function ClientPicker({ onPick }: { onPick: (client: ChosenClient) => void }) {
  const [search, setSearch] = useState('')
  const deferred = useDeferredValue(search.trim())
  const { data, isPending } = useQuery(clientListQuery(deferred === '' ? undefined : deferred))
  const clients = data?.items ?? []

  return (
    <Command shouldFilter={false} className="rounded-[10px] border border-[#e8e8e9]">
      <CommandInput autoFocus value={search} onValueChange={setSearch} placeholder="Rechercher un client" />
      <CommandList className="max-h-[280px]">
        {!isPending && <CommandEmpty>Aucun client ne correspond.</CommandEmpty>}
        <CommandGroup>
          {clients.map((client) => (
            <CommandItem
              key={client.id}
              value={client.id}
              onSelect={() => onPick({ id: client.id, name: client.name })}
              className="gap-2"
            >
              <HugeiconsIcon icon={Building03Icon} size={15} strokeWidth={1.6} className="shrink-0 text-[#8d8d8d]" />
              <span className="flex-1 truncate">{client.name}</span>
              {client.contact_name !== '' && (
                <span className="truncate text-[12px] text-[#a2a3a7]">{client.contact_name}</span>
              )}
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </Command>
  )
}
