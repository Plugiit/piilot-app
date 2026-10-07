import { GitMergeIcon, GitPullRequestIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'

import { cn } from '@/lib/utils'
import type { PullRequest } from '@/types/api'

const STATE: Record<PullRequest['state'], { label: string; className: string }> = {
  open: { label: 'ouverte', className: 'border-[#d5dafd] bg-[#eef0fe] text-[#3b45c9]' },
  merged: { label: 'fusionnée', className: 'border-[#afe3ca] bg-[#dcf7ea] text-[#006f1f]' },
  closed: { label: 'fermée', className: 'border-[#e8e8e9] bg-[#f3f4f4] text-[#73757c]' },
}

/**
 * Les pull requests d'un ticket ou d'une tache : le numero, l'etat, et le
 * lien vers GitHub ou GitLab. Rien a faire ici — elles arrivent par webhook.
 */
export function PullRequestChips({ items, className }: { items: PullRequest[]; className?: string }) {
  if (items.length === 0) return null

  return (
    <ul className={cn('flex flex-wrap gap-1.5', className)}>
      {items.map((pr) => {
        const state = STATE[pr.state]
        const prefix = pr.provider === 'gitlab' ? '!' : '#'
        return (
          <li key={pr.id}>
            <a
              href={pr.url}
              target="_blank"
              rel="noreferrer"
              title={pr.title}
              className={cn(
                'flex max-w-[260px] items-center gap-1.5 rounded-full border px-2 py-0.5 text-[12px] hover:underline',
                state.className,
              )}
            >
              <HugeiconsIcon icon={pr.state === 'merged' ? GitMergeIcon : GitPullRequestIcon} size={13} strokeWidth={1.8} className="shrink-0" />
              <span className="shrink-0 tabular-nums">
                {prefix}
                {pr.number}
              </span>
              <span className="min-w-0 truncate">{pr.title}</span>
              <span className="shrink-0 opacity-75">· {state.label}</span>
            </a>
          </li>
        )
      })}
    </ul>
  )
}
