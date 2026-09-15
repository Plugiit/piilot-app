import { Avatars, StatusPill } from '@/features/projects/ui'
import { TICKET_PRIORITY, TICKET_STATUS, TICKET_TRACKER } from '@/features/tickets/format'
import type {
  TicketEntry,
  TicketPerson,
  TicketPriority,
  TicketStatus,
  TicketTracker,
} from '@/types/api'

/**
 * Le registre d'un ticket, en deux vues.
 *
 * « Discussion » ne montre que ce qui s'est dit, « Historique » que ce qui est
 * arrive.
 *
 * Ni bulle ni rail vertical : un fil se lit par son ordre, le dessiner
 * n'ajoute que du bruit entre les lignes. Mais les messages sont des cartes,
 * eux — poses a plat sur le fond blanc, ils se lisaient comme un seul bloc de
 * texte ou l'on ne voyait plus ou l'un finissait. Les changements d'etat
 * restent des lignes : ils se parcourent, ils ne se lisent pas.
 */

/** Nom affichable, l'un des deux champs pouvant etre vide. */
function nameOf(person: TicketPerson | null): string {
  if (person === null) return 'Compte supprimé'

  return `${person.firstname} ${person.lastname}`.trim() || 'Sans nom'
}

const DAY_TIME = new Intl.DateTimeFormat('fr-FR', {
  day: 'numeric',
  month: 'long',
  hour: '2-digit',
  minute: '2-digit',
})

/**
 * Pastille d'une valeur du journal.
 *
 * Le journal stocke des cles — `in_progress`, `critical` — et non des libelles :
 * un statut retire de la nomenclature doit continuer de s'afficher dans les
 * registres qui l'ont connu. D'ou le repli sur la cle brute.
 */
function ValuePill({ field, value }: { field: string; value: string }) {
  if (value === '') return <span className="text-[13px] text-[#a2a3a7]">personne</span>

  const tint =
    field === 'status'
      ? TICKET_STATUS[value as TicketStatus]
      : field === 'priority'
        ? TICKET_PRIORITY[value as TicketPriority]
        : field === 'tracker'
          ? TICKET_TRACKER[value as TicketTracker]
          : undefined

  if (tint === undefined) return <span className="text-[13px] text-[#1b1b1b]">{value}</span>

  return <StatusPill label={tint.label} color={tint.pill.text} pill={tint.pill} />
}

const FIELD_VERB: Record<string, string> = {
  subject: 'a renommé le ticket',
  status: 'a fait avancer le ticket',
  priority: 'a changé la priorité',
  tracker: 'a changé le tracker',
  assignee: 'a changé l’assignation',
}

/** Avatar d'une personne, ou rien quand le compte a disparu. */
function Face({ person, size }: { person: TicketPerson | null; size: number }) {
  if (person === null) return null

  return (
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
      size={size}
    />
  )
}

/**
 * Un message.
 *
 * Une carte, pour se detacher du fond blanc de la page : posee a plat, une
 * suite de messages se lisait comme un seul bloc de texte ou l'on ne voyait
 * plus ou l'un finissait.
 *
 * Tous les messages ont le meme poids, quel que soit leur auteur. Distinguer
 * l'agence du client reviendrait a dire qu'une parole compte plus que l'autre,
 * alors que le registre les enregistre a egalite — et le nom suffit a savoir
 * qui parle.
 *
 * La seule difference qui subsiste est la note interne, et elle ne dit pas qui
 * ecrit mais qui lira : le client ne la voit jamais. C'est une question de
 * destinataire, pas de rang.
 *
 * La carte est exactement celle d'un message public : c'est un trait orange
 * pose DEHORS, contre le bord gauche, qui la signale — le meme repere que
 * l'entree active de la barre laterale. Il ne mange rien a la carte, ne teinte
 * aucun paragraphe, et se voit du coin de l'oeil en parcourant la colonne.
 *
 * Il est seul a porter l'information : le titre qui le doublait a ete retire.
 * Le prix a payer est qu'il faut connaitre la convention — mais elle s'apprend
 * en une fois, et le fil y gagne de n'avoir plus qu'une etiquette par message
 * au lieu de deux.
 */
