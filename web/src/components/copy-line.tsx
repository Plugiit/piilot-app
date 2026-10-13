import { Copy01Icon, Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'

/**
 * Une valeur a copier : lisible, selectionnable, et une icone dans le champ
 * pour la prendre d'un coup. Un secret se masque ; son bouton « Afficher »
 * a la hauteur du champ.
 */
export function CopyLine({ value, secret = false }: { value: string; secret?: boolean }) {
  const [shown, setShown] = useState(!secret)
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error('Copie impossible : sélectionnez la valeur à la main.')
    }
  }

  return (
    <div className="flex min-w-0 flex-1 items-center gap-2">
      <div className="relative flex h-9 min-w-0 flex-1 items-center rounded-[8px] border border-[#e8e8e9] bg-[#fafafa]">
        <code className="min-w-0 flex-1 truncate py-2 pr-1 pl-3 font-mono text-[13px] text-[#1b1b1b] select-all">
          {shown ? value : '•'.repeat(32)}
        </code>
        <button
          type="button"
          onClick={() => void copy()}
          aria-label={copied ? 'Copié' : 'Copier'}
          title={copied ? 'Copié' : 'Copier'}
          className="mr-1 flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-[6px] text-[#73757c] transition-colors hover:bg-[#ececed] hover:text-[#1b1b1b]"
        >
          <HugeiconsIcon icon={copied ? Tick02Icon : Copy01Icon} size={16} strokeWidth={1.8} className={copied ? 'text-[#006f1f]' : undefined} />
        </button>
      </div>
      {secret && (
        <Button type="button" variant="outline" className="h-9 px-3" onClick={() => setShown((v) => !v)}>
          {shown ? 'Masquer' : 'Afficher'}
        </Button>
      )}
    </div>
  )
}
