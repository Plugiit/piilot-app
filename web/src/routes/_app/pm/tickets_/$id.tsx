import {
  Building03Icon,
  Calendar03Icon,
  Note01Icon,
  UserMultipleIcon,
} from '@hugeicons/core-free-icons'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useEffect, useRef, useState, type ReactNode } from 'react'

import { PageFrame } from '@/components/layout/page-frame'
import { PanelCard } from '@/components/panel-card'
import { Avatars, StatusPill } from '@/features/projects/ui'
import { ticketDetailQuery } from '@/features/tickets/api'
import {
  TICKET_PRIORITY,
  TICKET_STATUS,
  TICKET_STATUS_ORDER,
  TICKET_TRACKER,
  formatDateTime,
} from '@/features/tickets/format'
import { TicketRedacteur } from '@/features/tickets/redacteur'
import { TicketDiscussion, TicketHistorique } from '@/features/tickets/registre'
import { SlideTabs } from '@/features/tickets/tabs'
import { TitreModifiable } from '@/features/tickets/titre'
import type { TicketPerson } from '@/types/api'

/**
 * Fiche d'un ticket.
 *
 * Un seul appel la remplit : l'en-tete, les coordonnees et le registre arrivent
 * ensemble. C'est la regle du projet — un endpoint par vue —, et c'est ce qui
 * evite trois requetes en cascade a l'ouverture.
 *
 * L'ecran est en deux colonnes : le registre a gauche, ou tout se passe, et la
 * fiche a droite, qui ne bouge presque jamais. Les blocs de droite sont des
 * `PanelCard`, comme la fiche d'un client.
 */
export const Route = createFileRoute('/_app/pm/tickets_/$id')({
  loader: ({ context, params }) =>
    context.queryClient.query({ ...ticketDetailQuery(params.id), staleTime: 'static' }),
  component: TicketDetailPage,
})

/** Ligne de la fiche : un intitule, une valeur. */
function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-2.5 border-b border-[#f0f0f0] py-2.5 last:border-b-0">
      <span className="w-[88px] shrink-0 text-[12px] text-[#a2a3a7]">{label}</span>
      <span className="flex min-w-0 items-center gap-2 text-[13px] text-[#1b1b1b]">{children}</span>
    </div>
  )
}

/** Personne de la fiche, avec son avatar. Dit quand il n'y a personne. */
function Person({ person, absent }: { person: TicketPerson | null; absent: string }) {
  if (person === null) return <span className="text-[#a2a3a7]">{absent}</span>

  return (
    <>
      <Avatars
        people={[
          {
            id: person.id,
            firstname: person.firstname,
            lastname: person.lastname,
            initials: person.initials,
            avatar_url: person.avatar_url,
          },
        ]}
        max={1}
        size={20}
      />
      {`${person.firstname} ${person.lastname}`.trim() || 'Sans nom'}
    </>
  )
}

