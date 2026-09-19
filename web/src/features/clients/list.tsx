import {
  Building03Icon,
  Contact01Icon,
  Folder01Icon,
  PieChartIcon,
  UserStar01Icon,
  UserMultipleIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { CLIENT_STATUS } from '@/features/clients/format'
import { PrimaryContactPicker } from '@/features/clients/primary-contact-picker'
import { ClientRowActions } from '@/features/clients/row-actions'
import { Avatars, StatusPill } from '@/features/projects/ui'
import { cn } from '@/lib/utils'
import type { CrmClient } from '@/types/api'

/**
 * Vue liste des clients.
 *
 * Meme gabarit que le tableau des taches : un tableau construit en flex et non
 * en `<table>`, pour que les cellules s'etirent et se replient avec la fenetre.
 *
 * Les deux compteurs sont lus tels quels — ce sont des colonnes tenues par
 * declencheur en base, rien n'est recompte ici.
 */
interface Column {
  key: string
  label: string
  icon?: IconSvgElement
  width: string
}

const COLUMNS: Column[] = [
  { key: 'name', label: 'Client', icon: Building03Icon, width: 'min-w-[240px] flex-[2_1_240px]' },
  {
    key: 'contact',
    label: 'Contact principal',
    icon: Contact01Icon,
    width: 'min-w-[240px] flex-[2_1_240px]',
  },
  {
    key: 'contacts',
    label: 'Contacts',
    icon: UserMultipleIcon,
    width: 'min-w-[110px] flex-[1_1_110px]',
  },
  {
    key: 'projects',
    label: 'Projets actifs',
    icon: Folder01Icon,
    width: 'min-w-[140px] flex-[1_1_140px]',
  },
  {
    key: 'portal',
    label: 'Comptes portail',
    icon: UserMultipleIcon,
    width: 'min-w-[160px] flex-[1_1_160px]',
  },
  { key: 'status', label: 'Statut', icon: PieChartIcon, width: 'min-w-[140px] flex-[1_1_140px]' },
  {
    key: 'manager',
    label: 'Chargé de compte',
    icon: UserStar01Icon,
    width: 'min-w-[200px] flex-[2_1_200px]',
  },
  // Sans libelle : une colonne d'actions n'a rien a annoncer.
  { key: 'actions', label: '', width: 'w-[52px] shrink-0' },
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

export function ClientTable({ clients, empty }: { clients: CrmClient[]; empty: ReactNode }) {
  return (
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

        {clients.length === 0 && (
          <p className="border-b border-[#e8e8e9] px-2.5 py-8 text-center text-[14px] text-[#73757c]">
            {empty}
          </p>
        )}

        {clients.map((client) => (
          <div key={client.id} className="flex items-stretch bg-white">
            <Cell width={COLUMNS[0]!.width}>
              <Link
                to="/crm/clients/$id"
                params={{ id: client.id }}
                className="truncate text-[14px] text-[#1b1b1b] hover:underline"
              >
                {client.name}
              </Link>
            </Cell>

            {/* Le contact principal se choisit sur place : c'est la cle
                etrangere du client, pas une valeur libre. */}
            <Cell width={COLUMNS[1]!.width}>
              <PrimaryContactPicker
                clientId={client.id}
                current={client.primary_contact}
                contactsCount={client.contacts_count}
              />
            </Cell>

            <Cell width={COLUMNS[2]!.width}>
              <span
                className={cn(
                  'text-[14px]',
                  client.contacts_count === 0 ? 'text-[#a2a3a7]' : 'text-[#1b1b1b]',
                )}
              >
                {client.contacts_count}
              </span>
            </Cell>

            <Cell width={COLUMNS[3]!.width}>
              <span
                className={cn(
                  'text-[14px]',
                  client.projects_active === 0 ? 'text-[#a2a3a7]' : 'text-[#1b1b1b]',
                )}
              >
                {client.projects_active}
              </span>
            </Cell>

            <Cell width={COLUMNS[4]!.width}>
              {client.portal_users === 0 ? (
                <span className="text-[14px] text-[#a2a3a7]">Aucun</span>
              ) : (
                <span className="text-[14px] text-[#1b1b1b]">{client.portal_users}</span>
              )}
            </Cell>

            <Cell width={COLUMNS[5]!.width}>
              <StatusPill
                label={CLIENT_STATUS[client.status].label}
                color={CLIENT_STATUS[client.status].color}
                pill={CLIENT_STATUS[client.status].pill}
              />
            </Cell>

            <Cell width={COLUMNS[6]!.width} className="gap-2">
              {client.account_manager === null ? (
                <span className="text-[14px] text-[#a2a3a7]">Non assigné</span>
              ) : (
                <>
                  <Avatars people={[client.account_manager]} max={1} size={20} />
                  <span className="truncate text-[14px] text-[#1b1b1b]">
                    {`${client.account_manager.firstname} ${client.account_manager.lastname}`.trim()}
                  </span>
                </>
              )}
            </Cell>

            <Cell width={COLUMNS[7]!.width} className="justify-center">
              <ClientRowActions client={client} />
            </Cell>
          </div>
        ))}
      </div>
    </div>
  )
}
