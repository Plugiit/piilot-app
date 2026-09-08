/**
 * Projets figes, en attendant que `GET /api/v1/admin/projects` reponde.
 *
 * Ce fichier est la specification de l'endpoint autant que le decor de
 * l'ecran : ce qu'une ligne porte ici est exactement ce que la vue affiche,
 * donc exactement ce que la route devra renvoyer — ni un champ de plus, ni un
 * appel de second niveau pour aller chercher l'equipe ou les heures.
 *
 * Il disparait le jour ou l'endpoint existe.
 */

/**
 * Statuts du cycle de vie d'un projet.
 *
 * Quatre, pas dix : au-dela, une pastille de couleur ne se lit plus et la
 * barre de filtres deborde. Le retard et le depassement n'en sont pas — ce
 * sont des signaux qui se superposent a n'importe quel statut.
 */
export type ProjectStatus = 'cadrage' | 'production' | 'attente' | 'livre'

/** Couleurs reprises du tableau de bord : meme vocabulaire d'un ecran a l'autre. */
export const STATUS: Record<ProjectStatus, { label: string; color: string }> = {
  cadrage: { label: 'Cadrage', color: '#4956f4' },
  production: { label: 'Production', color: '#0db471' },
  attente: { label: 'Attente client', color: '#eab308' },
  livre: { label: 'Livré', color: '#c4c4c4' },
}

/** Ordre d'affichage des filtres — celui du cycle de vie, pas l'alphabetique. */
export const STATUS_ORDER: ProjectStatus[] = ['cadrage', 'production', 'attente', 'livre']

export interface Project {
  id: string
  name: string
  client: string
  status: ProjectStatus
  /** Avancement declare, de 0 a 100. */
  progress: number
  /** Heures saisies et heures vendues : leur rapport dit le depassement. */
  hoursSpent: number
  hoursSold: number
  /** Echeance de la prochaine livraison attendue. */
  due: Date
  team: { initials: string; name: string; tint: string }[]
}

/**
 * Memes personnes, memes initiales et memes teintes que la carte « Équipe
 * aujourd'hui ». Le nom accompagne les initiales : une pile d'avatars se lit
 * en initiales, un menu de filtre demande un nom.
 */
const P = {
  MM: { initials: 'MM', name: 'Maxence M.', tint: '#e9dcb3' },
  UL: { initials: 'UL', name: 'Ugo L.', tint: '#cfd4c7' },
  CR: { initials: 'CR', name: 'Camille R.', tint: '#e0dad5' },
  AB: { initials: 'AB', name: 'Adrien B.', tint: '#dcd7e0' },
  TB: { initials: 'TB', name: 'Théo B.', tint: '#e0e0e0' },
} as const

/**
 * Echeance exprimee en jours depuis aujourd'hui, puis convertie en date.
 *
 * Des dates absolues figeraient la maquette : passe le mois prochain, elle
 * n'afficherait plus qu'un mur de retards, et le rouge ne voudrait plus rien
 * dire. Le decalage garde le melange voulu — quelques retards, des echeances
 * proches, le reste au large.
 */
function inDays(days: number) {
  const date = new Date()
  date.setHours(0, 0, 0, 0)
  date.setDate(date.getDate() + days)

  return date
}

