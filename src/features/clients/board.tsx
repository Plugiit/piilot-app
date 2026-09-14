import { Contact01Icon, Folder01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { motion } from 'framer-motion'
import { useRef, useState } from 'react'

import { CLIENT_STATUS, CLIENT_STATUS_ORDER } from '@/features/clients/format'
import { Avatars } from '@/features/projects/ui'
import { cn } from '@/lib/utils'
import type { ClientStatus, CrmClient } from '@/types/api'

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
  onGrab: () => void
  onCarry: (point: { x: number; y: number }) => void
  onRelease: (point: { x: number; y: number } | null) => void
}) {
  // Un glisser se termine par un clic que le navigateur envoie quand meme : le
  // relachement au-dessus du nom declencherait sinon sa navigation.
  const carried = useRef(false)

  return (
    <motion.article
      drag
      // La carte revient d'elle-meme si elle est laissee hors d'une colonne.
      dragSnapToOrigin
      dragElastic={0.12}
      dragMomentum={false}
      whileDrag={{ scale: 1.03, boxShadow: '0 12px 28px -8px rgb(16 24 40 / 0.28)', zIndex: 40 }}
      onDragStart={() => {
        carried.current = true
        onGrab()
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
        <span className="flex items-center gap-1 text-[12px] text-[#73757c]">
          <HugeiconsIcon icon={Folder01Icon} size={14} strokeWidth={1.6} />
          {client.projects_active}
        </span>

        {client.account_manager !== null && (
          <Avatars people={[client.account_manager]} max={1} size={20} />
        )}
      </div>
    </motion.article>
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
  const [dragging, setDragging] = useState<string | null>(null)
  const [over, setOver] = useState<ClientStatus | null>(null)

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

    onMove(client, target)
  }

  return (
    <div className="flex min-h-0 flex-1 gap-3 overflow-x-auto p-4">
      {CLIENT_STATUS_ORDER.map((status) => {
        const column = clients.filter((client) => client.status === status)
        const tint = CLIENT_STATUS[status]!

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
              over === status && dragging !== null ? 'border-brand' : 'border-transparent',
            )}
          >
            <header className="flex items-center gap-2 px-2 py-1.5">
              <span className="size-2 shrink-0 rounded-full" style={{ background: tint.color }} />
              <h2 className="font-heading text-[14px] font-medium text-[#1b1b1b]">{tint.label}</h2>
              <span className="text-[12px] text-[#73757c]">{column.length}</span>
            </header>

            <div className="flex flex-col gap-2">
              {column.length === 0 ? (
                <p className="px-2 py-6 text-center text-[13px] text-[#a2a3a7]">Aucun client</p>
              ) : (
                column.map((client) => (
                  <ClientCard
                    key={client.id}
                    client={client}
                    onGrab={() => setDragging(client.id)}
                    onCarry={(point) => setOver(columnAt(point))}
                    onRelease={(point) => release(client, point)}
                  />
                ))
              )}
            </div>
          </section>
        )
      })}
    </div>
  )
}
