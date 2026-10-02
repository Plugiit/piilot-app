import { LockIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { roleMatrixQuery, useSetRolePermissions } from '@/features/accounts/api'
import { roleOf } from '@/features/accounts/format'
import { HttpError } from '@/lib/api'
import { sessionQuery } from '@/lib/auth'
import { cn } from '@/lib/utils'
import type { PermissionInfo, RoleMatrix } from '@/types/api'

/**
 * Ordre des groupes : celui du travail — on produit, on suit le client, puis
 * on administre — plutot que l'ordre alphabetique des codes, qui ouvrait la
 * grille sur le CRM.
 */
const GROUP_ORDER = [
  'Pilotage',
  'Projets',
  'Tâches',
  'Livrables',
  'Tickets',
  'Temps passé',
  'CRM',
  'Comptes',
  'Rôles',
  'Système',
]

export const Route = createFileRoute('/_app/parametres/comptes/roles')({
  component: RolesPage,
})

/**
 * Ce que chaque role permet.
 *
 * Une grille plutot qu'une fiche par role : la question qu'on se pose ici est
 * presque toujours « qui peut faire ceci ? », et elle se lit sur une ligne.
 *
 * Les cases grisees ne sont pas des oublis : le role admin garde tout, les
 * droits d'administration ne quittent pas le role admin, et le portail n'a
 * acces qu'a ses propres permissions. L'infobulle de chaque case le dit.
 */
function RolesPage() {
  const { data: matrix, isPending } = useQuery(roleMatrixQuery)
  const { data: session } = useQuery(sessionQuery)
  const canEdit = session?.permissions.includes('roles.write') === true

  if (isPending || matrix === undefined) {
    return <div className="m-4 h-[320px] animate-pulse rounded-[12px] bg-[#fafafa]" />
  }

  return <RoleGrid key={JSON.stringify(matrix.roles.map((r) => r.permissions))} matrix={matrix} canEdit={canEdit} />
}

function RoleGrid({ matrix, canEdit }: { matrix: RoleMatrix; canEdit: boolean }) {
  const save = useSetRolePermissions()

  // Brouillon par role : on coche librement, puis on enregistre ou on
  // abandonne d'un geste. Une ecriture par case aurait laisse la politique
  // dans un etat intermediaire a chaque clic.
  const [draft, setDraft] = useState<Record<string, string[]>>(() =>
    Object.fromEntries(matrix.roles.map((role) => [role.code, role.permissions])),
  )

  const changed = matrix.roles.filter((role) => {
    const now = draft[role.code] ?? []
    return now.length !== role.permissions.length || now.some((code) => !role.permissions.includes(code))
  })

  const groups = new Map<string, PermissionInfo[]>()
  const rank = (group: string) => {
    const index = GROUP_ORDER.indexOf(group)
    return index === -1 ? GROUP_ORDER.length : index
  }
  for (const permission of [...matrix.permissions].sort((a, b) => rank(a.group) - rank(b.group))) {
    groups.set(permission.group, [...(groups.get(permission.group) ?? []), permission])
  }

  function toggle(role: string, code: string, on: boolean) {
    setDraft((prev) => {
      const current = prev[role] ?? []
      return { ...prev, [role]: on ? [...current, code] : current.filter((c) => c !== code) }
    })
  }

  async function submit() {
    try {
      for (const role of changed) {
        await save.mutateAsync({ role: role.code, permissions: draft[role.code] ?? [] })
      }
      toast.success('Permissions enregistrées. Elles s’appliquent dès la prochaine action de chacun.')
    } catch (error) {
      toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible')
    }
  }

  const columns = `minmax(260px,1fr) repeat(${matrix.roles.length}, minmax(120px,160px))`

  return (
    <div className="flex flex-1 flex-col gap-3 p-4 pb-24">
      <p className="text-[13px] text-[#73757c]">
        Un droit retiré s’applique tout de suite, sans attendre que la personne se reconnecte.
      </p>

      <div className="overflow-x-auto rounded-[12px] border border-[#e8e8e9]">
        <div className="min-w-max">
          <div
            className="sticky top-0 z-10 grid items-end border-b border-[#e8e8e9] bg-[#f3f4f4] px-3 py-3"
            style={{ gridTemplateColumns: columns }}
          >
            <span className="text-[13px] text-[#73757c]">Permission</span>
            {matrix.roles.map((role) => (
              <div key={role.code} className="flex flex-col items-center gap-0.5 text-center" title={role.note}>
                <span className="flex items-center gap-1.5 text-[14px] font-medium text-[#1b1b1b]">
                  <span aria-hidden className="size-2 rounded-full" style={{ backgroundColor: roleOf(role.code).color }} />
                  {role.label}
                  {!role.editable && (
                    <HugeiconsIcon icon={LockIcon} size={13} strokeWidth={1.8} className="text-[#a2a3a7]" />
                  )}
                </span>
                <span className="text-[12px] text-[#73757c]">
                  {role.users} compte{role.users > 1 ? 's' : ''}
                </span>
              </div>
            ))}
          </div>

          {[...groups.entries()].map(([group, permissions]) => (
            <div key={group}>
              <div className="bg-[#fafafa] px-3 py-1.5 text-[12px] font-medium tracking-wide text-[#73757c] uppercase">
                {group}
              </div>
              {permissions.map((permission) => (
                <div
                  key={permission.code}
                  className="grid items-center border-t border-[#f0f0f1] bg-white px-3 py-2.5 hover:bg-[#fcfcfc]"
                  style={{ gridTemplateColumns: columns }}
                >
                  <div className="flex flex-col">
                    <span className="text-[14px] text-[#1b1b1b]">{permission.label}</span>
                    <span className="font-mono text-[11px] text-[#a2a3a7]">{permission.code}</span>
                  </div>

                  {matrix.roles.map((role) => {
                    const grantable = role.grantable.includes(permission.code)
                    const checked = (draft[role.code] ?? []).includes(permission.code)
                    const locked = !canEdit || !role.editable || !grantable
                    const reason = !role.editable
                      ? 'Le rôle administrateur garde toutes les permissions'
                      : !grantable
                        ? permission.admin_only
                          ? 'Réservée aux administrateurs'
                          : 'Sans effet dans le portail client'
                        : !canEdit
                          ? 'Modifiable par un administrateur'
                          : undefined

                    return (
                      <div key={role.code} className="flex justify-center" title={reason}>
                        <Checkbox
                          checked={checked}
                          disabled={locked}
                          aria-label={`${permission.label} pour ${role.label}`}
                          onCheckedChange={(value) => toggle(role.code, permission.code, value === true)}
                          className={cn('size-5 rounded-[6px]', locked && !checked && 'opacity-40')}
                        />
                      </div>
                    )
                  })}
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>

      {changed.length > 0 && (
        <div className="fixed bottom-6 left-1/2 z-20 flex -translate-x-1/2 items-center gap-3 rounded-[12px] border border-[#e8e8e9] bg-white py-2 pr-2 pl-4 shadow-[0_12px_28px_-8px_rgb(16_24_40/0.28)]">
          <p className="text-[13px] text-[#1b1b1b]">
            Modifications non enregistrées : {changed.map((role) => role.label).join(', ')}
          </p>
          <Button
            size="sm"
            variant="ghost"
            onClick={() =>
              setDraft(Object.fromEntries(matrix.roles.map((role) => [role.code, role.permissions])))
            }
          >
            Annuler
          </Button>
          <Button size="sm" disabled={save.isPending} onClick={() => void submit()}>
            {save.isPending ? 'Enregistrement…' : 'Enregistrer'}
          </Button>
        </div>
      )}
    </div>
  )
}
