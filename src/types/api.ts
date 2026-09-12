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

/** Agregats du tableau de bord. */
export type DashboardSummary = components['schemas']['DashboardSummary']

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