function Message({ entry }: { entry: TicketEntry }) {
  return (
    <article className="relative flex flex-col gap-2.5 rounded-[12px] border border-[#ebebeb] bg-white p-4 shadow-[0_1px_2px_0_rgb(16_24_40/0.05)]">
      {/* Meme gabarit que le repere de la barre laterale : 3px a bout droit
          arrondi, colle au bord de la surface de page.

          17px et non 16 : `left` se compte depuis la boite de REMBOURRAGE de la
          carte, donc la bordure de 1px de celle-ci s'ajoute au `p-4` de la
          page. 16 + 1 = 17, et le trait arrive pile au bord interne du cadre.

          C'est la butee : au-dela il passerait a gauche de l'origine du
          conteneur qui defile, dont l'`overflow-y` force le rognage sur l'axe
          horizontal, et il y perdrait sa premiere colonne de pixels. Le dernier
          cheveu qui reste est la bordure du cadre elle-meme, qu'on ne peut pas
          franchir depuis l'interieur. */}
      {entry.is_internal && (
        <span
          aria-hidden
          className="bg-brand absolute inset-y-0 -left-[17px] w-[3px] rounded-r-[3px]"
        />
      )}

      <header className="flex flex-wrap items-center gap-2">
        <Face person={entry.author} size={24} />

        <span className="text-[14px] font-medium text-[#1b1b1b]">{nameOf(entry.author)}</span>

        <span className="ml-auto text-[12px] text-[#a2a3a7] tabular-nums">
          {DAY_TIME.format(new Date(entry.at))}
        </span>
      </header>

      {/* `whitespace-pre-wrap` : les retours a la ligne de la saisie sont du
          sens, pas de la mise en forme accidentelle. */}
      <p className="max-w-[78ch] text-[14px] leading-[1.6] whitespace-pre-wrap text-[#111]/90">
        {entry.body}
      </p>
    </article>
  )
}

/**
 * Un changement d'etat : une ligne, rien de plus.
 *
 * Volontairement sans carte, alors qu'il cotoie des messages qui en ont une :
 * c'est ce qui permet de balayer un historique long sans confondre ce qui se
 * lit et ce qui se constate.
 */
function Event({ entry }: { entry: TicketEntry }) {
  return (
    <div className="flex flex-wrap items-center gap-1.5 px-1 py-0.5 text-[13px] text-[#73757c]">
      <Face person={entry.author} size={20} />
      <span className="font-medium text-[#1b1b1b]">{nameOf(entry.author)}</span>
      {FIELD_VERB[entry.field] ?? 'a modifié le ticket'}
      <ValuePill field={entry.field} value={entry.old_value} />
      <span className="text-[#a2a3a7]">→</span>
      <ValuePill field={entry.field} value={entry.new_value} />
      <span className="ml-auto text-[12px] text-[#a2a3a7] tabular-nums">
        {DAY_TIME.format(new Date(entry.at))}
      </span>
    </div>
  )
}

function Empty({ children }: { children: string }) {
  return <p className="py-10 text-center text-[13px] text-[#8d8d8d]">{children}</p>
}

/**
 * Disposition commune aux deux vues.
 *
 * Aucun retrait : le trait des notes internes va jusqu'au bord de la page, il
 * n'a pas besoin qu'on lui menage une gouttiere. Les cartes restent donc
 * alignees sur l'en-tete du ticket.
 */
const COLONNE = 'flex flex-col gap-3'

/** Ce qui s'est dit. */
export function TicketDiscussion({ entries }: { entries: TicketEntry[] }) {
  const messages = entries.filter((entry) => entry.kind === 'message')

  if (messages.length === 0) {
    return <Empty>Rien n’a encore été dit. Votre entrée ouvrira la discussion.</Empty>
  }

  return (
    <div className={COLONNE}>
      {messages.map((entry) => (
        <Message key={entry.id} entry={entry} />
      ))}
    </div>
  )
}

/**
 * Tout ce qui s'est passe : ce qui s'est dit ET ce qui est arrive, entrelaces
 * par ordre chronologique.
 *
 * L'historique porte les messages en plus des changements parce qu'un
 * changement seul ne s'explique pas : « passe en critique » ne veut rien dire
 * sans le message qui l'a declenche deux minutes plus tot. « Discussion » reste
 * la pour qui veut relire la conversation sans le bruit des mouvements.
 */
export function TicketHistorique({ entries }: { entries: TicketEntry[] }) {
  if (entries.length === 0) {
    return <Empty>Rien au registre pour l’instant.</Empty>
  }

  return (
    <div className={COLONNE}>
      {entries.map((entry) =>
        entry.kind === 'message' ? (
          <Message key={`m-${entry.id}`} entry={entry} />
        ) : (
          <Event key={`e-${entry.id}`} entry={entry} />
        ),
      )}
    </div>
  )
}
