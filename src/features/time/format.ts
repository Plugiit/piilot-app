/**
 * Lecture d'une duree saisie a la main.
 *
 * Personne ne tape « 90 » en pensant « une heure et demie » : on ecrit 1h30,
 * ou 1,5, ou 90 selon l'humeur et le moment. Les trois formes mènent au meme
 * nombre de minutes, parce qu'exiger la bonne serait le meilleur moyen de
 * faire abandonner la saisie.
 *
 * Rend null quand rien d'exploitable n'est reconnu : l'appelant affiche alors
 * l'erreur plutot que d'enregistrer un nombre invente.
 */
export function parseDuration(raw: string): number | null {
  const value = raw.trim().toLowerCase().replace(',', '.')

  if (value === '') return null

  // « 1h30 », « 1 h 30 », « 2h », « h30 » n'existe pas : l'heure precede.
  const composite = /^(\d+)\s*h\s*(\d+)?$/.exec(value)
  if (composite !== null) {
    const heures = Number(composite[1])
    const minutes = composite[2] === undefined ? 0 : Number(composite[2])

    // « 1h75 » n'est pas une duree : au-dela de 59, c'est une faute de frappe
    // qu'il vaut mieux signaler que reinterpreter.
    if (minutes > 59) return null

    return heures * 60 + minutes
  }

  // « 1.5 » et « 1,5 » : des heures decimales. Arrondi a la minute, la seule
  // unite que la base connait.
  const decimal = /^(\d+)\.(\d+)$/.exec(value)
  if (decimal !== null) {
    return Math.round(Number(value) * 60)
  }

  // « 90 » : des minutes, tout simplement. C'est la forme la plus courte, donc
  // celle qu'on tape le plus souvent.
  const entier = /^(\d+)$/.exec(value)
  if (entier !== null) {
    return Number(value)
  }

  return null
}

/**
 * Ecriture d'une duree, pour l'affichage.
 *
 * « 1h30 » plutot que « 1,5 h » : c'est ainsi qu'on parle d'un temps passe, et
 * les minutes rondes se lisent sans conversion mentale.
 */
export function formatDuration(minutes: number): string {
  if (minutes <= 0) return '0h'

  const heures = Math.floor(minutes / 60)
  const reste = minutes % 60

  if (heures === 0) return `${reste} min`
  if (reste === 0) return `${heures}h`

  return `${heures}h${String(reste).padStart(2, '0')}`
}