export const PROJECTS: Project[] = [
  {
    id: 'refonte-vitrine',
    name: 'Refonte vitrine',
    client: 'Eric HDK',
    status: 'production',
    progress: 68,
    hoursSpent: 82,
    hoursSold: 96,
    due: inDays(9),
    team: [P.MM, P.UL, P.CR],
  },
  {
    id: 'application-mobile',
    name: 'Application mobile',
    client: 'Novaterre',
    status: 'production',
    progress: 54,
    hoursSpent: 148,
    hoursSold: 120,
    due: inDays(-3),
    team: [P.MM, P.AB],
  },
  {
    id: 'portail-client',
    name: 'Portail client',
    client: 'Groupe Vallier',
    status: 'attente',
    progress: 90,
    hoursSpent: 61,
    hoursSold: 80,
    due: inDays(2),
    team: [P.AB, P.CR],
  },
  {
    id: 'identite-visuelle',
    name: 'Identité visuelle',
    client: 'Atelier Bonnet',
    status: 'cadrage',
    progress: 12,
    hoursSpent: 8,
    hoursSold: 45,
    due: inDays(21),
    team: [P.UL],
  },
  {
    id: 'site-institutionnel',
    name: 'Site institutionnel',
    client: 'Fondation Lemaire',
    status: 'production',
    progress: 41,
    hoursSpent: 57,
    hoursSold: 140,
    due: inDays(34),
    team: [P.CR, P.TB, P.MM, P.UL],
  },
  {
    id: 'vitrine-holding',
    name: 'Site vitrine holding',
    client: 'Marceau Holding',
    status: 'attente',
    progress: 76,
    hoursSpent: 39,
    hoursSold: 40,
    due: inDays(-11),
    team: [P.UL, P.TB],
  },
  {
    id: 'catalogue-produits',
    name: 'Catalogue produits',
    client: 'Sereine Cosmétiques',
    status: 'cadrage',
    progress: 5,
    hoursSpent: 3,
    hoursSold: 60,
    due: inDays(48),
    team: [P.CR],
  },
  {
    id: 'intranet-rh',
    name: 'Intranet RH',
    client: 'Groupe Vallier',
    status: 'production',
    progress: 33,
    hoursSpent: 96,
    hoursSold: 180,
    due: inDays(27),
    team: [P.MM, P.TB],
  },
  {
    id: 'refonte-ecommerce',
    name: 'Refonte e-commerce',
    client: 'Maison Delcour',
    status: 'production',
    progress: 62,
    hoursSpent: 210,
    hoursSold: 200,
    due: inDays(5),
    team: [P.MM, P.CR, P.AB, P.UL, P.TB],
  },
  {
    id: 'landing-campagne',
    name: 'Landing campagne été',
    client: 'Novaterre',
    status: 'livre',
    progress: 100,
    hoursSpent: 22,
    hoursSold: 25,
    due: inDays(-24),
    team: [P.UL, P.CR],
  },
  {
    id: 'charte-editoriale',
    name: 'Charte éditoriale',
    client: 'Fondation Lemaire',
    status: 'livre',
    progress: 100,
    hoursSpent: 34,
    hoursSold: 30,
    due: inDays(-38),
    team: [P.AB],
  },
  {
    id: 'application-terrain',
    name: 'Application terrain',
    client: 'Bertaud TP',
    status: 'cadrage',
    progress: 18,
    hoursSpent: 14,
    hoursSold: 90,
    due: inDays(16),
    team: [P.MM, P.AB],
  },
  {
    id: 'espace-adherents',
    name: 'Espace adhérents',
    client: 'Union Sportive',
    status: 'attente',
    progress: 84,
    hoursSpent: 71,
    hoursSold: 75,
    due: inDays(1),
    team: [P.CR, P.TB],
  },
  {
    id: 'refonte-newsletter',
    name: 'Refonte newsletter',
    client: 'Maison Delcour',
    status: 'livre',
    progress: 100,
    hoursSpent: 17,
    hoursSold: 20,
    due: inDays(-52),
    team: [P.UL],
  },
]

/* -------------------------------------------------------------------------
 * Detail d'un projet.
 *
 * Les jalons, livrables, tickets et l'activite sont communs a tous les
 * projets : les decliner quatorze fois donnerait quatorze fois le meme travail
 * d'invention sans rien apprendre de plus sur l'ecran. Ce qui distingue un
 * projet d'un autre — heures, avancement, echeance, equipe, client — vient de
 * `PROJECTS` et traverse la page.
 *
 * Les dates sont relatives a aujourd'hui pour la meme raison que les
 * echeances : figees, elles feraient d'une frise de jalons un mur de retards
 * des le mois prochain.
 * ------------------------------------------------------------------------- */

