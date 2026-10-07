import {
  Call02Icon,
  Clock01Icon,
  Comment01Icon,
  Contact01Icon,
  Folder01Icon,
  Mail01Icon,
  MoreHorizontalIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link, useNavigate } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'framer-motion'
import { useRef, useState } from 'react'

import { DropGap, useColumnTransition, useLanding } from '@/components/kanban-gap'
import {
  CLIENT_AGE_TINT,
  CLIENT_STATUS,
  CLIENT_STATUS_ORDER,
  clientAge,
} from '@/features/clients/format'
import { Avatars } from '@/features/projects/ui'
import { cn } from '@/lib/utils'
import type { ClientStatus, CrmClient } from '@/types/api'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { requestCreate } from '@/lib/palette'

/** Position du pointeur, quel que soit le type d'evenement rendu par le geste. */
function pointerPosition(event: MouseEvent | TouchEvent | PointerEvent) {
  if ('clientX' in event) return { x: event.clientX, y: event.clientY }

  const touch = event.changedTouches[0]

  return touch === undefined ? null : { x: touch.clientX, y: touch.clientY }
}

/** Une carte du pipeline. */
function ClientCard({
  client,
  onGrab,
  onCarry,
  onRelease,
}: {
  client: CrmClient
  /** Recoit la hauteur de la carte : c'est celle de l'emplacement a ouvrir. */
  onGrab: (height: number) => void
  onCarry: (point: { x: number; y: number }) => void
  onRelease: (point: { x: number; y: number } | null) => void
}) {
  // Un glisser se termine par un clic que le navigateur envoie quand meme : le
  // relachement au-dessus du nom declencherait sinon sa navigation.
  const carried = useRef(false)
  const self = useRef<HTMLElement>(null)

  const age = clientAge(client.status_changed_at)

  return (
    <motion.article
      ref={self}
      drag
      // La carte revient d'elle-meme si elle est laissee hors d'une colonne.
      dragSnapToOrigin
      dragElastic={0.12}
      dragMomentum={false}
      whileDrag={{ scale: 1.03, boxShadow: '0 12px 28px -8px rgb(16 24 40 / 0.28)', zIndex: 40 }}
      onDragStart={() => {
        carried.current = true
        // `offsetHeight` et non le rectangle : il ignore l'echelle de
        // `whileDrag`, qui gonflerait l'emplacement de 3 %.
        onGrab(self.current?.offsetHeight ?? 0)
      }}
      onDrag={(event) => {
        const point = pointerPosition(event)
        if (point !== null) onCarry(point)
      }}
      onDragEnd={(event) => {
        onRelease(pointerPosition(event))
        // Rendu au tour suivant : le clic de fin de geste part avant.
        window.setTimeout(() => {
          carried.current = false
        }, 0)
      }}
      className="flex cursor-grab touch-none flex-col gap-2 rounded-[10px] bg-white p-3 transition-shadow select-none hover:shadow-[0_1px_4px_0_rgb(16_24_40/0.10)] active:cursor-grabbing"
    >
      {/* `draggable={false}` n'est pas un detail : un <a> se glisse
          nativement, et le navigateur lancerait son propre glisser d'URL en
          volant le geste a `motion`. La carte deviendrait alors insaisissable
          par son nom, c'est-a-dire par l'endroit ou l'on l'attrape. */}
      <Link
        to="/crm/clients/$id"
        params={{ id: client.id }}
        draggable={false}
        onClick={(event) => {
          if (carried.current) event.preventDefault()
        }}
        className="line-clamp-2 text-[15px] leading-[1.4] font-medium text-[#1b1b1b] hover:underline"
      >
        {client.name}
      </Link>

      {client.primary_contact !== null && (
        <span className="flex items-center gap-1.5 text-[12px] text-[#73757c]">
          <HugeiconsIcon icon={Contact01Icon} size={14} strokeWidth={1.6} className="shrink-0" />
          <span className="truncate">
            {`${client.primary_contact.firstname} ${client.primary_contact.lastname}`.trim()}
          </span>
        </span>
      )}

      <div className="flex items-center justify-between gap-2 pt-1">
        <span className="flex items-center gap-2.5">
          <span className="flex items-center gap-1 text-[12px] text-[#73757c]">
            <HugeiconsIcon icon={Folder01Icon} size={14} strokeWidth={1.6} />
            {client.projects_active}
          </span>

          {/* Anciennete dans l'etape : ce qu'un pipeline sert a voir. Sans
              elle, une carte arrivee hier et une carte bloquee depuis six
              semaines se lisent pareil. */}
          <span
            title={age.title}
            className={cn(
              'flex items-center gap-1 text-[12px]',
              age.level !== 'fresh' && 'font-medium',
            )}
            style={{ color: CLIENT_AGE_TINT[age.level] }}
          >
            <HugeiconsIcon icon={Clock01Icon} size={14} strokeWidth={1.6} />
            {age.label}
          </span>
        </span>

        <span className="flex items-center gap-1">
          {client.account_manager !== null && (
            <Avatars people={[client.account_manager]} max={1} size={20} />
          )}
          <CardMenu client={client} />
        </span>
      </div>
    </motion.article>
  )
}

