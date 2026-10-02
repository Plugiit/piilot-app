import { Copy01Icon, Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'

const DATE_TIME = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'long',
  hour: '2-digit',
  minute: '2-digit',
})

/**
 * Lien d'invitation ou de reinitialisation, pret a copier.
 *
 * Il n'est montre qu'une fois : le serveur n'en garde que l'empreinte, et ne
 * pourra plus le rendre. D'ou le rappel sous le champ, et un bouton de copie
 * plutot qu'une selection a la souris dans un champ trop etroit pour l'URL.
 */
export function LinkBox({ link, expiresAt }: { link: string; expiresAt: string }) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(link)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      // Presse-papiers refuse (page hors HTTPS, permission) : le lien reste
      // selectionnable dans le champ.
      toast.error('Copie impossible : sélectionnez le lien et copiez-le à la main')
    }
  }

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex items-center gap-2">
        <input
          readOnly
          value={link}
          aria-label="Lien"
          onFocus={(event) => event.currentTarget.select()}
          className="h-9 min-w-0 flex-1 truncate rounded-[10px] border border-[#e8e8e9] bg-[#f8f8f8] px-3 text-[13px] text-[#4b4b4f]"
        />
        <Button type="button" variant="outline" size="sm" onClick={() => void copy()} className="gap-1.5">
          <HugeiconsIcon icon={copied ? Tick02Icon : Copy01Icon} size={16} strokeWidth={1.8} />
          {copied ? 'Copié' : 'Copier'}
        </Button>
      </div>
      <p className="text-[12px] text-[#73757c]">
        Valable jusqu’au {DATE_TIME.format(new Date(expiresAt))}, pour un seul usage. Il ne sera
        plus affiché ensuite.
      </p>
    </div>
  )
}
