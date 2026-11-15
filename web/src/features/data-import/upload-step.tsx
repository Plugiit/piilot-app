import { useRef, useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { fileSize } from '@/features/portal/format'
import { fieldErrors, HttpError } from '@/lib/api'

import { useUploadImport } from './api'
import { ENTITIES, IMPORT_ORDER, type DataEntity } from './entities'

/**
 * Dépôt d'un fichier : on choisit ce qu'il contient, puis le fichier. Le
 * serveur le lit une première fois (encodage, séparateur, colonnes) et
 * l'assistant s'ouvre sur son aperçu.
 */
export function UploadStep({ onUploaded }: { onUploaded: (id: string) => void }) {
  const [entity, setEntity] = useState<DataEntity>('clients')
  const [file, setFile] = useState<File | undefined>()
  const [error, setError] = useState<string | undefined>()
  const input = useRef<HTMLInputElement>(null)
  const upload = useUploadImport()

  function submit() {
    if (file === undefined) return
    setError(undefined)
    upload.mutate(
      { entity, file },
      {
        onSuccess: (detail) => onUploaded(detail.id),
        onError: (e) => {
          const fields = fieldErrors(e)
          const message = fields.file ?? fields.entity ?? (e instanceof HttpError ? e.message : 'Dépôt impossible')
          setError(message)
          toast.error(message)
        },
      },
    )
  }

  return (
    <div className="flex flex-col gap-3">
      <p className="text-[13px] text-[#73757c]">
        Ordre recommandé : <strong className="font-medium text-[#1b1b1b]">{IMPORT_ORDER}</strong>. Un contact, un
        projet ou une saisie de temps retrouve son client ou son projet par son nom : importez-les d’abord.
      </p>

      <div className="flex flex-wrap items-end gap-3">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="import-entity" className="text-[13px]">Le fichier contient</Label>
          <Select value={entity} onValueChange={(value) => setEntity(value as DataEntity)}>
            <SelectTrigger id="import-entity" className="h-9 w-[200px] text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ENTITIES.map((e) => (
                <SelectItem key={e.value} value={e.value}>
                  {e.plural}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="import-file" className="text-[13px]">Fichier CSV</Label>
          <input
            ref={input}
            id="import-file"
            type="file"
            accept=".csv,text/csv"
            onChange={(e) => setFile(e.target.files?.[0])}
            className="text-[13px] file:mr-3 file:h-9 file:cursor-pointer file:rounded-[8px] file:border file:border-[#ebebeb] file:bg-white file:px-3 file:text-[13px]"
          />
        </div>

        <Button size="lg" disabled={file === undefined || upload.isPending} onClick={submit}>
          {upload.isPending ? 'Envoi…' : 'Déposer et continuer'}
        </Button>
      </div>

      {file !== undefined && <p className="text-[12px] text-[#777]">{file.name} · {fileSize(file.size)}</p>}
      {error !== undefined && (
        <p role="alert" className="text-[13px] text-[#e5484d]">
          {error}
        </p>
      )}
    </div>
  )
}
