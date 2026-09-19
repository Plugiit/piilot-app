import { LinkSquare02Icon, TextAlignLeftIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { PageFrame } from '@/components/layout/page-frame'
import { sidebarAppListQuery } from '@/features/sidebar-apps/api'
import { AppMark, AppRowActions, NewAppDialog } from '@/features/sidebar-apps/dialogs'
import { HttpError } from '@/lib/api'

/**
 * Apps du rail.
 *
 * Les raccourcis de la barre laterale etaient ecrits en dur dans le front :
 * ajouter un outil demandait de toucher au code. Ils se tiennent ici.
 *
 * Pas de recherche ni de pagination : la liste est bornee a ce qui tient dans
 * un rail, et au-dela ce n'est plus un rail.
 */
export const Route = createFileRoute('/_app/parametres/apps/')({
  loader: ({ context }) =>
    context.queryClient.query({ ...sidebarAppListQuery(), staleTime: 'static' }),
  component: AppsPage,
})

function AppsPage() {
  const { data, isError, error } = useQuery(sidebarAppListQuery())
  const items = data?.items ?? []

  return (
    <PageFrame title="Apps du rail">
      <div className="flex min-h-full flex-col">
        <div className="flex flex-wrap items-center justify-between gap-3 p-4">
          <p className="text-[13px] text-[#73757c]">
            Les raccourcis affichés en bas de la barre latérale, dans leur ordre. Le logo est
            récupéré depuis le site quelques instants après l’ajout ; déposez le vôtre pour le
            remplacer.
          </p>

          <NewAppDialog />
        </div>

        {isError ? (
          <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
            {error instanceof HttpError ? error.message : 'Chargement impossible'}
          </p>
        ) : (
          <div className="flex min-h-0 flex-1 flex-col overflow-x-auto">
            <div className="flex min-w-max flex-col">
              <div className="flex items-stretch">
                <Cell width="w-[64px] shrink-0" head>
                  Logo
                </Cell>
                <Cell width="min-w-[200px] flex-[1_1_200px]" head icon={TextAlignLeftIcon}>
                  Nom
                </Cell>
                <Cell width="min-w-[280px] flex-[2_1_280px]" head icon={LinkSquare02Icon}>
                  Lien
                </Cell>
                <Cell width="w-[56px] shrink-0 justify-end" head>
                  {''}
                </Cell>
              </div>

              {items.length === 0 && (
                <p className="border-b border-[#e8e8e9] px-2.5 py-8 text-center text-[14px] text-[#73757c]">
                  Aucune app. Ajoutez la première depuis « Ajouter une app ».
                </p>
              )}

              {items.map((app) => (
                <div key={app.id} className="flex items-stretch bg-white">
                  <Cell width="w-[64px] shrink-0">
                    <AppMark app={app} />
                  </Cell>

                  <Cell width="min-w-[200px] flex-[1_1_200px]">
                    <span className="min-w-0 truncate text-[14px] text-[#1b1b1b]">{app.name}</span>

                    {/* Dit d'ou vient le logo : un favicon recupere peut etre
                        remplace par une nouvelle tentative, un fichier depose
                        ne l'est jamais. */}
                    {app.logo_is_favicon && (
                      <span
                        title="Logo récupéré depuis le site"
                        className="shrink-0 rounded-full bg-[#f3f4f4] px-1.5 text-[11px] text-[#73757c]"
                      >
                        auto
                      </span>
                    )}
                  </Cell>

                  <Cell width="min-w-[280px] flex-[2_1_280px]">
                    <a
                      href={app.url}
                      target="_blank"
                      rel="noreferrer"
                      title={app.url}
                      className="truncate text-[14px] text-[#4770e4] underline underline-offset-2"
                    >
                      {app.url}
                    </a>
                  </Cell>

                  <Cell width="w-[56px] shrink-0 justify-end">
                    <AppRowActions app={app} />
                  </Cell>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </PageFrame>
  )
}

/** Une cellule : meme gabarit dans l'en-tete et dans les rangees. */
function Cell({
  width,
  head = false,
  icon,
  children,
}: {
  width: string
  head?: boolean
  icon?: Parameters<typeof HugeiconsIcon>[0]['icon']
  children: React.ReactNode
}) {
  return (
    <div
      className={`flex items-center gap-2 border-b border-[#e8e8e9] px-2.5 py-3 ${width} ${
        head ? 'bg-[#f3f4f4]' : ''
      }`}
    >
      {icon !== undefined && (
        <HugeiconsIcon icon={icon} size={20} strokeWidth={1.6} className="shrink-0 text-[#73757c]" />
      )}
      {head ? <span className="text-[14px] text-[#73757c]">{children}</span> : children}
    </div>
  )
}