export type MilestoneState = 'done' | 'current' | 'todo'

export interface Milestone {
  label: string
  state: MilestoneState
  /** Jours par rapport a aujourd'hui. */
  offset: number
}

export const MILESTONES: Milestone[] = [
  { label: 'Cadrage validé', state: 'done', offset: -42 },
  { label: 'Maquettes livrées', state: 'done', offset: -21 },
  { label: 'Intégration', state: 'current', offset: 4 },
  { label: 'Recette client', state: 'todo', offset: 18 },
  { label: 'Mise en production', state: 'todo', offset: 31 },
]

/**
 * Etats d'un livrable.
 *
 * `review` est l'etat propre au portail client : le livrable est parti chez le
 * client et attend son avis. C'est le seul que l'agence ne peut pas faire
 * avancer seule, d'ou sa couleur d'attente.
 */
export const DELIVERABLE_STATE = {
  draft: { label: 'En cours', color: '#4956f4' },
  review: { label: 'Chez le client', color: '#eab308' },
  approved: { label: 'Validé', color: '#0db471' },
  rejected: { label: 'À reprendre', color: '#e5484d' },
} as const

export type DeliverableState = keyof typeof DELIVERABLE_STATE

export const DELIVERABLES: { name: string; state: DeliverableState; offset: number }[] = [
  { name: 'Charte graphique', state: 'approved', offset: -28 },
  { name: 'Maquettes desktop', state: 'approved', offset: -14 },
  { name: 'Maquettes mobile', state: 'review', offset: -2 },
  { name: 'Intégration — page d’accueil', state: 'draft', offset: 6 },
  { name: 'Rédaction des contenus', state: 'rejected', offset: -5 },
]

export const TICKETS = { open: 3, waiting: 1, closed: 12 }

export const ACTIVITY: { who: string; what: string; target: string; offset: number }[] = [
  { who: 'Camille R.', what: 'a validé le livrable', target: 'Maquettes desktop', offset: -1 },
  { who: 'Client', what: 'a demandé une reprise sur', target: 'Rédaction des contenus', offset: -2 },
  { who: 'Maxence M.', what: 'a saisi 3 h 10 sur', target: 'Intégration', offset: -2 },
  { who: 'Ugo L.', what: 'a ouvert le ticket', target: 'Formulaire de contact', offset: -4 },
  { who: 'Adrien B.', what: 'a franchi le jalon', target: 'Maquettes livrées', offset: -21 },
]

/** Contact cote client — le portail lui est destine. */
export const CONTACT = { name: 'Hélène Vasseur', role: 'Directrice communication' }

/* -------------------------------------------------------------------------
 * Taches.
 *
 * Meme parti que les jalons et les livrables : un jeu commun a tous les
 * projets. Ce qui varie d'un projet a l'autre, c'est qui les porte — les
 * affectations sont tirees de `project.team`, si bien qu'un projet a une
 * personne ne montre pas les cinq avatars d'un autre.
 *
 * `offset` plutot qu'une date, pour la meme raison qu'ailleurs dans ce
 * fichier : des echeances figees feraient un mur de retards le mois prochain.
 * ------------------------------------------------------------------------- */

/**
 * Colonnes du tableau.
 *
 * Quatre et non trois : entre « en cours » et « termine » vit la revue, le
 * moment ou le travail est fait mais pas encore accepte. C'est la colonne ou
 * s'accumulent les taches d'une agence, celle qu'un tableau a trois colonnes
 * cache dans « en cours » — et avec elle, le vrai goulot.
 */
export type TaskStatus = 'todo' | 'progress' | 'review' | 'done'