function TicketDetailPage() {
  const { id } = Route.useParams()
  const { data } = useQuery(ticketDetailQuery(id))

  const [vue, setVue] = useState<'discussion' | 'historique'>('discussion')

  const fin = useRef<HTMLDivElement>(null)

  // Le registre se lit du plus ancien au plus recent : ce qu'on vient ouvrir
  // voir est en bas. Arriver en haut obligerait a derouler tout l'historique
  // pour lire la derniere reponse.
  //
  // `scrollIntoView` sur une sentinelle plutot qu'un `scrollTop` calcule : le
  // conteneur qui defile appartient a `PageFrame`, et le chercher depuis ici
  // coderait en dur la structure du chassis.
  //
  // Deux moments seulement : l'ouverture de l'ecran, et l'arrivee d'une entree
  // — le compte augmente alors d'une unite.
  //
  // Pas au changement d'onglet : passer en « Historique » sert a remonter le
  // fil, et y etre ramene en bas defait le geste qu'on vient de faire. Les deux
  // vues gardent donc leur position de defilement.
  //
  // Sans animation : voir la page defiler depuis le haut a l'ouverture
  // donnerait l'impression d'un ecran qui se cherche.
  const compte = data?.entries.length ?? 0

  useEffect(() => {
    fin.current?.scrollIntoView({ block: 'end', behavior: 'instant' })
  }, [compte])

  // Le loader a deja rempli le cache : `data` est present des le premier rendu.
  if (data === undefined) return null

  const status = TICKET_STATUS[data.status]!
  const tracker = TICKET_TRACKER[data.tracker]!
  const priority = TICKET_PRIORITY[data.priority]!

  // Avancement dans le cycle : la position de l'etape dans la liste. « Annulé »
  // est hors course — c'est une sortie, pas une etape de plus.
  const step = TICKET_STATUS_ORDER.indexOf(data.status)
  const running = TICKET_STATUS_ORDER.filter((value) => value !== 'annule')

  return (
    <PageFrame
      title={`#${data.numero} — ${data.subject}`}
      trail={[{ label: 'Tickets', to: '/pm/tickets', search: { page: 1 } }]}
    >
      <div className="p-4">
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_332px] xl:items-start">
          {/* ===== Le registre, en-tete compris =====
              L'en-tete est DANS la colonne de gauche et non au-dessus de la
              grille : pose en frere au-dessus, il repoussait la fiche vers le
              bas et celle-ci ne commencait plus en haut de page. */}
          <section className="flex min-w-0 flex-col gap-4">
            {/* Coque grise plutot que carte blanche : le blanc est reserve au
                contenu — les messages —, et l'en-tete est de la chrome. Sur
                fond blanc, une carte blanche de plus se confondait avec les
                cartes du fil et ne se lisait plus comme le titre de l'ecran. */}
            <header className="border-surface-sunken bg-surface flex flex-col gap-2.5 rounded-[12px] border p-5">
              {/* Le numero entre dans le titre, a sa taille : c'est ainsi
                  qu'on designe un ticket a l'oral et dans un e-mail. Relegue
                  en petit au-dessus, il n'etait plus qu'une mention. */}
              {/* La typographie reste portee par le `h1` et non par ses seuls
                  enfants : `max-w-[48ch]` se calcule sur la taille de police de
                  l'element, et sans elle les 48 caracteres se mesuraient sur les
                  14px herites du corps — le titre se repliait a mi-largeur. */}
              <h1 className="flex max-w-[48ch] flex-wrap items-baseline gap-x-2.5 text-[26px] leading-[1.3] font-medium text-[#1b1b1b]">
                <span className="shrink-0 text-[#a2a3a7] tabular-nums">#{data.numero}</span>

                {/* Le sujet se change d'un clic, sans que rien ne bouge autour :
                    le champ occupe exactement la boite du texte. */}
                <TitreModifiable ticketId={data.id} subject={data.subject} />
              </h1>

              <div className="flex flex-wrap items-center gap-1.5">
                <StatusPill label={tracker.label} color={tracker.pill.text} pill={tracker.pill} />
                <StatusPill label={status.label} color={status.pill.text} pill={status.pill} />
                <StatusPill
                  label={priority.label}
                  color={priority.pill.text}
                  pill={priority.pill}
                />
              </div>

              <p className="text-[12.5px] text-[#73757c]">
                Déposé le <span className="tabular-nums">{formatDateTime(data.created_at)}</span>
                {data.reporter !== null &&
                  ` par ${`${data.reporter.firstname} ${data.reporter.lastname}`.trim() || 'un compte sans nom'}`}
              </p>

              {data.description !== '' && (
                <p className="max-w-[72ch] border-t border-[#e0e1e2] pt-3 text-[14px] leading-[1.6] whitespace-pre-wrap text-[#4f5059]">
                  {data.description}
                </p>
              )}
            </header>

            {/* Deux vues du meme registre : ce qui s'est dit, ce qui est
                arrive. Les separer evite qu'une discussion de dix messages
                soit hachee par les changements de statut — et inversement,
                qu'un historique se perde entre les paragraphes. */}
            <div className="flex min-w-0 flex-col gap-4">
              <div className="border-b border-[#ebebeb] px-1">
                <SlideTabs
                  label="Vue du registre"
                  value={vue}
                  onChange={setVue}
                  options={[
                    { value: 'discussion', label: 'Discussion' },
                    { value: 'historique', label: 'Historique' },
                  ]}
                />
              </div>

              {vue === 'discussion' ? (
                <>
                  <TicketDiscussion entries={data.entries} />

                  {/* Le redacteur ne parait qu'en discussion : on n'ecrit pas
                      dans un historique, il s'ecrit tout seul. */}
                  <TicketRedacteur ticket={data} />
                </>
              ) : (
                <TicketHistorique entries={data.entries} />
              )}

              {/* Sentinelle de fin de registre. Elle porte la hauteur d'une
                  ligne pour que le bas du redacteur ne colle pas au bord. */}
              <div ref={fin} aria-hidden className="h-2" />
            </div>
          </section>

          {/* ===== La fiche =====
              Collee en haut pendant qu'on descend le registre : les
              coordonnees du ticket sont ce qu'on relit en lisant le fil, et
              devoir remonter pour verifier le projet ou la priorite casse la
              lecture.

              `items-start` sur la grille donne au bloc sa marge de manoeuvre :
              sa zone de grille fait la hauteur de la colonne de gauche, et
              c'est dedans que le collant se deplace. Sans lui, le bloc
              occuperait toute la hauteur et n'aurait nulle part ou glisser.

              `top-4` reprend le `p-4` du conteneur, pour qu'il s'arrete la ou
              le contenu commence et non contre le bord.

              La hauteur est bornee et le debordement rendu : sur une fenetre
              basse, les quatre panneaux depassent l'ecran et leur bas
              deviendrait inatteignable une fois colle. */}
          <aside className="flex min-w-0 flex-col gap-3 xl:sticky xl:top-4 xl:max-h-[calc(100dvh-7rem)] xl:overflow-y-auto">
            <PanelCard icon={Note01Icon} title="AVANCEMENT">
              <div
                role="img"
                aria-label={`Étape ${step + 1} sur ${running.length} : ${status.label}`}
                className="flex gap-[3px] pt-1 pb-2"
              >
                {running.map((value, index) => (
                  <span
                    key={value}
                    className="h-1.5 flex-1 rounded-full"
                    style={{
                      background:
                        data.status === 'annule'
                          ? '#e8e8e9'
                          : index < step
                            ? '#0db471'
                            : index === step
                              ? status.pill.text
                              : '#e8e8e9',
                    }}
                  />
                ))}
              </div>
              <Row label="Statut">
                <StatusPill label={status.label} color={status.pill.text} pill={status.pill} />
              </Row>
              <Row label="Mis à jour">
                <span className="tabular-nums">{formatDateTime(data.updated_at)}</span>
              </Row>
            </PanelCard>

            <PanelCard icon={Building03Icon} title="IDENTITÉ">
              <Row label="Projet">
                <Link
                  to="/pm/projets/$id"
                  params={{ id: data.project.id }}
                  className="truncate hover:underline"
                >
                  {data.project.name}
                </Link>
              </Row>
              <Row label="Client">
                {data.client === null ? (
                  <span className="text-[#a2a3a7]">Sans client</span>
                ) : (
                  <Link
                    to="/crm/clients/$id"
                    params={{ id: data.client.id }}
                    className="truncate hover:underline"
                  >
                    {data.client.name}
                  </Link>
                )}
              </Row>
              <Row label="Tracker">
                <StatusPill label={tracker.label} color={tracker.pill.text} pill={tracker.pill} />
              </Row>
              <Row label="Priorité">
                <StatusPill
                  label={priority.label}
                  color={priority.pill.text}
                  pill={priority.pill}
                />
              </Row>
            </PanelCard>

            <PanelCard icon={UserMultipleIcon} title="PERSONNES">
              <Row label="Assigné à">
                <Person person={data.assignee} absent="À prendre" />
              </Row>
              <Row label="Déposé par">
                <Person person={data.reporter} absent="Inconnu" />
              </Row>
            </PanelCard>

            <PanelCard icon={Calendar03Icon} title="DATES">
              <Row label="Créé le">
                <span className="tabular-nums">{formatDateTime(data.created_at)}</span>
              </Row>
              <Row label="Mis à jour">
                <span className="tabular-nums">{formatDateTime(data.updated_at)}</span>
              </Row>
            </PanelCard>
          </aside>
        </div>
      </div>
    </PageFrame>
  )
}
