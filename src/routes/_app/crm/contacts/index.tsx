import { Search01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  CONTACTS_PAGE_SIZE,
  contactListQuery,
  type CrmContactParams,
} from '@/features/contacts/api'
import { ContactTable } from '@/features/contacts/list'
import { NewContactDialog } from '@/features/contacts/new-contact-dialog'
import { clientListQuery } from '@/features/projects/api'

/**
 * Ecran « Contacts » du CRM.
 *
 * Meme gabarit que l'ecran des clients, en vue tableau seule. Les filtres
 * vivent dans l'adresse : « les contacts de Maison Aubert » est une vue qu'on
 * partage et ou le bouton Retour ramene.
 */
const searchSchema = z.object({
  search: z.string().optional(),
  // « libres » n'est pas un identifiant de client : c'est l'absence de client,
  // que le serveur traite par un filtre a part.
  client: z.string().optional(),
  // `default` et non le seul `catch` : la page se rejoint sans parametres.
  page: z.number().int().min(1).default(1).catch(1),
})

type Search = z.infer<typeof searchSchema>

/** Ce que la barre d'outils envoie au serveur, deduit de l'adresse. */
function paramsOf(search: Search): CrmContactParams {
  return {
    search: search.search?.trim() === '' ? undefined : search.search,
    clientId: search.client === FREE ? undefined : search.client,
    onlyFree: search.client === FREE,
    page: search.page,
  }
}

/** Valeur reservee du filtre client : les contacts qui n'en ont aucun. */
const FREE = 'libres'

export const Route = createFileRoute('/_app/crm/contacts/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => paramsOf(search),
  loader: ({ context, deps }) =>
    context.queryClient.query({ ...contactListQuery(deps), staleTime: 'static' }),
  component: ContactsPage,
})

function ContactsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data: page } = useQuery(contactListQuery(paramsOf(search)))
  // Cent clients suffisent a remplir le filtre : au-dela, c'est la recherche
  // qui sert, pas un menu qu'on deroule.
  const { data: clients } = useQuery(clientListQuery())

  const contacts = page?.items ?? []
  const total = page?.total ?? 0
  const current = page?.page ?? 1
  const totalPages = Math.max(1, Math.ceil(total / CONTACTS_PAGE_SIZE))

  const clientOptions: Option[] = [
    { value: FREE, label: 'Sans client' },
    ...(clients?.items ?? []).map((client) => ({ value: client.id, label: client.name })),
  ]

  // Tout changement de filtre ramene page 1 : rester en page 3 d'une liste qui
  // vient de se reduire n'afficherait rien.
  function setFilter(patch: Partial<Omit<Search, 'page'>>) {
    void navigate({ search: (prev) => ({ ...prev, ...patch, page: 1 }) })
  }

  const filtered = (search.search ?? '') !== '' || search.client !== undefined

  return (
    <PageFrame title="Contacts">
      <div className="flex min-h-full flex-col">
        <div className="flex flex-col gap-3 p-4">
          {/* Barre d'outils : recherche, puis le filtre, puis l'action.
              L'ordre suit la lecture — on reduit la liste de gauche a droite,
              et ce qui l'augmente est au bout. */}
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative min-w-[200px] flex-1">
              <HugeiconsIcon
                icon={Search01Icon}
                size={16}
                strokeWidth={1.6}
                className="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-[#8d8d8d]"
              />
              <Input
                type="search"
                value={search.search ?? ''}
                onChange={(event) => setFilter({ search: event.target.value || undefined })}
                placeholder="Rechercher un contact, un e-mail ou un client"
                aria-label="Rechercher un contact"
                className="h-9 pl-8 text-[13px]"
              />
            </div>

            <FilterMenu
              name="Client"
              all="Tous les clients"
              value={search.client}
              options={clientOptions}
              onChange={(value) => setFilter({ client: value })}
            />

            <NewContactDialog />
          </div>
        </div>

        <ContactTable
          contacts={contacts}
          empty={filtered ? 'Aucun contact ne correspond.' : 'Aucun contact pour le moment.'}
        />

        <div className="flex flex-wrap items-center justify-between gap-2 p-4">
          <p className="text-[12px] text-[#777]">
            {total} contact{total > 1 ? 's' : ''} · page {current} sur {totalPages}
          </p>

          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={current <= 1}
              onClick={() => void navigate({ search: (prev) => ({ ...prev, page: current - 1 }) })}
            >
              Précédent
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={current >= totalPages}
              onClick={() => void navigate({ search: (prev) => ({ ...prev, page: current + 1 }) })}
            >
              Suivant
            </Button>
          </div>
        </div>
      </div>
    </PageFrame>
  )
}
