import { Delete02Icon, File01Icon, Upload04Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  projectDetailQuery,
  fileUrl,
  useDeleteProjectFile,
  useUpdateProject,
  useUploadProjectFile,
} from '@/features/projects/api'
import { PRIORITY_TONE } from '@/features/projects/format'
import { ServicesPicker } from '@/features/services/tag'
import { HttpError } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { ProjectDetail, ProjectPriority } from '@/types/api'

import { CHAMP, Card, Choices, Field, SaveBar } from '@/components/settings-ui'

/**
 * Un lien vide est permis — tous les projets n'ont pas de preproduction — mais
 * un lien saisi doit etre une adresse http(s). Le serveur applique la meme
 * regle ; la refaire ici evite un aller-retour pour une faute de frappe.
 */
const lien = z
  .string()
  .trim()
  .refine(
    (value) => value === '' || /^https?:\/\/\S+$/.test(value),
    'Adresse http(s) attendue, ou rien',
  )

const schema = z.object({
  name: z.string().trim().min(1, 'Le nom du projet est requis'),
  description: z.string().trim(),
  priority: z.enum(['low', 'medium', 'high']),
  // Vide quand le projet ne releve d'aucune prestation : un chantier interne.
  // Le formulaire envoie toujours la liste entiere, que l'API applique telle
  // quelle.
  service_ids: z.array(z.string()),
  figma_url: lien,
  prod_url: lien,
  preprod_url: lien,
})

type Values = z.infer<typeof schema>

export const Route = createFileRoute('/_app/pm/projets/$id_/parametres/')({
  component: GeneralPage,
})

const PRIORITES: { value: ProjectPriority; label: string; color: string }[] = (
  ['low', 'medium', 'high'] as const
).map((value) => ({ value, label: PRIORITY_TONE[value].label, color: PRIORITY_TONE[value].bg }))

