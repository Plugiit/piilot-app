import {
  Building03Icon,
  CheckmarkSquare02Icon,
  Contact01Icon,
  Folder01Icon,
  PlusSignIcon,
  Ticket02Icon,
  UserIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useDeferredValue, useEffect, useState } from 'react'

import { MODULES, menuFor, type MenuItem } from '@/components/layout/modules'
import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command'
import { ProjectLogo } from '@/features/projects/ui'
import { searchQuery } from '@/features/search/api'
import { can, outOfReach, sessionQuery } from '@/lib/auth'
import { requestCreate, setPaletteOpen, usePaletteOpen, type CreateKind } from '@/lib/palette'

/**
 * Palette Cmd+K : chercher n'importe quoi, aller n'importe ou, creer sans
 * passer par le bon ecran.
 *
 * Deux sources. Les commandes — navigation et creation — sont filtrees par
 * cmdk sur ce qu'on tape. Les resultats — projets, clients, contacts, taches,
 * tickets — viennent du serveur, qui n'en rend que les cinq premiers de chaque
 * famille et taira celles qu'on n'a pas le droit de voir ; cmdk ne les
 * refiltre pas, le serveur l'a deja fait.
 */
export function CommandPalette() {
  const open = usePaletteOpen()
  const navigate = useNavigate()
  const { data: session } = useQuery(sessionQuery)
  const [query, setQuery] = useState('')
  const deferred = useDeferredValue(query.trim())

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setPaletteOpen(true)
        return
      }
      // « / » ouvre aussi, sauf depuis un champ de saisie ou l'on tape du
      // texte, et sauf avec un modificateur.
      if (event.key === '/' && !event.metaKey && !event.ctrlKey && !event.altKey && !typing(event)) {
        event.preventDefault()
        setPaletteOpen(true)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const { data, isFetching } = useQuery({
    ...searchQuery(deferred),
    enabled: open && deferred.length >= 2,
  })

  function close() {
    setPaletteOpen(false)
    setQuery('')
  }

  function go(to: string, params?: Record<string, string>, search?: Record<string, unknown>) {
    close()
    void navigate({ to, params, search } as never)
  }

  function create(kind: CreateKind, to: string) {
    close()
    requestCreate(kind)
    void navigate({ to } as never)
  }

  const permissions = session?.permissions ?? []
  const destinations = MODULES.flatMap((module) =>
    menuFor(module, permissions, (to) => outOfReach(session, to)).flatMap((group) =>
      group.items.flatMap((item) => leavesOf(item, module.label)),
    ),
  )

  const creations = (
    [
    { kind: 'project', label: 'Nouveau projet', to: '/pm/projets', permission: 'projects.write', icon: Folder01Icon },
    { kind: 'task', label: 'Nouvelle tâche', to: '/pm/taches', permission: 'tasks.write', icon: CheckmarkSquare02Icon },
    { kind: 'ticket', label: 'Nouveau ticket', to: '/pm/tickets', permission: 'tickets.write', icon: Ticket02Icon },
    { kind: 'client', label: 'Nouveau client', to: '/crm/clients', permission: 'clients.write', icon: Building03Icon },
    { kind: 'contact', label: 'Nouveau contact', to: '/crm/contacts', permission: 'clients.write', icon: Contact01Icon },
    { kind: 'interaction', label: 'Noter une interaction', to: '/crm/interactions', permission: 'clients.write', icon: UserIcon },
    ] satisfies { kind: CreateKind; label: string; to: string; permission: string; icon: typeof PlusSignIcon }[]
  ).filter((c) => can(session, c.permission) && !outOfReach(session, c.to))

  const results = data
  const hasResults =
    results !== undefined &&
    (results.projects.length + results.clients.length + results.contacts.length + results.tasks.length + results.tickets.length) > 0

  return (
    <CommandDialog
      open={open}
      onOpenChange={(next) => (next ? setPaletteOpen(true) : close())}
      title="Recherche"
      description="Chercher un projet, un client, une tâche, un ticket, ou lancer une action."
      className="sm:max-w-[640px]"
    >
      <Command shouldFilter={deferred.length < 2 || !hasResults} loop>
        <CommandInput
          value={query}
          onValueChange={setQuery}
          placeholder="Chercher un projet, un client, une tâche… ou taper une action"
        />
        <CommandList className="max-h-[420px]">
          <CommandEmpty>
            {deferred.length < 2
              ? 'Tapez au moins deux caractères.'
              : isFetching
                ? 'Recherche…'
                : 'Rien ne correspond.'}
          </CommandEmpty>

          {results !== undefined && results.projects.length > 0 && (
            <CommandGroup heading="Projets">
              {results.projects.map((p) => (
                <CommandItem key={p.id} value={`projet-${p.id}`} onSelect={() => go('/pm/projets/$id', { id: p.id })} className="gap-2.5">
                  {p.logo_url == null ? (
                    <HugeiconsIcon icon={Folder01Icon} size={16} strokeWidth={1.6} className="shrink-0 text-[#8d8d8d]" />
                  ) : (
                    <ProjectLogo url={p.logo_url} size={20} className="rounded-[5px]" />
                  )}
                  <span className="min-w-0 flex-1 truncate">{p.name}</span>
                  <span className="truncate text-[12px] text-[#a2a3a7]">{p.client_name}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          )}

          {results !== undefined && results.clients.length > 0 && (
            <CommandGroup heading="Clients">
              {results.clients.map((c) => (
                <CommandItem key={c.id} value={`client-${c.id}`} onSelect={() => go('/crm/clients/$id', { id: c.id })} className="gap-2.5">
                  <HugeiconsIcon icon={c.kind === 'particulier' ? UserIcon : Building03Icon} size={16} strokeWidth={1.6} className="shrink-0 text-[#8d8d8d]" />
                  <span className="min-w-0 flex-1 truncate">{c.name}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          )}

          {results !== undefined && results.contacts.length > 0 && (
            <CommandGroup heading="Contacts">
              {results.contacts.map((c) => (
                <CommandItem
                  key={c.id}
                  value={`contact-${c.id}`}
                  onSelect={() =>
                    c.client_id == null
                      ? go('/crm/contacts', undefined, { search: `${c.firstname} ${c.lastname}`.trim(), page: 1 })
                      : go('/crm/clients/$id', { id: c.client_id })
                  }
                  className="gap-2.5"
                >
                  <HugeiconsIcon icon={Contact01Icon} size={16} strokeWidth={1.6} className="shrink-0 text-[#8d8d8d]" />
                  <span className="min-w-0 flex-1 truncate">{`${c.firstname} ${c.lastname}`.trim()}</span>
                  <span className="truncate text-[12px] text-[#a2a3a7]">{c.client_name !== '' ? c.client_name : (c.email ?? '')}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          )}

          {results !== undefined && results.tasks.length > 0 && (
            <CommandGroup heading="Tâches">
              {results.tasks.map((t) => (
                <CommandItem key={t.id} value={`tache-${t.id}`} onSelect={() => go('/pm/taches', undefined, { tache: t.id, page: 1 })} className="gap-2.5">
                  <HugeiconsIcon icon={CheckmarkSquare02Icon} size={16} strokeWidth={1.6} className="shrink-0 text-[#8d8d8d]" />
                  <span className={`min-w-0 flex-1 truncate ${t.status === 'done' ? 'text-[#8d8d8d] line-through' : ''}`}>{t.title}</span>
                  <span className="truncate text-[12px] text-[#a2a3a7]">{t.project_name}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          )}

          {results !== undefined && results.tickets.length > 0 && (
            <CommandGroup heading="Tickets">
              {results.tickets.map((t) => (
                <CommandItem key={t.id} value={`ticket-${t.id}`} onSelect={() => go('/pm/tickets/$id', { id: t.id })} className="gap-2.5">
                  <HugeiconsIcon icon={Ticket02Icon} size={16} strokeWidth={1.6} className="shrink-0 text-[#8d8d8d]" />
                  <span className="shrink-0 text-[12px] text-[#a2a3a7] tabular-nums">#{t.numero}</span>
                  <span className="min-w-0 flex-1 truncate">{t.subject}</span>
                  <span className="truncate text-[12px] text-[#a2a3a7]">{t.project_name}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          )}

          {hasResults && <CommandSeparator />}

          {creations.length > 0 && (
            <CommandGroup heading="Créer">
              {creations.map((c) => (
                <CommandItem key={c.kind} value={`creer ${c.label}`} onSelect={() => create(c.kind, c.to)} className="gap-2.5">
                  <HugeiconsIcon icon={c.icon} size={16} strokeWidth={1.6} className="shrink-0 text-brand" />
                  {c.label}
                </CommandItem>
              ))}
            </CommandGroup>
          )}

          <CommandGroup heading="Aller à">
            {destinations.map((d) => (
              <CommandItem key={d.to} value={`aller ${d.module} ${d.label}`} onSelect={() => go(d.to)} className="gap-2.5">
                {d.icon !== undefined ? (
                  <HugeiconsIcon icon={d.icon} size={16} strokeWidth={1.6} className="shrink-0 text-[#8d8d8d]" />
                ) : (
                  <span className="size-4 shrink-0" />
                )}
                <span className="min-w-0 flex-1 truncate">{d.label}</span>
                <span className="truncate text-[12px] text-[#a2a3a7]">{d.module}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      </Command>
    </CommandDialog>
  )
}

/** Un menu deroulant donne ses feuilles ; une entree simple se donne elle-meme. */
function leavesOf(item: MenuItem, module: string): { to: string; label: string; module: string; icon?: typeof PlusSignIcon }[] {
  if ('to' in item && typeof item.to === 'string') {
    return [{ to: item.to, label: item.label, module, icon: item.icon }]
  }
  if ('children' in item && Array.isArray(item.children)) {
    return item.children.map((leaf: { to: string; label: string }) => ({
      to: leaf.to,
      label: `${item.label} · ${leaf.label}`,
      module,
      icon: item.icon,
    }))
  }
  return []
}

function typing(event: KeyboardEvent): boolean {
  const target = event.target as HTMLElement | null
  if (target === null) return false
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable
}
