import { ViewIcon, ViewOffIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'

import logoUrl from '@/assets/sidebar/logo.svg'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

/**
 * Gabarit des pages publiques hors connexion : invitation, mot de passe oublie,
 * reinitialisation.
 *
 * Plus sobre que la page de connexion : ces pages s'ouvrent depuis un e-mail,
 * souvent sur un telephone, pour une seule action. Le logo dit ou l'on est, le
 * titre dit ce qu'on y fait, rien ne detourne de l'unique formulaire.
 */
export function AuthCard({
  title,
  subtitle,
  children,
  footer,
}: {
  title: string
  subtitle?: ReactNode
  children: ReactNode
  footer?: ReactNode
}) {
  return (
    <main className="flex min-h-full items-center justify-center bg-[#f7f7f8] px-4 py-12">
      <div className="flex w-full max-w-[440px] flex-col gap-6">
        <div className="flex flex-col items-center gap-4 text-center">
          <img src={logoUrl} alt="Piilot" className="size-12" />
          <div className="flex flex-col gap-1.5">
            <h1 className="text-[24px] leading-[1.3] font-semibold text-[#1b1b1b]">{title}</h1>
            {subtitle !== undefined && (
              <p className="text-[15px] leading-[1.5] text-[#73757c]">{subtitle}</p>
            )}
          </div>
        </div>

        <div className="rounded-[16px] border border-[#e8e8e9] bg-white p-6 shadow-[0_1px_3px_0_rgb(16_24_40/0.06)]">
          {children}
        </div>

        {footer ?? (
          <p className="text-center text-[14px] text-[#73757c]">
            <Link to="/login" className="text-brand hover:underline">
              Retour à la connexion
            </Link>
          </p>
        )}
      </div>
    </main>
  )
}

/**
 * Champ de mot de passe avec bouton « afficher » et jauge de longueur.
 *
 * La regle est une longueur minimale, pas une liste de caracteres imposes :
 * c'est ce qui rend un mot de passe solide, et une phrase de quelques mots se
 * retient mieux qu'un melange de symboles. La jauge la rend visible pendant la
 * saisie, plutot que de la decouvrir dans un message d'erreur.
 */
export function PasswordField({
  id,
  label,
  value,
  onChange,
  error,
  autoFocus,
  showMeter = false,
  autoComplete = 'new-password',
}: {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
  error?: string
  autoFocus?: boolean
  showMeter?: boolean
  autoComplete?: string
}) {
  const [shown, setShown] = useState(false)
  const min = 12
  const length = [...value].length
  const enough = length >= min

  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-[14px] font-medium text-[#1b1b1b]">
        {label}
      </label>
      <div className="relative">
        <Input
          id={id}
          type={shown ? 'text' : 'password'}
          value={value}
          autoFocus={autoFocus}
          autoComplete={autoComplete}
          aria-invalid={Boolean(error)}
          onChange={(event) => onChange(event.target.value)}
          className="h-11 pr-11"
        />
        <button
          type="button"
          onClick={() => setShown((s) => !s)}
          aria-label={shown ? 'Masquer le mot de passe' : 'Afficher le mot de passe'}
          className="absolute top-1/2 right-2 flex size-8 -translate-y-1/2 cursor-pointer items-center justify-center rounded-[8px] text-[#73757c] hover:bg-[#f3f4f4] hover:text-[#1b1b1b]"
        >
          <HugeiconsIcon icon={shown ? ViewOffIcon : ViewIcon} size={18} strokeWidth={1.6} />
        </button>
      </div>

      {showMeter && (
        <div className="flex items-center gap-2">
          <div aria-hidden className="h-1 flex-1 overflow-hidden rounded-full bg-[#ebebeb]">
            <div
              className={cn('h-full rounded-full transition-[width,background-color]', enough ? 'bg-[#0db471]' : 'bg-brand')}
              style={{ width: `${Math.min(100, (length / min) * 100)}%` }}
            />
          </div>
          <span className={cn('text-[12px] tabular-nums', enough ? 'text-[#0db471]' : 'text-[#73757c]')}>
            {enough ? 'Longueur suffisante' : `${length} / ${min} caractères`}
          </span>
        </div>
      )}

      {error && <p className="text-[13px] text-[#e5484d]">{error}</p>}
    </div>
  )
}