/** Taille lisible d'une piece jointe. */
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} Ko`

  return `${(bytes / (1024 * 1024)).toFixed(1).replace('.0', '')} Mo`
}

/**
 * Depot de documents.
 *
 * Le glissement et le bouton menent au meme endroit : deposer un fichier est
 * un geste que tout le monde ne connait pas, et le bouton reste la seule voie
 * au clavier.
 */
function Documents({ project }: { project: ProjectDetail }) {
  const input = useRef<HTMLInputElement>(null)
  const [survole, setSurvole] = useState(false)
  const upload = useUploadProjectFile(project.id)
  const remove = useDeleteProjectFile(project.id)

  function envoyer(fichiers: FileList | null) {
    const fichier = fichiers?.[0]
    if (fichier === undefined) return

    upload.mutate(fichier, {
      onSuccess: () => toast.success('Document ajouté'),
      onError: (error) =>
        toast.error(error instanceof HttpError ? error.message : 'Envoi impossible'),
    })
  }

  return (
    <Field label="Déposer un document">
      <div
        onDragOver={(event) => {
          event.preventDefault()
          setSurvole(true)
        }}
        onDragLeave={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node)) setSurvole(false)
        }}
        onDrop={(event) => {
          event.preventDefault()
          setSurvole(false)
          envoyer(event.dataTransfer.files)
        }}
        className={cn(
          'flex w-full flex-col items-center justify-center gap-3 rounded-[12px] border border-dashed px-3 py-4 transition-colors',
          survole ? 'border-brand bg-[#fff7f2]' : 'border-[#d0d1d3] bg-white',
        )}
      >
        <span className="rounded-full border border-[#e8e8e9] bg-white p-2">
          <HugeiconsIcon icon={Upload04Icon} size={24} strokeWidth={1.6} />
        </span>

        <div className="flex flex-col gap-0.5 text-center">
          <p className="text-[16px] leading-[1.5] text-[#1b1b1b]">
            Glissez votre document ici pour l’envoyer.
          </p>
          <p className="text-[14px] leading-[1.5] text-[#73757c]">
            JPG, PNG ou PDF, 25 Mo au maximum.
          </p>
        </div>

        <input
          ref={input}
          type="file"
          hidden
          onChange={(event) => {
            envoyer(event.target.files)
            // Remis a zero pour que redeposer le meme fichier declenche bien
            // un nouvel evenement `change`.
            event.target.value = ''
          }}
        />

        <Button
          type="button"
          variant="outline"
          size="lg"
          disabled={upload.isPending}
          onClick={() => input.current?.click()}
        >
          {upload.isPending ? 'Envoi…' : 'Sélectionner un fichier'}
        </Button>
      </div>

      {project.files.length > 0 && (
        <ul className="mt-1 flex flex-col gap-1">
          {project.files.map((file) => (
            <li
              key={file.id}
              className="flex items-center gap-2 rounded-[10px] border border-[#e8e8e9] px-3 py-2"
            >
              <HugeiconsIcon
                icon={File01Icon}
                size={16}
                strokeWidth={1.6}
                className="shrink-0 text-[#73757c]"
              />
              <a
                href={fileUrl(file.id)}
                className="min-w-0 flex-1 truncate text-[14px] text-[#1b1b1b] underline underline-offset-2"
              >
                {file.filename}
              </a>
              <span className="shrink-0 text-[14px] text-[#73757c]">
                {formatSize(file.size_bytes)}
              </span>
              <button
                type="button"
                aria-label={`Supprimer ${file.filename}`}
                onClick={() => remove.mutate(file.id)}
                className="shrink-0 cursor-pointer text-[#a2a3a7] transition-colors hover:text-[#e5484d]"
              >
                <HugeiconsIcon icon={Delete02Icon} size={16} strokeWidth={1.6} />
              </button>
            </li>
          ))}
        </ul>
      )}
    </Field>
  )
}

function GeneralPage() {
  const { id } = Route.useParams()
  const { data: project } = useQuery(projectDetailQuery(id))

  if (project === undefined) return null

  return <GeneralForm key={project.id} project={project} />
}

function GeneralForm({ project }: { project: ProjectDetail }) {
  const update = useUpdateProject(project.id)

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: project.name,
      description: project.description,
      priority: project.priority,
      service_ids: project.services.map((service) => service.id),
      figma_url: project.figma_url,
      prod_url: project.prod_url,
      preprod_url: project.preprod_url,
    },
  })

  const errors = form.formState.errors

  function onSubmit(values: Values) {
    update.mutate(values, {
      onSuccess: () => {
        toast.success('Paramètres enregistrés')
        form.reset(values)
      },
      onError: (error) =>
        toast.error(error instanceof HttpError ? error.message : 'Enregistrement impossible'),
    })
  }

  return (
    <form noValidate onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-3 p-4">
      <Card title="Informations du projet">
        <Field label="Nom du projet" error={errors.name?.message}>
          <Input {...form.register('name')} placeholder="Refonte du site vitrine" className={CHAMP} />
        </Field>

        <Field label="Description" error={errors.description?.message}>
          <Input
            {...form.register('description')}
            placeholder="(ex. : refonte complète du site et de son socle technique)"
            className={CHAMP}
          />
        </Field>

        <Field label="Priorité">
          <Choices
            value={form.watch('priority')}
            options={PRIORITES}
            onChange={(value) => form.setValue('priority', value, { shouldDirty: true })}
          />
        </Field>

        <Field label="Services">
          <ServicesPicker
            value={form.watch('service_ids')}
            onChange={(value) => form.setValue('service_ids', value, { shouldDirty: true })}
          />
        </Field>
      </Card>

      <Card title="Documents et liens">
        <Documents project={project} />

        <Field label="Lien Figma" error={errors.figma_url?.message}>
          <Input
            {...form.register('figma_url')}
            placeholder="https://www.figma.com/design/…"
            className={CHAMP}
          />
        </Field>

        <Field label="Lien de production" error={errors.prod_url?.message}>
          <Input {...form.register('prod_url')} placeholder="https://exemple.fr" className={CHAMP} />
        </Field>

        <Field label="Lien de préproduction" error={errors.preprod_url?.message}>
          <Input
            {...form.register('preprod_url')}
            placeholder="https://preprod.exemple.fr"
            className={CHAMP}
          />
        </Field>
      </Card>

      <SaveBar
        dirty={form.formState.isDirty}
        pending={update.isPending}
        onCancel={() => form.reset()}
      />
    </form>
  )
}
