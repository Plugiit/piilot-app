import { LinkSquare02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { createFileRoute } from '@tanstack/react-router'

import calendarUrl from '@/assets/integrations/google-calendar.svg'
import { PageFrame } from '@/components/layout/page-frame'

/**
 * Planning.
 *
 * L'ecran attend son modele de donnees. Plutot qu'un marque-place qui dit
 * seulement « pas encore developpe », la page envoie la ou le planning se tient
 * aujourd'hui : l'agenda partage de l'agence. C'est la meme carte que la page
 * des integrations, ou Google Agenda figure parmi les outils a brancher.
 */
export const Route = createFileRoute('/_app/pm/planning/')({
  component: PlanningPage,
})

function PlanningPage() {
  return (
    <PageFrame title="Planning">
      <div className="flex h-full items-center justify-center p-6">
        <article className="flex w-full max-w-[340px] flex-col gap-3 rounded-[12px] border border-[#e8e8e9] bg-white p-3">
          {/* La boite fait 48px et le logo s'y inscrit sans se deformer, comme
              sur la grille des integrations. */}
          <span className="flex size-12 shrink-0 items-center justify-center">
            <img src={calendarUrl} alt="" className="size-full object-contain" />
          </span>

          <div className="flex flex-col gap-1">
            <h2 className="text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">Google Agenda</h2>
            <p className="text-[12px] leading-[1.5] text-[#73757c]">
              Le planning n’est pas encore dans l’outil. En attendant, les jalons et les échéances
              des projets vivent dans l’agenda partagé de l’agence.
            </p>
          </div>

          <a
            href="https://calendar.google.com"
            target="_blank"
            rel="noreferrer"
            className="relative flex h-10 items-center justify-center gap-2 rounded-[12px] border border-[#e8e8e9] bg-white text-[14px] font-medium text-[#1b1b1b] shadow-[inset_0px_-2px_0px_0px_rgba(0,0,0,0.05)] transition-colors hover:bg-[#f8f8f8]"
          >
            Ouvrir l’agenda
            <HugeiconsIcon icon={LinkSquare02Icon} size={16} strokeWidth={1.8} />
          </a>
        </article>
      </div>
    </PageFrame>
  )
}
