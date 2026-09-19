import { extensionOf } from '@/lib/utils'

/**
 * Vignette d'un fichier joint.
 *
 * Le trait vient du fichier de design « File / Document Format Type » : un
 * aplat aux coins arrondis, le coin superieur droit replie en plus clair, et
 * une pastille sombre en bas a gauche qui porte l'extension.
 *
 * Les vignettes sont dessinees ici plutot qu'importees : le pack d'origine est
 * payant et son echantillon gratuit ne couvre aucun des formats qu'une agence
 * recoit vraiment — ni PDF, ni tableur, ni archive. Une forme unique que la
 * couleur et le libelle parametrent evite autant de fichiers que de formats,
 * et se substituera d'un seul endroit le jour ou le pack sera acquis.
 */

/**
 * Couleur par extension.
 *
 * Celles des marques quand le format en a une — le bleu de Word, le vert
 * d'Excel, la brique de PowerPoint, l'orange de HTML5 — pour que la vignette
 * se reconnaisse avant d'etre lue. Les formats sans identite propre prennent
 * une teinte qui les distingue de leurs voisins : le vert du CSV n'est pas
 * celui d'Excel, sans quoi un export se confondrait avec un classeur.
 */
const FILE_COLORS: Record<string, string> = {
  pdf: '#e5252a',

  doc: '#2b579a',
  docx: '#2b579a',
  odt: '#2b579a',
  rtf: '#2b579a',

  xls: '#217346',
  xlsx: '#217346',
  ods: '#217346',

  ppt: '#c43e1c',
  pptx: '#c43e1c',
  odp: '#c43e1c',
  key: '#c43e1c',

  png: '#2196f3',
  jpg: '#2196f3',
  jpeg: '#2196f3',
  gif: '#2196f3',
  webp: '#2196f3',
  avif: '#2196f3',
  heic: '#2196f3',
  bmp: '#2196f3',

  svg: '#ff5722',

  zip: '#f0a500',
  rar: '#f0a500',
  '7z': '#f0a500',
  tar: '#f0a500',
  gz: '#f0a500',

  txt: '#607d8b',
  log: '#607d8b',

  csv: '#1d9d5c',
  tsv: '#1d9d5c',

  json: '#3b3f46',
  yml: '#3b3f46',
  yaml: '#3b3f46',

  md: '#4a5568',
  mdx: '#4a5568',

  html: '#e34f26',
  htm: '#e34f26',

  fig: '#a259ff',

  mp4: '#7c3aed',
  mov: '#7c3aed',
  webm: '#7c3aed',
  avi: '#7c3aed',
}

/** Teinte des formats qu'on ne connait pas : lisible, sans rien affirmer. */
const UNKNOWN = '#c4c4c4'

/**
 * Geometrie de la vignette, dans la grille de 64 du SVG.
 *
 * Les bords sont nommes parce qu'ils se retrouvent a cinq endroits une fois
 * les trace ecrits : le bord droit sert au bord haut, a la diagonale, au coin
 * inferieur et deux fois au repli. Ecrits en dur, il suffisait d'en oublier un
 * pour deformer la silhouette.
 *
 * La vignette ne remplit pas le carre : elle laisse sa marge en bas a gauche a
 * la pastille, qui deborde volontairement sur elle.
 */
const LEFT = 12
const RIGHT = 52
const TOP = 2
const BOTTOM = 54
const RADIUS = 7

/** Cote du coin replie — environ un tiers de la largeur, comme au modele. */
const FOLD_SIZE = 14

const BADGE_HEIGHT = 16
const BADGE_X = LEFT - 8
const BADGE_Y = BOTTOM - BADGE_HEIGHT - 2

/** Le corps, coin superieur droit coupe. */
const BODY = [
  `M${LEFT + RADIUS} ${TOP}`,
  `H${RIGHT - FOLD_SIZE}`,
  `L${RIGHT} ${TOP + FOLD_SIZE}`,
  `V${BOTTOM - RADIUS}`,
  `A${RADIUS} ${RADIUS} 0 0 1 ${RIGHT - RADIUS} ${BOTTOM}`,
  `H${LEFT + RADIUS}`,
  `A${RADIUS} ${RADIUS} 0 0 1 ${LEFT} ${BOTTOM - RADIUS}`,
  `V${TOP + RADIUS}`,
  `A${RADIUS} ${RADIUS} 0 0 1 ${LEFT + RADIUS} ${TOP}`,
  'Z',
].join(' ')

/** Le repli, pose par-dessus le corps en blanc translucide. */
const FOLD = [
  `M${RIGHT - FOLD_SIZE} ${TOP}`,
  `L${RIGHT} ${TOP + FOLD_SIZE}`,
  `H${RIGHT - FOLD_SIZE + 5}`,
  `A5 5 0 0 1 ${RIGHT - FOLD_SIZE} ${TOP + FOLD_SIZE - 5}`,
  'Z',
].join(' ')

export function FileIcon({
  filename,
  size = 64,
  className,
}: {
  filename: string
  size?: number
  className?: string
}) {
  const extension = extensionOf(filename)
  const color = FILE_COLORS[extension] ?? UNKNOWN

  // Quatre caracteres au plus : au-dela la pastille deborderait de la vignette
  // et les extensions plus longues que cela sont rarissimes.
  const label = extension.slice(0, 4).toUpperCase()

  // La pastille se dimensionne sur son texte. Les chiffres sont ceux d'une
  // graisse grasse a 9px, mesures au trace : il n'y a pas moyen de connaitre
  // la largeur rendue avant qu'elle ne le soit.
  const width = label.length * 6 + 10

  return (
    <svg
      viewBox="0 0 64 64"
      width={size}
      height={size}
      role="img"
      aria-label={extension === '' ? 'Fichier' : `Fichier ${label}`}
      className={className}
    >
      <path d={BODY} fill={color} />
      <path d={FOLD} fill="#ffffff" fillOpacity={0.35} />

      {/* La pastille mord sur le coin inferieur gauche du document : elle
          deborde d'un cran a gauche, et reste au-dessus du bord bas. Ses
          reperes sont pris sur les bords plutot qu'ecrits en dur, pour qu'un
          changement de largeur ne la laisse pas en arriere. */}
      {label !== '' && (
        <>
          <rect
            x={BADGE_X}
            y={BADGE_Y}
            width={width}
            height={BADGE_HEIGHT}
            rx={5}
            fill="#1b1b1b"
          />
          <text
            x={BADGE_X + width / 2}
            y={BADGE_Y + BADGE_HEIGHT / 2 + 3.4}
            textAnchor="middle"
            fontSize={9}
            fontWeight={700}
            letterSpacing={-0.2}
            fill="#ffffff"
          >
            {label}
          </text>
        </>
      )}
    </svg>
  )
}
