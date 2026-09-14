import {
  Building03Icon,
  Contact01Icon,
  Mail01Icon,
  SmartPhone01Icon,
  UserStar01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import type { ReactNode } from 'react'

import { ContactRowActions } from '@/features/contacts/row-actions'
import { formatPhone } from '@/lib/countries'
import { cn } from '@/lib/utils'
import type { CrmContact } from '@/types/api'

/**
 * Vue liste des contacts.
 *
 * Meme gabarit que les tableaux des taches et des clients : construit en flex
 * et non en `<table>`, pour que les cellules s'etirent avec la fenetre.
 */
interface Column {
  key: string
  label: string
  icon?: IconSvgElement
  width: string
}

const COLUMNS: Column[] = [
  { key: 'name', label: 'Contact', icon: Contact01Icon, width: 'min-w-[220px] flex-[2_1_220px]' },
  { key: 'client', label: 'Client', icon: Building03Icon, width: 'min-w-[200px] flex-[2_1_200px]' },
  { key: 'role', label: 'Rôle', icon: UserStar01Icon, width: 'min-w-[190px] flex-[2_1_190px]' },
  { key: 'email', label: 'E-mail', icon: Mail01Icon, width: 'min-w-[220px] flex-[2_1_220px]' },
  { key: 'phone', label: 'Téléphone', icon: SmartPhone01Icon, width: 'min-w-[150px] flex-[1_1_150px]' },
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

/** Nom affichable, l'un des deux champs pouvant etre vide. */
function fullName(contact: CrmContact): string {
  return `${contact.firstname} ${contact.lastname}`.trim()
}

export function ContactTable({ contacts, empty }: { contacts: CrmContact[]; empty: ReactNode }) {
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

        {contacts.length === 0 && (
          <p className="border-b border-[#e8e8e9] px-2.5 py-8 text-center text-[14px] text-[#73757c]">
            {empty}
          </p>
        )}

        {contacts.map((contact) => (
          <div key={contact.id} className="flex items-stretch bg-white">
            <Cell width={COLUMNS[0]!.width} className="gap-2">
              <span className="truncate text-[14px] text-[#1b1b1b]">{fullName(contact)}</span>
              {/* La pastille dit d'un coup d'oeil qui le client montre en
                  premier, sans avoir a ouvrir l'autre tableau. */}
              {contact.is_primary && (
                <span className="shrink-0 rounded-full bg-brand/10 px-2 py-0.5 text-[11px] font-medium text-brand">
                  Principal
                </span>
              )}
            </Cell>

            {/* Un contact libre n'a pas encore d'entreprise : le dire
                plutot que de laisser une cellule vide. */}
            <Cell width={COLUMNS[1]!.width}>
              {contact.client_name === null ? (
                <span className="text-[14px] text-[#a2a3a7]">Sans client</span>
              ) : (
                <span className="truncate text-[14px] text-[#1b1b1b]">{contact.client_name}</span>
              )}
            </Cell>

            <Cell width={COLUMNS[2]!.width}>
              {contact.role === '' ? (
                <span className="text-[14px] text-[#a2a3a7]">—</span>
              ) : (
                <span className="truncate text-[14px] text-[#73757c]">{contact.role}</span>
              )}
            </Cell>

            <Cell width={COLUMNS[3]!.width}>
              {contact.email === null || contact.email === '' ? (
                <span className="text-[14px] text-[#a2a3a7]">—</span>
              ) : (
                <a
                  href={`mailto:${contact.email}`}
                  className="truncate text-[14px] text-[#1b1b1b] hover:underline"
                >
                  {contact.email}
                </a>
              )}
            </Cell>

            <Cell width={COLUMNS[4]!.width}>
              {contact.phone === '' ? (
                <span className="text-[14px] text-[#a2a3a7]">—</span>
              ) : (
                <span className="text-[14px] whitespace-nowrap text-[#1b1b1b]">
                  {formatPhone(contact.phone)}
                </span>
              )}
            </Cell>

            <Cell width={COLUMNS[5]!.width} className="justify-center">
              <ContactRowActions contact={contact} />
            </Cell>
          </div>
        ))}
      </div>
    </div>
  )
}