export const TASK_STATUS: Record<TaskStatus, { label: string; color: string }> = {
  todo: { label: 'À faire', color: '#c4c4c4' },
  progress: { label: 'En cours', color: '#ff782b' },
  review: { label: 'En revue', color: '#eab308' },
  done: { label: 'Terminé', color: '#0db471' },
}

/** Ordre des colonnes : celui du flux de travail, de gauche a droite. */
export const TASK_STATUS_ORDER: TaskStatus[] = ['todo', 'progress', 'review', 'done']

export interface Task {
  id: string
  title: string
  status: TaskStatus
  /** Nature de la tache, portee par la carte comme par le tiroir. */
  tag: string
  /** Echeance en jours depuis aujourd'hui. */
  offset: number
  /** Estimation en heures : elle donne la charge d'une colonne, pas seulement son compte. */
  hours: number
}

export const TASKS: Task[] = [
  { id: 'specs-api', title: 'Rédiger les specs de l’API', status: 'done', tag: 'Technique', offset: -18, hours: 6 },
  { id: 'charte-graphique', title: 'Charte graphique', status: 'done', tag: 'Design', offset: -28, hours: 12 },
  { id: 'maquettes-desktop', title: 'Maquettes desktop', status: 'done', tag: 'Design', offset: -14, hours: 20 },
  { id: 'arborescence', title: 'Arborescence et wireframes', status: 'done', tag: 'Design', offset: -22, hours: 8 },

  { id: 'maquettes-mobile', title: 'Maquettes mobile', status: 'review', tag: 'Design', offset: -2, hours: 14 },
  { id: 'contenus-rubriques', title: 'Reprise des contenus de rubriques', status: 'review', tag: 'Contenu', offset: -5, hours: 9 },
  { id: 'recette-mobile', title: 'Recette mobile', status: 'review', tag: 'Recette', offset: 1, hours: 5 },

  { id: 'integration-accueil', title: 'Intégration de la page d’accueil', status: 'progress', tag: 'Intégration', offset: 6, hours: 16 },
  { id: 'tunnel-commande', title: 'Tunnel de commande', status: 'progress', tag: 'Intégration', offset: 9, hours: 24 },
  { id: 'formulaire-contact', title: 'Formulaire de contact', status: 'progress', tag: 'Intégration', offset: 3, hours: 4 },
  { id: 'migration-contenus', title: 'Migration des contenus', status: 'progress', tag: 'Contenu', offset: 12, hours: 11 },

  { id: 'accessibilite', title: 'Tests d’accessibilité', status: 'todo', tag: 'Recette', offset: 15, hours: 7 },
  { id: 'suivi-analytics', title: 'Mise en place du suivi analytics', status: 'todo', tag: 'Technique', offset: 18, hours: 3 },
  { id: 'optimisation-images', title: 'Optimisation des images', status: 'todo', tag: 'Technique', offset: 20, hours: 4 },
  { id: 'redirections-seo', title: 'Plan de redirections', status: 'todo', tag: 'SEO', offset: 22, hours: 5 },
  { id: 'environnement-recette', title: 'Environnement de recette', status: 'todo', tag: 'Technique', offset: 10, hours: 6 },
  { id: 'pages-legales', title: 'Pages légales', status: 'todo', tag: 'Contenu', offset: 25, hours: 2 },
  { id: 'formation-client', title: 'Formation du client', status: 'todo', tag: 'Recette', offset: 30, hours: 4 },
]

/**
 * Affecte les taches a l'equipe du projet, en tourniquet.
 *
 * Un tirage au hasard changerait a chaque rendu — une carte ne peut pas
 * changer de main parce qu'on a bouge une autre carte. Le tourniquet donne un
 * resultat stable, reparti, et qui suit l'equipe reellement affectee.
 */
export function tasksOf(project: Project): (Task & { assignee: Project['team'][number] })[] {
  return TASKS.map((task, index) => ({
    ...task,
    assignee: project.team[index % project.team.length]!,
  }))
}
