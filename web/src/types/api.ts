/**
 * Alias de confort au-dessus du contrat genere.
 *
 * Ce fichier s'ecrit a la main ; `./api-generated.ts` est ecrase par
 *
 *   npm run api:types
 *
 * qui le regenere depuis /openapi.json servi par plugiit-api-go. La separation
 * est deliberee : sans elle, chaque regeneration effacerait les alias et le
 * code applicatif devrait manipuler
 * `components['schemas']['User']` partout.
 *
 * Un type qui n'est PAS ici vient forcement de la spec. S'il manque, c'est
 * l'API qu'il faut completer, pas ce fichier — cote Go, un test verifie que la
 * spec decrit exactement les routes montees.
 */
import type { components, paths } from './api-generated'

export type { paths }

/** Format d'erreur unique de l'API. Brancher sur `code`, jamais sur `message`. */
export type ApiError = components['schemas']['Error']

/**
 * Identite de l'appelant.
 *
 * `role` est un `string` et non une union fermee : les roles vivent en base et
 * la liste peut s'etendre sans redeploiement. Fermer le type ici recreerait en
 * TypeScript la contrainte que le RBAC a justement supprimee.
 */
export type User = components['schemas']['User']

/** Enveloppe rendue par login, refresh et me. Les jetons sont dans les cookies. */
export type SessionResponse = components['schemas']['SessionResponse']

/** Fiche d'un ticket : en-tete, coordonnees et registre. */
export type TicketDetail = components['schemas']['TicketDetail']

/** Une entree du registre : un message ou un changement d'etat. */
export type TicketEntry = components['schemas']['TicketEntry']

/** Qui a parle ou agi sur un ticket. */
export type TicketPerson = components['schemas']['TicketPerson']

/** Ligne du tableau « Tickets ». */
export type Ticket = components['schemas']['Ticket']

/** Nature d'un ticket : anomalie, evolution ou assistance. */
export type TicketTracker = Ticket['tracker']

/** Etat d'avancement d'un ticket. */
export type TicketStatus = Ticket['status']

/** Gravite d'un ticket. */
export type TicketPriority = Ticket['priority']

/** Agregats du tableau de bord. */
export type DashboardSummary = components['schemas']['DashboardSummary']

/** Une nature de tache et ce qu'elle porte par etat, pour le graphique
 *  « Avancement des taches ». */
export type TaskProgress = components['schemas']['TaskProgress']

/** Projet tel que la vue liste l'affiche. Contrat cible lui aussi. */
export type Project = components['schemas']['Project']

/** Enveloppe de pagination des listes de projets. */
export type ProjectPage = components['schemas']['ProjectPage']

/* -------------------------------------------------------------------------
 * Module PM. Ces types viennent des endpoints reellement ecrits, pas d'un
 * contrat cible : le module projets et taches repond en base.
 * ------------------------------------------------------------------------- */

/** En-tete d'un projet : ce que le chassis affiche, quel que soit l'onglet. */
export type ProjectDetail = components['schemas']['ProjectDetail']

/** Projet etoile, tel que les raccourcis de la barre laterale l'affichent. */
export type ProjectShortcut = components['schemas']['ProjectShortcut']

/** Identite reduite : pastille d'avatar, nom dans une liste. */
export type Person = components['schemas']['Person']

/** Client de l'agence, tel que le champ du formulaire de projet le propose. */
export type Client = components['schemas']['Client']

/** Ligne du tableau « Clients » : le meme client, avec ses agregats. */
export type CrmClient = components['schemas']['CrmClient']

/** Enveloppe paginee du tableau « Clients ». */
export type CrmClientPage = components['schemas']['CrmClientPage']

/** Contact reduit : ce qu'une ligne de client montre de son interlocuteur. */
export type ContactRef = components['schemas']['ContactRef']

/** Kanban commercial : toutes les cartes, bornees. */
export type CrmClientBoard = components['schemas']['CrmClientBoard']

