import {
  Building03Icon,
  Contact01Icon,
  Folder01Icon,
  Globe02Icon,
  Mail01Icon,
  SmartPhone01Icon,
  UserMultipleIcon,
  UserStar01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { PageFrame } from '@/components/layout/page-frame'
import { PanelCard } from '@/components/panel-card'
import { Button } from '@/components/ui/button'
import { clientDetailQuery } from '@/features/clients/api'
import { EditClientDialog } from '@/features/clients/edit-dialog'
import { CLIENT_STATUS, CLIENT_STATUS_ORDER } from '@/features/clients/format'
import { PrimaryContactPicker } from '@/features/clients/primary-contact-picker'
import { ClientRowActions } from '@/features/clients/row-actions'
import { ContactRowActions } from '@/features/contacts/row-actions'
import { PROJECT_STATUS } from '@/features/projects/format'
import { Avatars, Meter, StatusPill } from '@/features/projects/ui'
import { formatPhone } from '@/lib/countries'
import type { CrmClientDetail, ProjectStatus } from '@/types/api'

const DATE_FORMAT = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

/**
 * Fiche d'un client.
 *
 * Un seul appel la remplit : l'en-tete, les contacts, les projets et les
 * comptes de portail arrivent ensemble. C'est la regle du projet — un endpoint
 * par vue —, et c'est ce qui evite quatre requetes en cascade a l'ouverture.
 *
 * L'habillage est celui du tableau de bord du module PM : `PanelCard` pour les
 * blocs — une coque grise de deux pixels, l'intitule pose dedans, la carte
 * blanche filetee ou vit le contenu — et la meme tuile que `StatTiles` pour les
 * chiffres. Rien n'est dessine ici qui n'existe deja ailleurs.
 */
export const Route = createFileRoute('/_app/crm/clients_/$id')({
  loader: ({ context, params }) =>
    context.queryClient.query({ ...clientDetailQuery(params.id), staleTime: 'static' }),
  component: ClientDetailPage,
})

/**
 * Chiffre-cle, au gabarit des tuiles du tableau de bord : coque grise, carte
 * blanche pour la valeur, pied gris pour la mention.
 *
 * Le pied est toujours rendu, meme vide : sans lui, une tuile sans mention
 * remonte d'un cran et la rangee ondule.
 */
function Stat({
  label,
  value,
  hint,
  children,
}: {
  label: string
  value: string
  hint?: string
  children?: ReactNode
}) {
  return (
    <div className="border-surface-sunken bg-surface flex min-w-px flex-1 flex-col gap-0.5 overflow-clip rounded-[12px] border p-0.5">
      <div className="flex w-full flex-1 flex-col gap-1 rounded-[10px] bg-white p-2">
        <p className="text-sm leading-[1.5] text-[#111]">{label}</p>
        <p className="text-xl leading-[1.4] font-semibold text-[#111] tabular-nums">{value}</p>
        {children !== undefined && <div className="mt-auto pt-1.5">{children}</div>}
      </div>

      <div className="bg-surface flex w-full items-center px-2 py-1.5">
        <p className="min-w-px flex-1 text-xs leading-[1.5] text-[#606060]">{hint ?? ''}</p>
      </div>
    </div>
  )
}

/** Ligne d'un bloc : meme gabarit d'un bloc a l'autre. */
function Row({ children }: { children: ReactNode }) {
  return (
    <li className="flex items-center gap-3 border-b border-[#f0f0f0] py-2.5 first:pt-0 last:border-b-0 last:pb-0">
      {children}
    </li>
  )
}

/** Ligne vide d'un bloc : dit ce qui manque, et comment le combler. */
function Empty({ children }: { children: ReactNode }) {
  return <p className="py-6 text-center text-[13px] text-[#8d8d8d]">{children}</p>
}

/** Coordonnee de l'entreprise : une icone, un intitule, une valeur. */
function Line({
  icon,
  label,
  value,
  href,
}: {
  icon: typeof Globe02Icon
  label: string
  value: string
  href?: string
}) {
  return (
    <div className="flex items-start gap-2.5 py-2">
      <HugeiconsIcon
        icon={icon}
        size={16}
        strokeWidth={1.6}
        className="mt-0.5 shrink-0 text-[#8d8d8d]"
      />
      <div className="flex min-w-0 flex-col">
        <span className="text-[11px] leading-[1.4] text-[#8d8d8d]">{label}</span>
        {value === '' ? (
          <span className="text-[14px] text-[#c4c4c4]">Non renseigné</span>
        ) : href !== undefined ? (
          <a
            href={href}
            target={href.startsWith('http') ? '_blank' : undefined}
            rel="noreferrer"
            className="truncate text-[14px] text-[#111] hover:underline"
          >
            {value}
          </a>
        ) : (
          <span className="truncate text-[14px] text-[#111]">{value}</span>
        )}
      </div>
    </div>
  )
}

function ClientDetailPage() {
  const { id } = Route.useParams()
  const navigate = useNavigate()

  const { data } = useQuery(clientDetailQuery(id))

  // Le loader a deja rempli le cache : `data` est present des le premier rendu.
  if (data === undefined) return null

  const client: CrmClientDetail = data
  const status = CLIENT_STATUS[client.status]

  // Avancement dans le pipeline : la position de l'etape dans la liste. Une
  // mesure grossiere, mais qui dit d'un coup d'oeil ou en est la relation.
  const step = CLIENT_STATUS_ORDER.indexOf(client.status)
  const pipeline = ((step + 1) / CLIENT_STATUS_ORDER.length) * 100

  const livres = client.projects.filter((project) => project.status === 'livre').length

  return (
    <PageFrame
      title={client.name}
      trail={[{ label: 'Clients', to: '/crm/clients', search: { page: 1 } }]}
    >
      <div className="flex flex-col gap-4 p-4">
        {/* En-tete : qui est ce client, et les gestes qui le concernent.
            Pose a plat, comme la fiche projet. L'encadrer en carte le rangeait
            au meme niveau que les blocs qu'il surplombe, et le titre y etait
            redit une troisieme fois apres l'onglet et le fil d'Ariane. */}
        <header className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex min-w-0 flex-col gap-1">
            <div className="flex flex-wrap items-center gap-2.5">
              <h1 className="truncate text-[24px] leading-[1.5] font-medium text-[#1b1b1b]">
                {client.name}
              </h1>
              <StatusPill label={status.label} color={status.color} pill={status.pill} />
            </div>

            <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
              <span className="text-[14px] leading-[1.5] text-[#73757c]">
                Client depuis le {DATE_FORMAT.format(new Date(client.created_at))}
              </span>

              {client.account_manager !== null && (
                <span className="flex items-center gap-1.5 text-[14px] leading-[1.5] text-[#73757c]">
                  <Avatars people={[client.account_manager]} max={1} size={20} />
                  {`${client.account_manager.firstname} ${client.account_manager.lastname}`.trim()}
                </span>
              )}
            </div>
          </div>

          {/* L'avancement dans le pipeline ne figure plus ici : la tuile
              « Étape », deux lignes plus bas, porte deja la meme jauge et la
              meme mention. */}
          <div className="flex shrink-0 flex-wrap items-center gap-4">
            <div className="flex flex-col gap-0.5">
              <span className="px-1.5 text-[11px] text-[#8d8d8d]">Contact principal</span>
              <PrimaryContactPicker
                clientId={client.id}
                current={client.primary_contact}
                contactsCount={client.contacts_count}
              />
            </div>

            <div className="flex items-center gap-2">
              <EditClientDialog
                client={client}
                trigger={<Button variant="outline">Modifier</Button>}
              />

              {/* Supprimer depuis la fiche renvoie a la liste : rester sur la
                  page d'un client efface n'aurait plus rien a montrer. */}
              <ClientRowActions
                client={client}
                onDeleted={() => void navigate({ to: '/crm/clients', search: { page: 1 } })}
              />
            </div>
          </div>
        </header>

        {/* Chiffres-cles, meme tuile que le tableau de bord du module PM. */}
        <div className="flex flex-wrap gap-3">
          <Stat
            label="Projets actifs"
            value={String(client.projects_active)}
            hint={livres > 0 ? `${livres} livré${livres > 1 ? 's' : ''}` : 'Aucun projet livré'}
          >
            {client.projects.length > 0 && (
              <Meter
                ratio={(client.projects_active / client.projects.length) * 100}
                color={PROJECT_STATUS.production.color}
              />
            )}
          </Stat>

          <Stat
            label="Contacts"
            value={String(client.contacts_count)}
            hint={
              client.primary_contact === null
                ? 'Aucun principal désigné'
                : `Principal : ${`${client.primary_contact.firstname} ${client.primary_contact.lastname}`.trim()}`
            }
          />

          <Stat
            label="Comptes portail"
            value={String(client.portal_users)}
            hint={client.portal_users === 0 ? 'Portail non ouvert' : 'Accès ouvert'}
          />

          <Stat
            label="Étape"
            value={status.label}
            hint={`Étape ${step + 1} sur ${CLIENT_STATUS_ORDER.length}`}
          >
            <Meter ratio={pipeline} color={status.color} />
          </Stat>
        </div>

        <div className="grid gap-3 xl:grid-cols-2">
          <PanelCard icon={Contact01Icon} title="CONTACTS">
            {client.contacts.length === 0 ? (
              <Empty>Aucun contact. Ajoutez-en un depuis l'écran Contacts.</Empty>
            ) : (
              <ul className="flex flex-col">
                {client.contacts.map((contact) => (
                  <Row key={contact.id}>
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="flex items-center gap-2">
                        <span className="truncate text-[14px] text-[#111]">
                          {`${contact.firstname} ${contact.lastname}`.trim()}
                        </span>
                        {contact.is_primary && (
                          <span className="shrink-0 rounded-full bg-brand/10 px-2 py-0.5 text-[11px] font-medium text-brand">
                            Principal
                          </span>
                        )}
                      </span>
                      <span className="truncate text-[12px] text-[#8d8d8d]">
                        {[contact.role, contact.email, formatPhone(contact.phone)]
                          .filter((part) => part !== '' && part !== null)
                          .join(' · ') || '—'}
                      </span>
                    </span>

                    <ContactRowActions contact={contact} />
                  </Row>
                ))}
              </ul>
            )}
          </PanelCard>

          <PanelCard icon={Folder01Icon} title="PROJETS">
            {client.projects.length === 0 ? (
              <Empty>Aucun projet pour ce client.</Empty>
            ) : (
              <ul className="flex flex-col">
                {client.projects.map((project) => {
                  const tint = PROJECT_STATUS[project.status as ProjectStatus]

                  return (
                    <Row key={project.id}>
                      <span className="flex min-w-0 flex-1 flex-col gap-1.5">
                        <Link
                          to="/pm/projets/$id"
                          params={{ id: project.id }}
                          className="truncate text-[14px] text-[#111] hover:underline"
                        >
                          {project.name}
                        </Link>
                        <Meter ratio={project.progress} color={tint.color} />
                      </span>

                      <span className="w-9 shrink-0 text-right text-[12px] text-[#8d8d8d] tabular-nums">
                        {project.progress} %
                      </span>

                      <StatusPill label={tint.label} color={tint.color} pill={tint.pill} />
                    </Row>
                  )
                })}
              </ul>
            )}
          </PanelCard>

          <PanelCard icon={Building03Icon} title="ENTREPRISE">
            <div className="grid gap-x-4 sm:grid-cols-2">
              <Line icon={Globe02Icon} label="Site web" value={client.website} href={client.website} />
              <Line
                icon={SmartPhone01Icon}
                label="Téléphone"
                value={formatPhone(client.phone)}
                href={client.phone === '' ? undefined : `tel:${client.phone}`}
              />
              <Line
                icon={Building03Icon}
                label="Adresse"
                value={[client.address, client.postal_code, client.city, client.country]
                  .filter((part) => part !== '')
                  .join(', ')}
              />
              <Line icon={UserStar01Icon} label="SIRET" value={client.siret} />
              <Line icon={UserStar01Icon} label="N° de TVA" value={client.vat_number} />
            </div>
          </PanelCard>

          <PanelCard icon={UserMultipleIcon} title="COMPTES PORTAIL">
            {client.accounts.length === 0 ? (
              <Empty>Aucun compte. Rien ne permet encore d'en ouvrir un depuis l'interface.</Empty>
            ) : (
              <ul className="flex flex-col">
                {client.accounts.map((account) => (
                  <Row key={account.id}>
                    <Avatars
                      people={[
                        {
                          id: account.id,
                          firstname: account.firstname,
                          lastname: account.lastname,
                          initials:
                            `${account.firstname.at(0) ?? ''}${account.lastname.at(0) ?? ''}`.toUpperCase(),
                          avatar_url: account.avatar_url,
                        },
                      ]}
                      max={1}
                      size={28}
                    />

                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate text-[14px] text-[#111]">
                        {`${account.firstname} ${account.lastname}`.trim() || account.email}
                      </span>
                      <span className="flex items-center gap-1 truncate text-[12px] text-[#8d8d8d]">
                        <HugeiconsIcon icon={Mail01Icon} size={12} strokeWidth={1.6} />
                        {account.email}
                      </span>
                    </span>

                    <span className="shrink-0 text-[12px] text-[#8d8d8d]">
                      {account.last_login_at === null
                        ? 'Jamais connecté'
                        : DATE_FORMAT.format(new Date(account.last_login_at))}
                    </span>
                  </Row>
                ))}
              </ul>
            )}
          </PanelCard>
        </div>
      </div>
    </PageFrame>
  )
}
