import { Alert02Icon, CheckmarkCircle02Icon, Loading03Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useDeferredValue, type ReactNode } from 'react'

import { isValidSiret, lookupSiret, normalizeSiret, sameLegalName, type RegistryCompany } from '@/features/clients/registry'
import { cn } from '@/lib/utils'

export type RegistryState = 'idle' | 'invalid' | 'loading' | 'found' | 'missing' | 'error'

/** Bandeau sous le SIRET : ce que le registre en dit. */
export function RegistryStatus({
  siret,
  state,
  company,
}: {
  siret: string
  state: RegistryState
  company: RegistryCompany | null | undefined
}) {
  if (state === 'idle') {
    return (
      <p className="text-[12px] text-[#73757c]">
        Le SIRET est vérifié au registre des entreprises, qui complète le reste de la fiche.
      </p>
    )
  }

  const tone = {
    invalid: 'border-[#f3c4c4] bg-[#fdf3f3] text-[#a12b2b]',
    loading: 'border-input bg-[#fafafa] text-[#73757c]',
    found: company?.active === false ? 'border-[#f2d7a6] bg-[#fdf8ee] text-[#8a5a00]' : 'border-[#c9e6d2] bg-[#f2faf4] text-[#1d6b3a]',
    missing: 'border-[#f3c4c4] bg-[#fdf3f3] text-[#a12b2b]',
    error: 'border-[#f2d7a6] bg-[#fdf8ee] text-[#8a5a00]',
  }[state]

  let icon = Alert02Icon
  let title: ReactNode
  let detail: ReactNode = null

  switch (state) {
    case 'invalid':
      title = siret.length === 14 ? 'Clé de contrôle invalide : vérifiez la saisie.' : `${siret.length} chiffres sur 14.`
      break
    case 'loading':
      icon = Loading03Icon
      title = 'Recherche au registre…'
      break
    case 'missing':
      title = 'Aucun établissement ne porte ce SIRET au registre.'
      break
    case 'error':
      title = 'Registre injoignable : le client sera créé sans vérification.'
      break
    case 'found':
      if (company == null) return null
      icon = company.active ? CheckmarkCircle02Icon : Alert02Icon
      title = company.active ? 'Établissement actif au registre' : 'Établissement fermé au registre'
      detail = (
        <>
          {company.hidden ? 'Identité non diffusée par l’entreprise' : company.legalName}
          {company.legalForm !== '' && ` · ${company.legalForm}`}
          {(company.address !== '' || company.city !== '') &&
            ` · ${[company.address, company.postalCode, company.city].filter(Boolean).join(' ')}`}
        </>
      )
      break
  }

  return (
    <div role="status" className={cn('flex items-start gap-2 rounded-[8px] border px-3 py-2 text-[13px]', tone)}>
      <HugeiconsIcon
        icon={icon}
        size={16}
        strokeWidth={1.8}
        className={cn('mt-0.5 shrink-0', state === 'loading' && 'animate-spin')}
      />
      <span className="flex min-w-0 flex-col">
        <span className="font-medium">{title}</span>
        {detail !== null && <span className="text-[12px] opacity-90">{detail}</span>}
      </span>
    </div>
  )
}

/**
 * Interroge le registre pour le SIRET saisi, des qu'il est complet et bien
 * forme. Le resultat se garde : revenir a un SIRET deja verifie ne relance
 * rien.
 */
export function useRegistry(raw: string) {
  const siret = normalizeSiret(raw)
  const deferred = useDeferredValue(siret)
  const valid = isValidSiret(deferred)

  const query = useQuery({
    queryKey: ['registry', 'siret', deferred],
    queryFn: ({ signal }) => lookupSiret(deferred, signal),
    enabled: valid,
    staleTime: Infinity,
    retry: false,
  })

  let state: RegistryState
  if (siret === '') state = 'idle'
  else if (!isValidSiret(siret)) state = 'invalid'
  else if (!valid || query.isPending) state = 'loading'
  else if (query.isError) state = 'error'
  else state = query.data === null ? 'missing' : 'found'

  return { siret, state, company: valid ? query.data : undefined }
}

/**
 * Conformite de la raison sociale saisie a celle du registre, avec de quoi
 * reprendre celle du registre d'un clic.
 */
export function LegalNameCheck({
  value,
  official,
  onAdopt,
}: {
  value: string
  official: string
  onAdopt: (official: string) => void
}) {
  if (official === '' || value === '') return null

  if (sameLegalName(value, official)) {
    return (
      <p className="flex items-center gap-1.5 text-[12px] text-[#1d6b3a]">
        <HugeiconsIcon icon={CheckmarkCircle02Icon} size={14} strokeWidth={1.8} />
        Conforme au registre
      </p>
    )
  }

  return (
    <p className="flex flex-wrap items-center gap-1.5 text-[12px] text-[#8a5a00]">
      <HugeiconsIcon icon={Alert02Icon} size={14} strokeWidth={1.8} />
      Le registre indique « {official} ».
      <button
        type="button"
        className="cursor-pointer font-medium underline underline-offset-2"
        onClick={() => onAdopt(official)}
      >
        Reprendre
      </button>
    </p>
  )
}
