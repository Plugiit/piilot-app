import {
  Key01Icon,
  MoreHorizontalIcon,
  Search01Icon,
  ShieldUserIcon,
  UserBlock01Icon,
  UserCheck01Icon,
  UserMultipleIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'
import { z } from 'zod'

import { FilterMenu, type Option } from '@/components/filter-menu'
import { HoverMenuContent, HoverMenuItem } from '@/components/hover-menu'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import {
  ACCOUNT_PAGE_SIZE,
  accountListQuery,
  usePasswordResetLink,
  useSetAccountEnabled,
  useSetRole,
} from '@/features/accounts/api'
import { displayName, lastSeen, ROLE, roleOf, type RoleCode } from '@/features/accounts/format'
import { LinkBox } from '@/features/accounts/link-box'
import { Avatars } from '@/features/projects/ui'
import { HttpError } from '@/lib/api'
import { sessionQuery } from '@/lib/auth'
import { useSearchField } from '@/lib/search-field'
import { cn } from '@/lib/utils'
import type { Account, SentLink } from '@/types/api'

const searchSchema = z.object({
  page: z.number().int().min(1).catch(1),
  search: z.string().optional(),
  role: z.enum(['admin', 'team', 'client']).optional().catch(undefined),
  status: z.enum(['active', 'disabled']).optional().catch(undefined),
})

export const Route = createFileRoute('/_app/parametres/comptes/')({
  validateSearch: searchSchema,
  component: AccountsPage,
})

function reportError(error: unknown) {
  toast.error(error instanceof HttpError ? error.message : 'Action impossible')
}

/** Pastille de role, dans la couleur du role. */
function RolePill({ role }: { role: string }) {
  const { label, color } = roleOf(role)
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-[#e8e8e9] px-2 py-0.5 text-[12px] whitespace-nowrap text-[#1b1b1b]">
      <span aria-hidden className="size-1.5 rounded-full" style={{ backgroundColor: color }} />
      {label}
    </span>
  )
}

/**
 * Actions sur un compte.
 *
 * Absentes de sa propre ligne : on ne retire pas son propre role, on ne se
 * desactive pas soi-meme — le serveur le refuse, l'ecran ne le propose pas.
 * La desactivation passe par une confirmation, qui dit ce qu'elle coupe ;
 * la reactivation, sans consequence, se fait d'un clic.
 */
function RowActions({ account, onLink }: { account: Account; onLink: (link: SentLink) => void }) {
  const setRole = useSetRole()
  const setEnabled = useSetAccountEnabled()
  const resetLink = usePasswordResetLink()
  const [confirm, setConfirm] = useState(false)

  const internal = account.role === 'admin' || account.role === 'team'
  const disabled = account.status === 'disabled'
  const other: RoleCode = account.role === 'admin' ? 'team' : 'admin'

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label={`Actions sur ${displayName(account)}`}
          className="flex cursor-pointer items-center justify-center rounded-[6px] p-1 text-[#73757c] transition-colors hover:bg-[#f3f4f4] hover:text-[#1b1b1b] aria-expanded:bg-[#f3f4f4]"
        >
          <HugeiconsIcon icon={MoreHorizontalIcon} size={18} strokeWidth={1.8} />
        </DropdownMenuTrigger>

        <HoverMenuContent align="end" className="min-w-[240px]">
          {internal && !disabled && (
            <HoverMenuItem
              onSelect={() =>
                setRole.mutate(
                  { id: account.id, role: other as 'admin' | 'team' },
                  {
                    onSuccess: () =>
                      toast.success(`${displayName(account)} passe en ${ROLE[other].label.toLowerCase()}`),
                    onError: reportError,
                  },
                )
              }
            >
              <HugeiconsIcon icon={ShieldUserIcon} size={16} strokeWidth={1.6} className="text-[#73757c]" />
              {other === 'admin' ? 'Passer administrateur' : 'Passer en équipe'}
            </HoverMenuItem>
          )}

          {!disabled && (
            <HoverMenuItem
              onSelect={() =>
                resetLink.mutate(account.id, {
                  onSuccess: onLink,
                  onError: reportError,
                })
              }
            >
              <HugeiconsIcon icon={Key01Icon} size={16} strokeWidth={1.6} className="text-[#73757c]" />
              Lien de réinitialisation
            </HoverMenuItem>
          )}

          <DropdownMenuSeparator />

          {disabled ? (
            <HoverMenuItem
              onSelect={() =>
                setEnabled.mutate(
                  { id: account.id, enabled: true },
                  {
                    onSuccess: () => toast.success(`${displayName(account)} peut de nouveau se connecter`),
                    onError: reportError,
                  },
                )
              }
            >
              <HugeiconsIcon icon={UserCheck01Icon} size={16} strokeWidth={1.6} className="text-[#73757c]" />
              Réactiver
            </HoverMenuItem>
          ) : (
            <HoverMenuItem danger onSelect={() => setConfirm(true)}>
              <HugeiconsIcon icon={UserBlock01Icon} size={16} strokeWidth={1.6} />
              Désactiver…
            </HoverMenuItem>
          )}
        </HoverMenuContent>
      </DropdownMenu>

      <Dialog open={confirm} onOpenChange={setConfirm}>
        <DialogContent className="sm:max-w-[460px]">
          <DialogHeader>
            <DialogTitle>Désactiver {displayName(account)} ?</DialogTitle>
            <DialogDescription>
              Le compte est déconnecté immédiatement, sur tous ses appareils, et ne peut plus se
              connecter. Ses tâches, son temps et ses commentaires restent en place. Vous pourrez le
              réactiver à tout moment.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" size="lg" onClick={() => setConfirm(false)}>
              Annuler
            </Button>
            <Button
              variant="destructive"
              size="lg"
              disabled={setEnabled.isPending}
              onClick={() =>
                setEnabled.mutate(
                  { id: account.id, enabled: false },
                  {
                    onSuccess: () => {
                      setConfirm(false)
                      toast.success(`Le compte de ${displayName(account)} est désactivé`)
                    },
                    onError: reportError,
                  },
                )
              }
            >
              Désactiver le compte
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

function AccountsPage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const { data: session } = useQuery(sessionQuery)
  const canManage = session?.permissions.includes('users.write') === true

  const { data, isPending } = useQuery(
    accountListQuery({
      page: search.page,
      search: search.search?.trim() === '' ? undefined : search.search,
      role: search.role,
      status: search.status,
    }),
  )

  // Lien de reinitialisation tout juste cree : il ne s'affiche qu'une fois.
  const [link, setLink] = useState<(SentLink & { name: string }) | null>(null)

  const rows = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / ACCOUNT_PAGE_SIZE))
  const filtered = Boolean(search.search || search.role || search.status)

  function setFilter(patch: Partial<z.infer<typeof searchSchema>>) {
    void navigate({ search: (prev) => ({ ...prev, ...patch, page: 1 }), replace: true })
  }

  const [draft, setDraft] = useSearchField(search.search ?? '', (value) =>
    setFilter({ search: value === '' ? undefined : value }),
  )

  const roleOptions: Option[] = (Object.keys(ROLE) as RoleCode[]).map((role) => ({
    value: role,
    label: ROLE[role].label,
    color: ROLE[role].color,
  }))
  const statusOptions: Option[] = [
    { value: 'active', label: 'Actifs', color: '#0db471' },
    { value: 'disabled', label: 'Désactivés', color: '#c4c4c4' },
  ]

  return (
    <div className="flex flex-1 flex-col gap-3 p-4">
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
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            placeholder="Rechercher un nom ou une adresse"
            aria-label="Rechercher un compte"
            className="h-9 pl-8 text-[13px]"
          />
        </div>
        <FilterMenu
          name="Rôle"
          all="Tous les rôles"
          value={search.role}
          options={roleOptions}
          onChange={(value) => setFilter({ role: value as RoleCode | undefined })}
        />
        <FilterMenu
          name="État"
          all="Tous les états"
          value={search.status}
          options={statusOptions}
          onChange={(value) => setFilter({ status: value as 'active' | 'disabled' | undefined })}
        />
      </div>

      {data !== undefined && !data.mail_enabled && canManage && (
        <p className="rounded-[10px] bg-[#f3f4f4] px-3 py-2.5 text-[13px] text-[#4b4b4f]">
          Piilot n’envoie pas d’e-mails sur cette instance : les liens d’invitation et de
          réinitialisation s’affichent ici, à transmettre vous-même. Renseignez SMTP_HOST pour les
          envoyer automatiquement.
        </p>
      )}

      <div className="overflow-hidden rounded-[12px] border border-[#e8e8e9]">
        <div className="grid grid-cols-[minmax(240px,2fr)_minmax(130px,1fr)_minmax(150px,1fr)_minmax(150px,1fr)_44px] items-center bg-[#f3f4f4] px-3 py-2.5 text-[13px] text-[#73757c] max-md:hidden">
          <span>Personne</span>
          <span>Rôle</span>
          <span>Client</span>
          <span>Dernière connexion</span>
          <span />
        </div>

        {isPending &&
          Array.from({ length: 5 }, (_, index) => (
            <div key={index} className="h-[61px] animate-pulse border-t border-[#e8e8e9] bg-[#fafafa]" />
          ))}

        {!isPending && rows.length === 0 && (
          <div className="flex flex-col items-center gap-2 border-t border-[#e8e8e9] px-4 py-10 text-center">
            <HugeiconsIcon icon={UserMultipleIcon} size={28} strokeWidth={1.4} className="text-[#a2a3a7]" />
            <p className="text-[14px] text-[#73757c]">
              {filtered ? 'Aucun compte ne correspond à ces filtres.' : 'Aucun compte pour l’instant.'}
            </p>
          </div>
        )}

        {rows.map((account) => {
          const disabled = account.status === 'disabled'
          return (
            <div
              key={account.id}
              className="grid grid-cols-[minmax(240px,2fr)_minmax(130px,1fr)_minmax(150px,1fr)_minmax(150px,1fr)_44px] items-center border-t border-[#e8e8e9] bg-white px-3 py-2.5 max-md:grid-cols-[1fr_44px] max-md:gap-y-1"
            >
              <div className={cn('flex min-w-0 items-center gap-2.5', disabled && 'opacity-55')}>
                <Avatars people={[account]} max={1} size={32} />
                <div className="flex min-w-0 flex-col">
                  <span className="flex items-center gap-1.5 truncate text-[14px] font-medium text-[#1b1b1b]">
                    {displayName(account)}
                    {account.is_self && (
                      <span className="rounded-full bg-[#f3f4f4] px-1.5 py-px text-[11px] font-normal text-[#73757c]">
                        Vous
                      </span>
                    )}
                  </span>
                  <span className="truncate text-[13px] text-[#73757c]">{account.email}</span>
                </div>
              </div>

              <div className="flex items-center gap-1.5 max-md:hidden">
                <RolePill role={account.role} />
                {disabled && (
                  <span className="rounded-full bg-[#f3f4f4] px-2 py-0.5 text-[12px] text-[#73757c]">
                    Désactivé
                  </span>
                )}
              </div>

              <span className="truncate text-[14px] text-[#1b1b1b] max-md:hidden">
                {account.client?.name ?? <span className="text-[#c4c4c4]">—</span>}
              </span>

              <span
                className={cn(
                  'text-[13px] max-md:hidden',
                  account.last_login_at === null ? 'text-[#a2a3a7]' : 'text-[#4b4b4f]',
                )}
                title={account.last_login_at ?? undefined}
              >
                {lastSeen(account)}
              </span>

              <div className="flex justify-end">
                {canManage && !account.is_self && (
                  <RowActions
                    account={account}
                    onLink={(sent) => setLink({ ...sent, name: displayName(account) })}
                  />
                )}
              </div>
            </div>
          )
        })}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-[12px] text-[#777]">
          {total} compte{total > 1 ? 's' : ''} · page {search.page} sur {totalPages}
        </p>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={search.page <= 1}
            onClick={() => void navigate({ search: (prev) => ({ ...prev, page: search.page - 1 }) })}
          >
            Précédent
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={search.page >= totalPages}
            onClick={() => void navigate({ search: (prev) => ({ ...prev, page: search.page + 1 }) })}
          >
            Suivant
          </Button>
        </div>
      </div>

      <Dialog open={link !== null} onOpenChange={(open) => !open && setLink(null)}>
        <DialogContent className="sm:max-w-[520px]">
          {link !== null && (
            <>
              <DialogHeader>
                <DialogTitle>Lien de réinitialisation</DialogTitle>
                <DialogDescription>
                  {link.emailed
                    ? `Un e-mail vient de partir pour ${link.name}. Ce lien sert s’il n’arrive pas.`
                    : `Transmettez ce lien à ${link.name} : il permet de choisir un nouveau mot de passe.`}
                </DialogDescription>
              </DialogHeader>
              <LinkBox link={link.link} expiresAt={link.expires_at} />
              <DialogFooter>
                <Button size="lg" onClick={() => setLink(null)}>
                  Terminer
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
