import { Search01Icon, Tag01Icon, TextAlignLeftIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { z } from 'zod'

import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { SERVICES_PAGE_SIZE, serviceListQuery } from '@/features/services/api'
import { NewServiceDialog, ServiceRowActions } from '@/features/services/dialogs'
import { HttpError } from '@/lib/api'
import { useSearchField } from '@/lib/search-field'
import { cn } from '@/lib/utils'
import type { Service } from '@/types/api'

/**
 * Referentiel des services.
 *
 * Une nomenclature qu'on tient a jour : les prestations que l'agence vend. Pas
 * de vue kanban ni de filtres — une liste alphabetique et quatre verbes sont
 * tout ce qu'un referentiel demande.
 *
 * La recherche vit dans l'adresse comme ailleurs, pour que la vue se partage
 * et que le bouton Retour y ramene.
 */
const searchSchema = z.object({
  page: z.number().int().min(1).catch(1),
  search: z.string().optional(),
})

export const Route = createFileRoute('/_app/parametres/services/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ search: search.search ?? '', page: search.page }),
  loader: ({ context, deps }) =>
    context.queryClient.query({
      ...serviceListQuery(deps.search, deps.page),
      staleTime: 'static',
    }),
  component: ServicesPage,
})

interface Column {
  key: string
  label: string
  icon?: IconSvgElement
  width: string
  cell: (service: Service) => ReactNode
}

const COLUMNS: Column[] = [
  {
    key: 'name',
    label: 'Service',
    icon: Tag01Icon,
    width: 'min-w-[220px] flex-[1_1_220px]',
    // La teinte precede le nom : c'est elle qui distingue une ligne de la
    // suivante quand on parcourt la liste du regard.
    cell: (service) => (
      <>
        <span
          aria-hidden
          className="size-2.5 shrink-0 rounded-full"
          style={{ backgroundColor: service.color }}
        />
        <span className="truncate text-[14px] text-[#1b1b1b]">{service.name}</span>
      </>
    ),
  },
  {
    key: 'description',
    label: 'Description',
    icon: TextAlignLeftIcon,
    width: 'min-w-[280px] flex-[2_1_280px]',
    cell: (service) =>
      service.description === '' ? (
        <span className="text-[14px] text-[#a2a3a7]">—</span>
      ) : (
        <span className="truncate text-[14px] text-[#73757c]" title={service.description}>
          {service.description}
        </span>
      ),
  },
  {
    key: 'actions',
    label: '',
    width: 'w-[56px] shrink-0 justify-end',
    cell: (service) => <ServiceRowActions service={service} />,
  },
]

/** Une cellule : meme gabarit dans l'en-tete et dans les rangees. */
function Cell({
  width,
  className,
  children,
}: {
  width: string
  className?: string
  children: ReactNode
}) {
  return (
    <div
      className={cn(
        'flex items-center gap-2 border-b border-[#e8e8e9] px-2.5 py-3',
        width,
        className,
      )}
    >
      {children}
    </div>
  )
}

function ServicesPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data, isError, error } = useQuery(serviceListQuery(search.search ?? '', search.page))

  const [draft, setDraft] = useSearchField(search.search ?? '', (value) => {
    // Toute recherche ramene page 1 : rester en page 3 d'une liste qui vient
    // de se reduire n'afficherait rien.
    void navigate({
      search: (prev) => ({ ...prev, search: value === '' ? undefined : value, page: 1 }),
      replace: true,
    })
  })

  const items = data?.items ?? []
  const total = data?.total ?? 0
  const current = data?.page ?? search.page
  const totalPages = Math.max(1, Math.ceil(total / SERVICES_PAGE_SIZE))

  return (
    <PageFrame title="Services">
      <div className="flex min-h-full flex-col">
        <div className="flex flex-wrap items-center gap-2 p-4">
          <div className="relative min-w-[200px] flex-1">
            <HugeiconsIcon
              icon={Search01Icon}
              size={16}
              strokeWidth={1.6}
              className="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-[#8d8d8d]"
            />
            <Input
              type="search"
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder="Rechercher un service"
              aria-label="Rechercher un service"
              className="h-9 pl-8 text-[13px]"
            />
          </div>

          <NewServiceDialog />
        </div>

        {isError ? (
          <p className="m-4 rounded-[12px] border border-[#f2d5d6] bg-[#fdf3f3] p-4 text-[13px] text-[#e5484d]">
            {error instanceof HttpError ? error.message : 'Chargement impossible'}
          </p>
        ) : (
          <>
            <div className="flex min-h-0 flex-1 flex-col overflow-x-auto">
              <div className="flex min-w-max flex-col">
                <div className="flex items-stretch">
                  {COLUMNS.map((column) => (
                    <Cell key={column.key} width={column.width} className="bg-[#f3f4f4]">
                      {column.icon !== undefined && (
                        <HugeiconsIcon
                          icon={column.icon}
                          size={20}
                          strokeWidth={1.6}
                          className="shrink-0 text-[#73757c]"
                        />
                      )}
                      <span className="text-[14px] text-[#73757c]">{column.label}</span>
                    </Cell>
                  ))}
                </div>

                {items.length === 0 && (
                  <p className="border-b border-[#e8e8e9] px-2.5 py-8 text-center text-[14px] text-[#73757c]">
                    {search.search === undefined || search.search === ''
                      ? 'Aucun service. Ajoutez le premier depuis « Ajouter un service ».'
                      : 'Aucun service ne correspond à cette recherche.'}
                  </p>
                )}

                {items.map((service) => (
                  <div key={service.id} className="flex items-stretch bg-white">
                    {COLUMNS.map((column) => (
                      <Cell key={column.key} width={column.width}>
                        {column.cell(service)}
                      </Cell>
                    ))}
                  </div>
                ))}
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-2 p-4">
              <p className="text-[12px] text-[#777]">
                {total} service{total > 1 ? 's' : ''} · page {current} sur {totalPages}
              </p>

              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={current <= 1}
                  onClick={() =>
                    void navigate({ search: (prev) => ({ ...prev, page: current - 1 }) })
                  }
                >
                  Précédent
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={current >= totalPages}
                  onClick={() =>
                    void navigate({ search: (prev) => ({ ...prev, page: current + 1 }) })
                  }
                >
                  Suivant
                </Button>
              </div>
            </div>
          </>
        )}
      </div>
    </PageFrame>
  )
}
