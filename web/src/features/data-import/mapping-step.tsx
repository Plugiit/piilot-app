import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { DataField } from '@/types/api'

import { missingRequired, sharedHeaders, unusedHeaders, type Mapping } from './mapping'

/** Valeur du choix « ne pas importer » : Radix refuse la chaîne vide comme option. */
const NONE = '__none__'

/**
 * Correspondance des colonnes : pour chaque champ de Piilot, la colonne du
 * fichier qui l'alimente. Un champ obligatoire sans colonne bloque la suite.
 */
export function MappingStep({
  fields,
  headers,
  mapping,
  onChange,
}: {
  fields: readonly DataField[]
  headers: readonly string[]
  mapping: Mapping
  onChange: (mapping: Mapping) => void
}) {
  const missing = missingRequired(fields, mapping).map((f) => f.key)
  const shared = sharedHeaders(mapping)
  const unused = unusedHeaders(headers, mapping)

  return (
    <div className="flex flex-col gap-3">
      <ul className="flex flex-col">
        {fields.map((field) => {
          const value = mapping[field.key] ?? ''
          const invalid = missing.includes(field.key)

          return (
            <li key={field.key} className="flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-[#f3f4f4] py-2 last:border-b-0">
              <span id={`field-${field.key}`} className="w-[200px] shrink-0 text-[14px] text-[#1b1b1b]">
                {field.header}
                {field.required && <span className="ml-1 text-[#e5484d]" title="Obligatoire">*</span>}
              </span>
              <Select
                value={value === '' ? NONE : value}
                onValueChange={(next) => onChange({ ...mapping, [field.key]: next === NONE ? '' : next })}
              >
                <SelectTrigger
                  aria-labelledby={`field-${field.key}`}
                  aria-invalid={invalid}
                  className="h-9 w-[260px] text-[13px]"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NONE}>
                    <span className="text-[#73757c]">Ne pas importer</span>
                  </SelectItem>
                  {headers.map((header) => (
                    <SelectItem key={header} value={header}>
                      {header}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {invalid && <span className="text-[12px] text-[#e5484d]">Colonne obligatoire</span>}
            </li>
          )
        })}
      </ul>

      {shared.length > 0 && (
        <p className="text-[13px] text-[#a05a00]">
          {shared.length > 1 ? 'Ces colonnes alimentent' : 'Cette colonne alimente'} plusieurs champs : {shared.join(', ')}.
        </p>
      )}
      {unused.length > 0 && (
        <p className="text-[13px] text-[#73757c]">
          Colonnes ignorées : {unused.join(', ')}.
        </p>
      )}
    </div>
  )
}
