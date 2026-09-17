import type { StatusPill } from '@/features/projects/format'
import type { DeliverableStatus } from '@/types/api'

/**
 * Libelles francais des etats d'un livrable.
 *
 * Meme partage que pour les tickets : l'API renvoie des cles stables et la
 * traduction vit ici, pour qu'un intitule reformule ne demande pas de toucher
 * au contrat.
 *
 * Les teintes reprennent celles des tickets : l'attente est neutre, un retour
 * se lit comme une anomalie, une validation comme un statut termine.
 */
export const DELIVERABLE_STATUS_ORDER: DeliverableStatus[] = [
  'brouillon',
  'en_attente',
  'retours',
  'valide',
]

export const DELIVERABLE_STATUS: Record<DeliverableStatus, { label: string; pill: StatusPill }> = {
  brouillon: { label: 'Brouillon', pill: { bg: '#f3f4f4', border: '#e3e4e6', text: '#8a8c91' } },
  en_attente: { label: 'En attente', pill: { bg: '#fff1d4', border: '#ffe8b7', text: '#9a6a00' } },
  retours: { label: 'Retours', pill: { bg: '#ffe8ec', border: '#ffd0d8', text: '#a30f2c' } },
  valide: { label: 'Validé', pill: { bg: '#dcf7ea', border: '#afe3ca', text: '#006f1f' } },
}

/**
 * Depuis combien de temps une version attend une reponse.
 *
 * Calculee a l'affichage et non renvoyee par l'API : une duree servie par le
 * serveur serait fausse des la seconde suivante. C'est le meme parti que
 * l'anciennete d'une carte du kanban des clients.
 *
 * Rend une chaine vide sous vingt-quatre heures : « 0 jour » se lirait comme
 * une donnee manquante alors que la demande vient de partir.
 */
export function attenteDepuis(submittedAt: string): string {
  const jours = Math.floor((Date.now() - new Date(submittedAt).getTime()) / 86_400_000)

  if (jours < 1) return ''
  if (jours === 1) return '1 jour'

  return `${jours} jours`
}
