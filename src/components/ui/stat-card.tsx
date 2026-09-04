import { cn } from '@/lib/utils'

interface StatCardProps {
  label: string
  value: number | undefined
  loading?: boolean
  tone?: 'default' | 'danger'
}

export function StatCard({ label, value, loading, tone = 'default' }: StatCardProps) {
  return (
    <div className="rounded-xl border border-border bg-surface-raised p-4">
      <p className="text-xs font-medium uppercase tracking-wide text-ink-muted">{label}</p>

      {loading ? (
        // Le skeleton occupe exactement la hauteur de la valeur finale : pas de
        // saut de mise en page quand la donnee arrive.
        <div className="mt-2 h-8 w-16 animate-pulse rounded bg-surface-sunken" />
      ) : (
        <p
          className={cn(
            'mt-2 text-3xl font-semibold tabular-nums',
            tone === 'danger' && value ? 'text-danger' : 'text-ink',
          )}
        >
          {value ?? 0}
        </p>
      )}
    </div>
  )
}
