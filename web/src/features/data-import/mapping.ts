import type { DataField } from '@/types/api'

/** Champ de Piilot -> en-tête du fichier. Une chaîne vide veut dire « ne pas importer ». */
export type Mapping = Record<string, string>

/**
 * Mapping à envoyer : seulement les champs reliés à une colonne, et seulement
 * si cette colonne existe encore dans le fichier. L'API refuse un en-tête
 * absent ; mieux vaut ne pas le lui envoyer qu'échouer sur un reste de brouillon.
 */
export function cleanMapping(mapping: Mapping, headers: readonly string[]): Mapping {
  const out: Mapping = {}
  for (const [key, header] of Object.entries(mapping)) {
    if (header !== '' && headers.includes(header)) out[key] = header
  }

  return out
}

/** Champs obligatoires qui n'ont pas de colonne. */
export function missingRequired(fields: readonly DataField[], mapping: Mapping): DataField[] {
  return fields.filter((f) => f.required && (mapping[f.key] ?? '') === '')
}

/**
 * Colonnes du fichier reliées à plus d'un champ : permis par l'API, mais
 * presque toujours une erreur de clic, donc signalé.
 */
export function sharedHeaders(mapping: Mapping): string[] {
  const seen = new Map<string, number>()
  for (const header of Object.values(mapping)) {
    if (header !== '') seen.set(header, (seen.get(header) ?? 0) + 1)
  }

  return [...seen].filter(([, n]) => n > 1).map(([header]) => header)
}

/** Colonnes du fichier qu'aucun champ ne lit : elles seront ignorées. */
export function unusedHeaders(headers: readonly string[], mapping: Mapping): string[] {
  const used = new Set(Object.values(mapping))

  return headers.filter((h) => !used.has(h))
}
