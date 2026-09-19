import { LinkSquare02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { createFileRoute } from '@tanstack/react-router'

import coolifyUrl from '@/assets/sidebar/app-coolify.svg'
import driveUrl from '@/assets/sidebar/app-google-drive.svg'
import proxmoxUrl from '@/assets/sidebar/app-proxmox.svg'
import uptimeUrl from '@/assets/sidebar/app-uptime-kuma.svg'
import calendarUrl from '@/assets/integrations/google-calendar.svg'
import figmaUrl from '@/assets/integrations/figma.svg'
import githubUrl from '@/assets/integrations/github.svg'
import slackUrl from '@/assets/integrations/slack.svg'

export const Route = createFileRoute('/_app/compte/integrations')({
  component: IntegrationsPage,
})

/**
 * Une entree de la grille.
 *
 * `href` distingue les deux etats : un outil que l'agence utilise deja mene
 * quelque part, les autres attendent d'etre branches. Rien de plus a stocker
 * pour l'instant — aucune de ces integrations n'echange de jeton, et une
 * colonne « connecte » qui ne refleterait rien vaut moins que son absence.
 */
interface Integration {
  name: string
  logo: string
  description: string
  href?: string
}

/**
 * Les outils deja en place, ceux du rail.
 *
 * Ils ne sont pas « connectes » au sens d'un echange de jetons : ce sont les
 * services que l'agence heberge ou utilise, et la carte mene a leur interface.
 * C'est ce que la page peut promettre aujourd'hui sans mentir.
 */
const CONNECTED: Integration[] = [
  {
    name: 'Google Drive',
    logo: driveUrl,
    description:
      'Les documents de l’agence : devis, comptes rendus, livrables. Le dossier partagé s’ouvre directement depuis le rail.',
    href: 'https://drive.google.com',
  },
  {
    name: 'Coolify',
    logo: coolifyUrl,
    description:
      'Les déploiements des projets clients. Chaque mise en production passe par là, avec ses journaux et ses variables.',
    href: 'https://cool.plugiit.com',
  },
  {
    name: 'Uptime Kuma',
    logo: uptimeUrl,
    description:
      'La surveillance des sites livrés. Une sonde par site, et l’historique des interruptions quand un client s’en inquiète.',
    href: 'https://uptime.plugiit.com',
  },
  {
    name: 'Proxmox',
    logo: proxmoxUrl,
    description:
      'Les machines qui portent l’hébergement. Réservé à l’administration du parc, pas au travail courant.',
    href: 'https://prox.plugiit.com',
  },
]

/**
 * Ce qui reste a brancher.
 *
 * Ces quatre-la sont ceux qui serviraient vraiment a une agence web : les
 * maquettes, le code, les echanges d'equipe et l'agenda. La carte les montre
 * sans bouton actif — il n'y a pas encore d'echange d'authentification a
 * declencher, et un bouton « Connecter » qui ouvrirait le vide serait pire que
 * pas de bouton.
 */
const PLANNED: Integration[] = [
  {
    name: 'Figma',
    logo: figmaUrl,
    description:
      'Rattacher les maquettes à un projet, et voir leur dernière mise à jour sans quitter la fiche.',
  },
  {
    name: 'GitHub',
    logo: githubUrl,
    description:
      'Relier un dépôt à un projet pour suivre les livraisons et rapprocher les tâches des commits.',
  },
  {
    name: 'Slack',
    logo: slackUrl,
    description:
      'Porter les changements de statut et les échéances dans le canal de l’équipe, sans venir les chercher.',
  },
  {
    name: 'Google Agenda',
    logo: calendarUrl,
    description:
      'Reporter les jalons et les échéances des projets dans l’agenda partagé de l’agence.',
  },
]

function IntegrationsPage() {
  return (
    <div className="flex w-full flex-col gap-6">
      <Section
        title="Outils de l’agence"
        hint="Les services déjà en place. La carte mène à leur interface."
        items={CONNECTED}
      />

      <Section
        title="À brancher"
        hint="Prévues, mais sans échange de données pour l’instant."
        items={PLANNED}
      />
    </div>
  )
}

function Section({
  title,
  hint,
  items,
}: {
  title: string
  hint: string
  items: Integration[]
}) {
  return (
    <section className="flex w-full flex-col gap-3">
      <header className="flex flex-col gap-0.5">
        <h2 className="text-[18px] leading-[1.5] font-medium tracking-[-0.18px] text-[#1b1b1b]">
          {title}
        </h2>
        <p className="text-[14px] leading-[1.5] text-[#73757c]">{hint}</p>
      </header>

      {/* La maquette pose des cartes de 300px cote a cote ; la grille les
          repartit sur la largeur disponible plutot que de fixer trois colonnes,
          le panneau des reglages n'ayant pas toujours la meme place. */}
      <div className="grid w-full gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {items.map((item) => (
          <IntegrationCard key={item.name} item={item} />
        ))}
      </div>
    </section>
  )
}

function IntegrationCard({ item }: { item: Integration }) {
  return (
    <article className="flex flex-col gap-3 rounded-[12px] border border-[#e8e8e9] bg-white p-3">
      {/* La boite fait 48px, le logo s'y inscrit sans se deformer. Forcer les
          deux cotes de l'image ecrasait ceux qui ne sont pas carres — le
          Figma est deux tiers plus haut que large, Proxmox et Uptime Kuma ont
          des cadres plus larges que hauts. */}
      <span className="flex size-12 shrink-0 items-center justify-center">
        <img src={item.logo} alt="" className="size-full object-contain" />
      </span>

      <div className="flex flex-1 flex-col gap-1">
        <h3 className="text-[16px] leading-[1.5] font-medium text-[#1b1b1b]">{item.name}</h3>
        <p className="text-[12px] leading-[1.5] text-[#73757c]">{item.description}</p>
      </div>

      {item.href === undefined ? (
        <p className="flex h-10 items-center justify-center rounded-[12px] border border-dashed border-[#e8e8e9] text-[14px] font-medium text-[#a2a3a7]">
          Bientôt
        </p>
      ) : (
        <a
          href={item.href}
          target="_blank"
          rel="noreferrer"
          className="relative flex h-10 items-center justify-center gap-2 rounded-[12px] border border-[#e8e8e9] bg-white text-[14px] font-medium text-[#1b1b1b] shadow-[inset_0px_-2px_0px_0px_rgba(0,0,0,0.05)] transition-colors hover:bg-[#f8f8f8]"
        >
          Ouvrir
          <HugeiconsIcon icon={LinkSquare02Icon} size={16} strokeWidth={1.8} />
        </a>
      )}
    </article>
  )
}
