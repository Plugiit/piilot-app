import { useId } from 'react'

/**
 * Courbe en aire, sans axe ni graduation : la forme suffit a dire si ca monte.
 * Les points vont du bord gauche au bord droit, le zero est le bas exact du
 * cadre. L'interpolation est cubique et monotone : la courbe s'arrondit sans
 * jamais passer sous zero ni au-dessus du maximum.
 */
export function AreaCurve({ values, color, label }: { values: number[]; color: string; label: string }) {
  const id = useId()
  if (values.length < 2) return null

  const W = 100
  const H = 34
  // Une marge haute pour que le trait du maximum ne soit pas rogne.
  const TOP = 3
  const max = Math.max(1, ...values)
  const points = values.map((v, i) => [(i / (values.length - 1)) * W, TOP + (1 - v / max) * (H - TOP)] as const)

  // Tangentes de Fritsch-Carlson : nulles aux extremums et aux paliers,
  // moyennees ailleurs, bornees pour ne jamais depasser les points voisins.
  const slopes = points.slice(1).map(([x, y], i) => {
    const prev = points[i] ?? [x, y]
    return (y - prev[1]) / (x - prev[0])
  })
  const tangents = points.map((_, i) => {
    const before = slopes[i - 1]
    const after = slopes[i]
    if (before === undefined) return after ?? 0
    if (after === undefined) return before
    if (before * after <= 0) return 0
    // Points equidistants : la moyenne ponderee se reduit a l'harmonique.
    return (2 * before * after) / (before + after)
  })
  const line = points
    .map(([x, y], i) => {
      const prev = points[i - 1]
      if (prev === undefined) return `M${x},${y}`
      const dx = (x - prev[0]) / 3
      const t0 = tangents[i - 1] ?? 0
      const t1 = tangents[i] ?? 0
      return `C${prev[0] + dx},${prev[1] + t0 * dx} ${x - dx},${y - t1 * dx} ${x},${y}`
    })
    .join(' ')
  const area = `${line} L${W},${H} L0,${H} Z`

  return (
    <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" role="img" aria-label={label} className="block size-full">
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity={0.28} />
          <stop offset="100%" stopColor={color} stopOpacity={0.02} />
        </linearGradient>
      </defs>
      <path d={area} fill={`url(#${id})`} />
      <path d={line} fill="none" stroke={color} strokeWidth={1.5} vectorEffect="non-scaling-stroke" strokeLinejoin="round" />
    </svg>
  )
}
