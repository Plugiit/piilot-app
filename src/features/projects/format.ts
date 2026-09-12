/**
 * Vocabulaire de presentation du module projet : couleurs, teintes, dates.
 *
 * Separe de `ui.tsx`, qui n'exporte que des composants : un module qui melange
 * les deux perd le rechargement a chaud, chaque modification d'une constante
 * remontant tout l'arbre au lieu de repeindre le composant touche.
 */
import type { ProjectPriority, ProjectStatus, TaskStatus } from '@/types/api'

/**
 * Couleurs des jauges du module.
 *
 * Le temps consomme etait bleu — une couleur qui n'existe nulle part ailleurs
 * dans le module, et qui tirait l'oeil sur la donnee la plus banale de la
 * rangee. Il passe a l'encre du texte : une jauge n'a pas a etre coloree pour
 * etre lue, la couleur est reservee a ce qui alerte.
 *
 * L'avancement, lui, garde l'orange de marque et vire au vert une fois
 * complet — c'est la seule jauge dont l'etat plein veut dire quelque chose.
 */
export const BILLABLE_COLOR = '#111'
export const PROGRESS_COLOR = '#ff782b'
export const DONE_COLOR = '#0db471'
export const WARN_COLOR = '#ff782b'
export const ALERT_COLOR = '#e5484d'

/**
 * Teintes des pastilles d'avatar.
 *
 * L'API ne renvoie pas de couleur par personne, et c'est normal : une teinte
 * d'affichage n'a rien a faire en base. Elle se deduit donc de l'identifiant,
 * de facon stable — la meme personne garde sa couleur d'un ecran a l'autre et
 * d'une session a l'autre, sans que rien ne soit stocke.
 */
const TINTS = ['#e9dcb3', '#cfd4c7', '#e0dad5', '#dcd7e0', '#e0e0e0', '#d8c7b6', '#b6cdd8']

export function tintOf(id: string): string {
  let hash = 0

  for (let i = 0; i < id.length; i += 1) {
    hash = (hash * 31 + id.charCodeAt(i)) >>> 0
  }

  return TINTS[hash % TINTS.length]!
}

/**
 * Convertit une date d'API (« AAAA-MM-JJ ») en Date locale.
 *
 * `new Date('2026-09-08')` serait interprete en UTC : a l'est de Greenwich,
 * une echeance lue le matin ressortirait la veille.
 */
export function parseApiDate(value: string | null | undefined): Date | null {
  if (value === null || value === undefined || value === '') return null

  const date = new Date(`${value}T00:00:00`)

  return Number.isNaN(date.getTime()) ? null : date
}

/** Jours entiers d'ici a une date, negatif une fois passee. */
export function daysUntil(date: Date): number {
  const today = new Date()
  today.setHours(0, 0, 0, 0)

  return Math.round((date.getTime() - today.getTime()) / 86_400_000)
}

/**
 * Statuts d'un projet : libelle et couleur.
 *
 * Trois ecrans les affichaient chacun avec sa propre table — la liste, le
 * chassis, la fiche. Une table unique evite qu'un statut renomme ne le soit
 * qu'a deux endroits sur trois.
 */
export interface StatusPill {
  bg: string
  border: string
  text: string
}

/**
 * Statuts d'un projet : libelle, couleur de pastille, palette de vignette.
 *
 * `color` reste la couleur pleine — jauges, points, tout ce qui n'a qu'un
 * aplat a poser. `pill` porte le triplet de la vignette de la liste, ou la
 * maquette demande un fond clair, une bordure a peine plus soutenue et un
 * texte assez sombre pour se lire dessus.
 *
 * Trois des quatre palettes sont celles du fichier de design, reprises telles
 * quelles. La quatrieme — le cadrage — n'a pas d'equivalent dans la maquette,
 * qui ne montre que trois etats : elle est composee dans le meme accord a
 * partir du bleu que le statut portait deja.
 */
export const PROJECT_STATUS: Record<
  ProjectStatus,
  { label: string; color: string; pill: StatusPill }
> = {
  cadrage: {
    label: 'Cadrage',
    color: '#4956f4',
    pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' },
  },
  production: {
    label: 'Production',
    color: '#0db471',
    pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' },
  },
  attente: {
    label: 'Attente client',
    color: '#eab308',
    pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' },
  },
  livre: {
    label: 'Livré',
    color: '#c4c4c4',
    pill: { bg: '#f3f4f4', border: '#e8e8e9', text: '#1b1b1b' },
  },
}

/**
 * Statuts d'une tache : libelle, couleur pleine, palette de vignette.
 *
 * Meme construction que celle des projets. La maquette ne montre que trois
 * etats de tache ; « en revue » n'y figure pas et reprend le bleu que le
 * module emploie deja pour ce qui attend une decision.
 */
export const TASK_STATUS: Record<
  TaskStatus,
  { label: string; color: string; pill: StatusPill }
> = {
  todo: {
    label: 'À faire',
    color: '#c4c4c4',
    pill: { bg: '#f3f4f4', border: '#e8e8e9', text: '#1b1b1b' },
  },
  progress: {
    label: 'En cours',
    color: '#ffaf00',
    pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' },
  },
  review: {
    label: 'En revue',
    color: '#4956f4',
    pill: { bg: '#eef0fe', border: '#d5dafd', text: '#3b45c9' },
  },
  done: {
    label: 'Terminé',
    color: '#0db471',
    pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' },
  },
}

/** Ordre du flux de travail, pas l'alphabetique. */
export const TASK_STATUS_ORDER: TaskStatus[] = ['todo', 'progress', 'review', 'done']

/**
 * Ton plein des vignettes de priorite.
 *
 * Contrairement aux statuts, la maquette les pose en aplat sature avec un
 * texte blanc : la priorite est le seul endroit de l'ecran ou la couleur crie.
 * Les trois valeurs sont celles du fichier de design.
 */
export const PRIORITY_TONE: Record<ProjectPriority, { label: string; bg: string }> = {
  high: { label: 'Haute', bg: '#ff4345' },
  medium: { label: 'Moyenne', bg: '#ffaf00' },
  low: { label: 'Basse', bg: '#006f1f' },
}

/**
 * Priorites d'un projet.
 *
 * La maquette les affiche toutes dans le meme gris : c'est une mention, pas
 * une alerte — seule l'echeance depassee a le droit de rougir sur la carte.
 */
export const PROJECT_PRIORITY: Record<ProjectPriority, { label: string }> = {
  high: { label: 'Haute' },
  medium: { label: 'Moyenne' },
  low: { label: 'Basse' },
}

/** Ordre du cycle de vie, pas l'alphabetique. */
export const PROJECT_STATUS_ORDER: ProjectStatus[] = [
  'cadrage',
  'production',
  'attente',
  'livre',
]
