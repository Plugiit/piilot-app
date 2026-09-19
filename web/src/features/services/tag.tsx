import { useQuery } from '@tanstack/react-query'

import { serviceOptionsQuery } from '@/features/services/api'
import { cn } from '@/lib/utils'
import type { ServiceTag as Tag } from '@/types/api'

/**
 * Pastilles des services d'un projet ou d'une tache.
 *
 * Un point de couleur et un nom, pas une etiquette pleine : le service est une
 * information de second plan a cote du statut ou de la priorite, et lui donner
 * la meme forme les mettrait en concurrence dans le regard.
 *
 * Ne rend rien quand la liste est vide : une mention « aucun service » sur tous
 * les projets internes n'apprendrait rien.
 */
export function ServicePills({ services }: { services: Tag[] }) {
  if (services.length === 0) return null

  return (
    <span className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
      {services.map((service) => (
        <span key={service.id} className="flex min-w-0 items-center gap-1.5">
          <span
            aria-hidden
            className="size-2 shrink-0 rounded-full"
            style={{ backgroundColor: service.color }}
          />
          <span className="truncate text-[14px] text-[#1b1b1b]">{service.name}</span>
        </span>
      ))}
    </span>
  )
}

/**
 * Choix des services, a plusieurs.
 *
 * Des pastilles a bascule plutot qu'un menu deroulant : un projet en porte
 * souvent deux ou trois, et un menu obligerait a le rouvrir a chaque ajout.
 * C'est aussi la forme des boutons de statut et de priorite des memes
 * formulaires.
 */
export function ServicesPicker({
  value,
  onChange,
}: {
  value: string[]
  onChange: (serviceIds: string[]) => void
}) {
  const { data } = useQuery(serviceOptionsQuery())
  const items = data?.items ?? []

  if (items.length === 0) {
    return (
      <p className="text-[13px] text-[#a2a3a7]">
        Aucun service dans le référentiel. Ajoutez-en depuis Paramètres › Services.
      </p>
    )
  }

  return (
    <div className="flex flex-wrap gap-2">
      {items.map((service) => {
        const active = value.includes(service.id)

        return (
          <button
            key={service.id}
            type="button"
            aria-pressed={active}
            onClick={() =>
              onChange(
                active ? value.filter((id) => id !== service.id) : [...value, service.id],
              )
            }
            className={cn(
              'flex cursor-pointer items-center gap-1.5 rounded-[8px] border px-2.5 py-1.5 text-[13px] transition-colors',
              active
                ? 'border-[#111] bg-[#f8f8f8] font-medium text-[#111]'
                : 'border-[#ebebeb] text-[#777] hover:bg-[#f8f8f8]',
            )}
          >
            <span
              aria-hidden
              className="size-2 shrink-0 rounded-full"
              style={{ backgroundColor: service.color }}
            />
            {service.name}
          </button>
        )
      })}
    </div>
  )
}
