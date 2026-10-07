import { GitBranchIcon, RefreshIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'

import { PageFrame } from '@/components/layout/page-frame'
import { Button } from '@/components/ui/button'
import { gitSettingsQuery, useRotateGitSecret } from '@/features/git/api'
import { HttpError } from '@/lib/api'
import { can } from '@/lib/auth'

import { Card, Field } from '@/components/settings-ui'

/**
 * Reglage des webhooks GitHub et GitLab : l'adresse a leur donner, le
 * secret, et la marche a suivre chez eux. Une fois par installation, puis
 * chaque projet renseigne son depot.
 */
export const Route = createFileRoute('/_app/parametres/git/')({
  beforeLoad: ({ context }) => {
    if (!can(context.user, 'users.write')) throw redirect({ to: '/parametres/services', search: { page: 1 }, replace: true })
  },
  component: GitSettingsPage,
})

function GitSettingsPage() {
  const { data, isPending } = useQuery(gitSettingsQuery)
  const rotate = useRotateGitSecret()
  const [confirm, setConfirm] = useState(false)

  return (
    <PageFrame title="Dépôts Git">
      <div className="flex flex-col gap-3 p-4">
        <Card
          title="Webhook"
          description="À copier une fois dans GitHub ou GitLab, au niveau de l’organisation ou du groupe pour couvrir tous les dépôts, ou dépôt par dépôt. Ensuite, chaque projet indique son dépôt dans ses paramètres."
        >
          {isPending || data === undefined ? (
            <div className="h-[88px] animate-pulse rounded-[10px] bg-[#fafafa]" />
          ) : (
            <>
              <Field label="Adresse du webhook">
                <CopyLine value={data.webhook_url} />
              </Field>

              <Field
                label="Secret"
                hint={
                  data.from_env
                    ? 'Défini par GIT_WEBHOOK_SECRET dans l’environnement : il se change là-bas.'
                    : 'Tiré au sort par Piilot. Le changer invalide les webhooks configurés avec l’ancien.'
                }
              >
                <div className="flex flex-wrap items-center gap-2">
                  <CopyLine value={data.secret} secret />
                  {!data.from_env && !confirm && (
                    <Button type="button" variant="ghost" size="sm" className="gap-1.5" onClick={() => setConfirm(true)}>
                      <HugeiconsIcon icon={RefreshIcon} size={14} strokeWidth={1.8} />
                      Changer le secret
                    </Button>
                  )}
                  {confirm && (
                    <span className="flex items-center gap-2 text-[13px] text-[#73757c]">
                      Les webhooks existants cesseront d’être acceptés.
                      <Button
                        type="button"
                        size="sm"
                        disabled={rotate.isPending}
                        onClick={() =>
                          rotate.mutate(undefined, {
                            onSuccess: () => {
                              setConfirm(false)
                              toast.success('Nouveau secret tiré : mettez-le à jour chez GitHub et GitLab.')
                            },
                            onError: (error) => toast.error(error instanceof HttpError ? error.message : 'Changement impossible'),
                          })
                        }
                      >
                        Confirmer
                      </Button>
                      <Button type="button" variant="ghost" size="sm" onClick={() => setConfirm(false)}>
                        Annuler
                      </Button>
                    </span>
                  )}
                </div>
              </Field>
            </>
          )}
        </Card>

        <Card title="Chez GitHub" description="Settings → Webhooks de l’organisation (un seul pour tous ses dépôts) ou d’un dépôt.">
          <Steps
            items={[
              'Payload URL : l’adresse ci-dessus. Content type : application/json.',
              'Secret : celui ci-dessus.',
              'Événements : Pull requests, Releases, Pushes (pour les tags), Deployment statuses.',
            ]}
          />
        </Card>

        <Card title="Chez GitLab" description="Settings → Webhooks du groupe ou du projet. Une instance auto-hébergée convient.">
          <Steps
            items={[
              'URL : l’adresse ci-dessus.',
              'Secret token : celui ci-dessus.',
              'Déclencheurs : Merge request events, Tag push events, Releases events, Deployment events.',
            ]}
          />
        </Card>

        <Card title="Rien à lier à la main" description="Une pull request nomme ce qu’elle fait avancer par son numéro, dans son titre, sa branche ou sa description.">
          <div className="flex flex-col gap-2 text-[14px] text-[#1b1b1b]">
            <p>
              <code className="rounded-[5px] bg-[#f3f4f4] px-1.5 py-0.5 text-[13px]">#47</code> ou{' '}
              <code className="rounded-[5px] bg-[#f3f4f4] px-1.5 py-0.5 text-[13px]">ticket-47</code> pour le ticket 47 ·{' '}
              <code className="rounded-[5px] bg-[#f3f4f4] px-1.5 py-0.5 text-[13px]">T-123</code> ou{' '}
              <code className="rounded-[5px] bg-[#f3f4f4] px-1.5 py-0.5 text-[13px]">task-123</code> pour la tâche 123. Le numéro d’une tâche
              se lit dans son panneau.
            </p>
            <ul className="list-disc pl-5 text-[13px] text-[#4b4b4f]">
              <li>Pull request ouverte : ticket et tâche passent « En revue ».</li>
              <li>Pull request fusionnée : le ticket passe « Prêt à déployer », la tâche est terminée.</li>
              <li>
                Mise en ligne (release publiée, tag poussé, déploiement réussi) : les tickets prêts à déployer sont clos avec un mot au client,
                le jalon « Mise en ligne » est atteint, le journal du client s’en souvient.
              </li>
            </ul>
            <p className="flex items-center gap-1.5 text-[13px] text-[#73757c]">
              <HugeiconsIcon icon={GitBranchIcon} size={14} strokeWidth={1.8} />
              Un numéro qui ne correspond à rien dans le projet du dépôt est ignoré.
            </p>
          </div>
        </Card>
      </div>
    </PageFrame>
  )
}

function Steps({ items }: { items: string[] }) {
  return (
    <ol className="flex list-decimal flex-col gap-1 pl-5 text-[14px] text-[#1b1b1b]">
      {items.map((item) => (
        <li key={item}>{item}</li>
      ))}
    </ol>
  )
}

/** Une valeur a copier : lisible, selectionnable, et un bouton pour la prendre d'un coup. */
function CopyLine({ value, secret = false }: { value: string; secret?: boolean }) {
  const [shown, setShown] = useState(!secret)
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error('Copie impossible : sélectionnez la valeur à la main.')
    }
  }

  return (
    <div className="flex min-w-0 flex-1 items-center gap-2">
      <code className="min-w-0 flex-1 truncate rounded-[8px] border border-[#e8e8e9] bg-[#fafafa] px-3 py-2 font-mono text-[13px] text-[#1b1b1b] select-all">
        {shown ? value : '•'.repeat(32)}
      </code>
      {secret && (
        <Button type="button" variant="outline" size="sm" onClick={() => setShown((v) => !v)}>
          {shown ? 'Masquer' : 'Afficher'}
        </Button>
      )}
      <Button type="button" variant="outline" size="sm" onClick={() => void copy()}>
        {copied ? 'Copié' : 'Copier'}
      </Button>
    </div>
  )
}
