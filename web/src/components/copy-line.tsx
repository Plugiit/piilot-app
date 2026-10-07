import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'

/** Une valeur a copier : lisible, selectionnable, et un bouton pour la prendre d'un coup. */
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
      <code className="min-w-0 flex-1 truncate rounded-[8px] border border-[#e8e8e9] bg-[#fafafa] px-3 py-2 font-mono text-[13px] text-[#1b1b1b] select-all">
        {shown ? value : '•'.repeat(32)}
      </code>
      {secret && (
        <Button type="button" variant="outline" size="sm" onClick={() => setShown((v) => !v)}>
          {shown ? 'Masquer' : 'Afficher'}
        </Button>
      )}
      <Button type="button" variant="outline" size="sm" onClick={() => void copy()}>
        {copied ? 'Copié' : 'Copier'}
      </Button>
    </div>
  )
}