/** Etape du pipeline commercial, tiree de la spec plutot que reecrite. */
export type ClientStatus = NonNullable<CrmClient['status']>

/** Tout ce que la fiche d'un client affiche, en un appel. */
export type CrmClientDetail = components['schemas']['CrmClientDetail']

/** Projet du client, reduit a ce que sa fiche montre. */
export type ClientProject = components['schemas']['ClientProject']

/** Compte de portail rattache a un client. */
export type PortalAccount = components['schemas']['PortalAccount']

/** Entree d'un menu de contacts : un ContactRef qui dit s'il est encore libre. */
export type ContactOption = components['schemas']['ContactOption']

/** Ligne du tableau « Contacts ». */
export type CrmContact = components['schemas']['CrmContact']

/** Enveloppe paginee du tableau « Contacts ». */
export type CrmContactPage = components['schemas']['CrmContactPage']

/** Carte du tableau des taches. */
export type TaskSummary = components['schemas']['TaskSummary']

/** Carte de l'ecran « Taches » : une TaskSummary qui porte le nom de son projet. */
export type TaskListItem = components['schemas']['TaskListItem']

/** Enveloppe de l'ecran « Taches » : bornee, non paginee. */
export type TaskList = components['schemas']['TaskList']

/** Contenu de l'onglet « Tâches » d'un projet. */
export type TaskBoard = components['schemas']['TaskBoard']

/** Tout ce que le panneau lateral affiche a son ouverture. */
export type TaskDetail = components['schemas']['TaskDetail']

/** Ligne a cocher du panneau. */
export type Subtask = components['schemas']['Subtask']

/** Message de l'onglet « Commentaires ». */
export type TaskComment = components['schemas']['TaskComment']

/** Ligne du journal d'activite d'une tache. */
export type TaskActivity = components['schemas']['TaskActivity']

/**
 * Statuts, tires de la spec plutot que reecrits.
 *
 * L'union vient de l'enum OpenAPI : ajouter une colonne au tableau cote API
 * fait echouer la compilation du front la ou le nouveau cas n'est pas traite,
 * au lieu de le laisser passer silencieusement.
 */
export type TaskStatus = NonNullable<TaskSummary['status']>
export type ProjectStatus = NonNullable<ProjectDetail['status']>
export type TaskPriority = NonNullable<TaskSummary['priority']>
export type ProjectPriority = NonNullable<ProjectDetail['priority']>

/** Piece jointe, d'un projet ou d'une tache. Le contenu se lit par /files/{id}. */
export type Attachment = components['schemas']['Attachment']

/** Chiffres d'en-tete du module PM, avec leur evolution sur trente jours. */
export type DashboardMetric = components['schemas']['Metric']

/**
 * Une ligne du panneau de notifications.
 *
 * Nommee `AppNotification` : `Notification` est deja pris par l'API du
 * navigateur, et l'ombrer ferait passer un type pour l'autre sans que rien ne
 * le signale.
 */
export type AppNotification = components['schemas']['Notification']

export type NotificationFeed = components['schemas']['NotificationFeed']

/** Ligne de l'ecran « Livrables ». */
export type Deliverable = components['schemas']['Deliverable']

/** brouillon, en_attente, valide ou retours. */
export type DeliverableStatus = Deliverable['status']

/** Une version dans le fil d'un livrable. */
export type DeliverableEntry = components['schemas']['DeliverableEntry']

/** Prestation du referentiel de l'agence. */
export type Service = components['schemas']['Service']

/** Service porte par un projet ou une tache, reduit a sa pastille. */
export type ServiceTag = components['schemas']['ServiceTag']

/** Application jointe depuis le rail de la barre latérale. */
export type SidebarApp = components['schemas']['SidebarApp']

/** Ligne de pointage. */
export type TimeEntry = components['schemas']['TimeEntry']

/** Feuille de temps : les lignes d'une plage, leur total, et le total par jour. */
export type TimeSheet = components['schemas']['TimeSheet']
