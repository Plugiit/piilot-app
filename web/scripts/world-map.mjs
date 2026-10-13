// Genere src/assets/world-map.json : le contour de chaque pays, deja projete,
// indexe par son code ISO 3166 alpha-2 — celui qu'Umami rend pour les pays.
//
// Le fond de carte est fige une fois pour toutes plutot que calcule dans le
// navigateur : pas de dependance cartographique dans l'application, un seul
// fichier statique, charge avec la page « Statistiques » seulement.
//
// Donnees : Natural Earth 1:110m (domaine public), via world-atlas (ISC).
// A relancer seulement pour changer de projection ou de resolution, depuis
// un dossier temporaire ou ces paquets sont installes :
//
//   npm i world-atlas@2 topojson-client@3 d3-geo@3 i18n-iso-countries@7
//   node <chemin>/web/scripts/world-map.mjs <chemin>/web/src/assets/world-map.json
import { writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { join } from 'node:path'

// Les paquets sont lus dans le dossier courant, pas dans le depot.
const require = createRequire(join(process.cwd(), 'noop.js'))
const { feature } = require('topojson-client')
const { geoNaturalEarth1, geoPath } = require('d3-geo')
const countries = require('i18n-iso-countries')
const topology = require('world-atlas/countries-110m.json')

const out = process.argv[2]
if (!out) {
  console.error('usage : node world-map.mjs <fichier de sortie>')
  process.exit(1)
}

const width = 960
const height = 470
const world = feature(topology, topology.objects.countries)
// L'Antarctique n'accueille pas de visiteurs et mange un quart de la hauteur.
world.features = world.features.filter((f) => f.properties.name !== 'Antarctica')

const projection = geoNaturalEarth1().fitSize([width, height], world)
const path = geoPath(projection)
// Une decimale suffit a cette echelle, et divise le fichier par trois.
const round = (d) => d.replace(/-?\d+\.\d+/g, (n) => String(Math.round(Number(n) * 10) / 10))

// Quelques territoires n'ont pas de code numerique dans Natural Earth.
const byName = { Kosovo: 'XK', 'N. Cyprus': 'CY', Somaliland: 'SO' }

const paths = {}
for (const f of world.features) {
  const code = (f.id !== undefined && countries.numericToAlpha2(f.id)) || byName[f.properties.name]
  const d = path(f)
  if (!code || !d) continue
  paths[code] = paths[code] ? `${paths[code]}${round(d)}` : round(d)
}

writeFileSync(out, JSON.stringify({ width, height, countries: paths }))
console.log(`${Object.keys(paths).length} pays → ${out}`)