/**
 * Les gestes d'une carte sans ouvrir la fiche : appeler, ecrire, noter ce qui
 * s'est dit, lancer un projet. Un pipeline sert a agir sur les cartes, pas
 * seulement a les deplacer.
 */
function CardMenu({ client }: { client: CrmClient }) {
  const navigate = useNavigate()
  const email = client.primary_contact?.email ?? null

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`Actions sur ${client.name}`}
          onPointerDown={(event) => event.stopPropagation()}
          className="flex size-6 cursor-pointer items-center justify-center rounded-[6px] text-[#a2a3a7] hover:bg-[#f3f4f4] hover:text-[#1b1b1b]"
        >
          <HugeiconsIcon icon={MoreHorizontalIcon} size={16} strokeWidth={1.8} />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        {client.phone !== '' && (
          <DropdownMenuItem asChild>
            <a href={`tel:${client.phone}`}>
              <HugeiconsIcon icon={Call02Icon} size={16} strokeWidth={1.6} />
              Appeler
            </a>
          </DropdownMenuItem>
        )}
        {email !== null && (
          <DropdownMenuItem asChild>
            <a href={`mailto:${email}`}>
              <HugeiconsIcon icon={Mail01Icon} size={16} strokeWidth={1.6} />
              Écrire au contact
            </a>
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onSelect={() => void navigate({ to: '/crm/clients/$id', params: { id: client.id }, hash: 'journal' })}>
          <HugeiconsIcon icon={Comment01Icon} size={16} strokeWidth={1.6} />
          Noter une interaction
        </DropdownMenuItem>
        <DropdownMenuItem
          onSelect={() => {
            // La fiche porte la fenetre de creation, deja renseignee du client.
            requestCreate('project')
            void navigate({ to: '/crm/clients/$id', params: { id: client.id } })
          }}
        >
          <HugeiconsIcon icon={Folder01Icon} size={16} strokeWidth={1.6} />
          Nouveau projet
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/**
 * Pipeline commercial.
 *
 * Meme geste que le tableau des taches : la carte se porte a la souris, et
 * c'est le code qui dit au-dessus de quelle colonne elle est relachee. Les
 * rectangles sont mesures au moment ou l'on en a besoin plutot que memorises,
 * pour que le compte reste juste quand le tableau defile pendant le geste.
 */
export function ClientBoard({
  clients,
  onMove,
}: {
  clients: CrmClient[]
  onMove: (client: CrmClient, status: ClientStatus) => void
}) {
  const [dragging, setDragging] = useState<{ id: string; status: ClientStatus; height: number } | null>(
    null,
  )
  const [over, setOver] = useState<ClientStatus | null>(null)

  const transition = useColumnTransition()

  // Le deplacement d'un client n'est pas optimiste : la carte n'arrive qu'au
  // retour du serveur. L'emplacement qui l'attend reste ouvert jusque-la.
  const { awaiting, landedIn, height: landingHeight, land, reset } = useLanding<ClientStatus>(({ id, column }) =>
    clients.some((client) => client.id === id && client.status === column),
  )

  const columnBoxes = useRef(new Map<ClientStatus, HTMLElement>())

  function columnAt(point: { x: number; y: number }): ClientStatus | null {
    for (const [status, element] of columnBoxes.current) {
      const box = element.getBoundingClientRect()

      if (
        point.x >= box.left &&
        point.x < box.right &&
        point.y >= box.top &&
        point.y < box.bottom
      ) {
        return status
      }
    }

    return null
  }

  function release(client: CrmClient, point: { x: number; y: number } | null) {
    setDragging(null)
    setOver(null)

    if (point === null) return

    const target = columnAt(point)
    if (target === null || target === client.status) return

    land(client.id, target, dragging?.height ?? 0)
    onMove(client, target)
  }

  return (
    <div className="flex min-h-0 flex-1 gap-3 overflow-x-auto p-4">
      {CLIENT_STATUS_ORDER.map((status) => {
        const column = clients.filter((client) => client.status === status)
        const tint = CLIENT_STATUS[status]!

        // Meme regle que le tableau des taches : l'emplacement s'ouvre sous
        // une carte venue d'une autre colonne, et attend qu'elle y arrive.
        const hovered = dragging !== null && over === status && dragging.status !== status
        const gapOpen = hovered || awaiting === status
        const gapHeight = hovered ? dragging.height : landingHeight

        return (
          <section
            key={status}
            ref={(element) => {
              if (element === null) columnBoxes.current.delete(status)
              else columnBoxes.current.set(status, element)
            }}
            className={cn(
              'flex w-[280px] shrink-0 flex-col gap-2 rounded-[12px] border bg-[#f3f4f4] p-2 transition-colors',
              // La bordure reste presente mais transparente : la colorer au
              // survol ne doit pas decaler la colonne d'un pixel. Meme parti
              // que le tableau des taches.
              hovered ? 'border-brand' : 'border-transparent',
            )}
          >
            <header className="flex items-center gap-2 px-2 py-1.5">
              <span className="size-2 shrink-0 rounded-full" style={{ background: tint.color }} />
              <h2 className="font-heading text-[14px] font-medium text-[#1b1b1b]">{tint.label}</h2>
              <span className="text-[12px] text-[#73757c]">{column.length}</span>
            </header>

            <div className="flex flex-col gap-2">
              {/* « Aucun client » se replie quand l'emplacement s'ouvre, au
                  lieu de disparaitre d'un coup : la colonne vide grandit sans
                  a-coup jusqu'a la hauteur de la carte. */}
              <AnimatePresence initial={false}>
                {column.length === 0 && !gapOpen && (
                  <motion.p
                    key="empty"
                    initial={{ height: 0, opacity: 0 }}
                    animate={{ height: 'auto', opacity: 1 }}
                    exit={{ height: 0, opacity: 0 }}
                    transition={transition}
                    className="overflow-hidden px-2 text-center text-[13px] text-[#a2a3a7]"
                  >
                    <span className="block py-6">Aucun client</span>
                  </motion.p>
                )}
              </AnimatePresence>

              {/* Voir le tableau des taches : les voisines glissent pour faire
                  place, et la carte qui part laisse une place qui se referme
                  au lieu de s'effacer en fondu. */}
              <AnimatePresence initial={false}>
                {column.map((client) => (
                  <motion.div
                    key={client.id}
                    layout="position"
                    initial={{ opacity: 0, scale: 0.97 }}
                    animate={{ opacity: 1, scale: 1 }}
                    exit={{ height: 0, opacity: 0, transition: { ...transition, opacity: { duration: 0 } } }}
                    transition={transition}
                    style={{ position: 'relative', zIndex: dragging?.id === client.id ? 40 : undefined }}
                  >
                    <ClientCard
                      client={client}
                      onGrab={(height) => {
                        reset()
                        setDragging({ id: client.id, status: client.status, height })
                      }}
                      onCarry={(point) => setOver(columnAt(point))}
                      onRelease={(point) => release(client, point)}
                    />
                  </motion.div>
                ))}
              </AnimatePresence>

              <DropGap
                open={gapOpen}
                height={gapHeight}
                instant={!hovered && landedIn === status}
                spacing={8}
              />
            </div>
          </section>
        )
      })}
    </div>
  )
}
