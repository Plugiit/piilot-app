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

/** Agregats du tableau de bord. Contrat cible : l'API repond encore 501. */
export type DashboardSummary = components['schemas']['DashboardSummary']

/** Projet tel que la vue liste l'affiche. Contrat cible lui aussi. */
export type Project = components['schemas']['Project']

/** Enveloppe de pagination des listes de projets. */
export type ProjectPage = components['schemas']['ProjectPage']
