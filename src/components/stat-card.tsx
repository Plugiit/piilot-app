import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

interface StatCardProps {
  label: string
  value: number | undefined
  loading?: boolean
  /** `danger` colore la valeur quand elle est non nulle : un compteur de retards a zero n'a rien d'alarmant. */
  tone?: 'default' | 'danger'
}

/**
 * Tuile de chiffre-cle du tableau de bord.
 *
 * Vit hors de `components/ui/`, reserve aux composants shadcn : melanger nos
 * composants aux leurs exposerait les notres a etre ecrases au prochain
 * `shadcn add --overwrite`.
 */
export function StatCard({ label, value, loading, tone = 'default' }: StatCardProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
          {label}
        </CardTitle>
      </CardHeader>

      <CardContent>
        {loading ? (
          // Le skeleton occupe la hauteur exacte de la valeur finale : la mise
          // en page ne saute pas quand la donnee arrive.
          <Skeleton className="h-9 w-16" />
        ) : (
          <p
            className={cn(
              'text-3xl font-semibold tabular-nums',
              tone === 'danger' && value ? 'text-destructive' : 'text-foreground',
            )}
          >
            {value ?? 0}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
