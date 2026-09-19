import { CheckmarkCircle02Icon, MoreHorizontalIcon, RefreshIcon, Upload01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

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
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { useDecideDeliverable, useSubmitVersion } from '@/features/deliverables/api'
import { HttpError } from '@/lib/api'
import type { Deliverable } from '@/types/api'

/**
 * Actions d'une ligne de livrable.
 *
 * Trancher et soumettre s'excluent dans le temps : on ne repond qu'a ce qui
 * attend, et on ne renvoie une version qu'apres des retours. Le menu n'offre
 * donc que ce que l'etat courant autorise, plutot que de proposer quatre
 * entrees dont deux repondraient par une erreur.
 *
 * L'agence tranche depuis le back-office parce que la reponse arrive souvent
 * ailleurs — au telephone, dans un fil de mail. Le portail fera le meme appel
 * au nom du client.
 */
export function DeliverableRowActions({ item }: { item: Deliverable }) {
  const [decision, setDecision] = useState<'valide' | 'retours' | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const attend = item.status === 'en_attente'
  const relancable = item.status === 'retours'

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="icon-sm" aria-label={`Actions sur ${item.title}`}>
            <HugeiconsIcon icon={MoreHorizontalIcon} size={16} strokeWidth={1.8} />
          </Button>
        </DropdownMenuTrigger>

        <DropdownMenuContent align="end" className="w-56">
          {attend && (
            <>
              <DropdownMenuItem onSelect={() => setDecision('valide')}>
                <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} strokeWidth={1.6} />
                Marquer validé
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setDecision('retours')}>
                <HugeiconsIcon icon={RefreshIcon} size={16} strokeWidth={1.6} />
                Enregistrer des retours
              </DropdownMenuItem>
              <DropdownMenuSeparator />
            </>
          )}

          <DropdownMenuItem onSelect={() => setSubmitting(true)}>
            <HugeiconsIcon icon={Upload01Icon} size={16} strokeWidth={1.6} />
            {relancable ? 'Renvoyer une version' : 'Soumettre une version'}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <DecisionDialog
        item={item}
        decision={decision}
        onClose={() => setDecision(null)}
      />
      <SubmitDialog item={item} open={submitting} onClose={() => setSubmitting(false)} />
    </>
  )
}

const decisionSchema = z.object({ feedback: z.string() })

/**
 * Ce que le client a repondu.
 *
 * Le commentaire est facultatif pour une validation — « c'est bon » n'a pas
 * besoin d'etre justifie — mais requis pour des retours : des retours sans
 * leur contenu ne disent pas quoi corriger.
 */
function DecisionDialog({
  item,
  decision,
  onClose,
}: {
  item: Deliverable
  decision: 'valide' | 'retours' | null
  onClose: () => void
}) {
  const decide = useDecideDeliverable(item.id)
  const form = useForm<z.infer<typeof decisionSchema>>({
    resolver: zodResolver(decisionSchema),
    defaultValues: { feedback: '' },
  })

  const retours = decision === 'retours'

  function onSubmit(values: z.infer<typeof decisionSchema>) {
    const feedback = values.feedback.trim()

    if (retours && feedback === '') {
      form.setError('feedback', { message: 'Dites ce qui est à corriger' })

      return
    }

    decide.mutate(
      { decision: decision === 'retours' ? 'retours' : 'valide', feedback },
      {
        onSuccess: () => {
          toast.success(retours ? 'Retours enregistrés' : `« ${item.title} » validé`)
          onClose()
          form.reset()
        },
        onError: (error) =>
          toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
      },
    )
  }

  return (
    <Dialog
      open={decision !== null}
      onOpenChange={(next) => {
        if (!next) onClose()
        form.reset()
      }}
    >
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{retours ? 'Enregistrer des retours' : 'Marquer validé'}</DialogTitle>
          <DialogDescription>
            {item.title}
            {item.version !== null && ` · v${item.version.numero}`}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form className="flex flex-col gap-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
            <FormField
              control={form.control}
              name="feedback"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{retours ? 'Ce qui est à corriger' : 'Commentaire'}</FormLabel>
                  <FormControl>
                    <Textarea
                      autoFocus
                      rows={4}
                      placeholder={retours ? 'Le logo est trop petit…' : 'Facultatif'}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    Ce que le client a répondu, tel qu’il l’a dit. C’est la trace qui restera.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type="button" size="lg" variant="outline" onClick={onClose}>
                Annuler
              </Button>
              <Button type="submit" size="lg" disabled={decide.isPending}>
                {decide.isPending ? 'Enregistrement…' : 'Enregistrer'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}

const versionSchema = z.object({
  url: z.string().trim().min(1, 'Le lien est requis'),
})

/** Une nouvelle version : le meme livrable, un nouveau lien. */
function SubmitDialog({
  item,
  open,
  onClose,
}: {
  item: Deliverable
  open: boolean
  onClose: () => void
}) {
  const submit = useSubmitVersion(item.id)
  const form = useForm<z.infer<typeof versionSchema>>({
    resolver: zodResolver(versionSchema),
    defaultValues: { url: '' },
  })

  const suivante = item.version === null ? 1 : item.version.numero + 1

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onClose()
        form.reset()
      }}
    >
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>Soumettre la v{suivante}</DialogTitle>
          <DialogDescription>
            {item.title} · elle repart en attente chez {item.client.name}.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form
            className="flex flex-col gap-4"
            onSubmit={form.handleSubmit((values) =>
              submit.mutate(values.url.trim(), {
                onSuccess: () => {
                  toast.success(`v${suivante} soumise`)
                  onClose()
                  form.reset()
                },
                onError: (error) =>
                  toast.error(error instanceof HttpError ? error.message : 'Dépôt impossible'),
              }),
            )}
            noValidate
          >
            <FormField
              control={form.control}
              name="url"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Lien</FormLabel>
                  <FormControl>
                    <Input autoFocus placeholder="https://preprod.exemple.fr" {...field} />
                  </FormControl>
                  <FormDescription>
                    La préproduction, la maquette ou le document à faire valider.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type="button" size="lg" variant="outline" onClick={onClose}>
                Annuler
              </Button>
              <Button type="submit" size="lg" disabled={submit.isPending}>
                {submit.isPending ? 'Envoi…' : 'Soumettre'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
