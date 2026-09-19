#!/usr/bin/env bash
# Recalcule le statut des releases de la roadmap depuis le fichier VERSION.
#
#   scripts/roadmap.sh           reecrit les blocs dans ROADMAP.md et README.md
#   scripts/roadmap.sh --check   echoue si les blocs ne sont pas a jour (CI)
#
# Source : le tableau entre <!-- roadmap:table --> et <!-- /roadmap:table -->
# dans ROADMAP.md. Seule la colonne Statut est calculee ; les autres se
# modifient a la main. Le README reprend le tableau entre
# <!-- roadmap:readme --> et <!-- /roadmap:readme -->.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

check=0
[ "${1:-}" = "--check" ] && check=1

current="$(tr -d '[:space:]' < VERSION)"

# Tableau recalcule : une ligne par release, statut compare a VERSION.
#   - meme mineure (ou meme serie rc) que VERSION : actuelle
#   - anterieure : livree
#   - posterieure : a venir
table="$(awk -v cur="$current" '
  function rank(v,    base, n, p) {
    base = v; sub(/-.*/, "", base)
    n = split(base, p, ".")
    r = p[1] * 1000000 + p[2] * 1000 + p[3]
    return (v ~ /-/) ? r - 0.5 : r
  }
  function family(v,    p) {
    if (v ~ /-rc/) { sub(/-.*/, "", v); return v "-rc" }
    split(v, p, "."); return p[1] "." p[2]
  }
  /<!-- roadmap:table -->/ { on = 1; next }
  /<!-- \/roadmap:table -->/ { on = 0 }
  on && /^\|/ && !/^\| *Statut/ && !/^\|-/ {
    n = split($0, c, "|")
    for (i = 1; i <= n; i++) gsub(/^ +| +$/, "", c[i])
    v = c[3]; gsub(/\*/, "", v)
    if (family(v) == family(cur)) {
      status = "📍 Actuelle"
      if (v != cur) status = status " (`" cur "`)"
      shown = "**" v "**"
    } else {
      status = (rank(v) < rank(cur)) ? "✅ Livrée" : "⏳ À venir"
      shown = v
    }
    printf "| %s | %s | %s | %s | %s |\n", status, shown, c[4], c[5], c[6]
  }
' ROADMAP.md)"

[ -n "$table" ] || { echo "erreur : tableau introuvable dans ROADMAP.md" >&2; exit 1; }

header='| Statut | Version | Nom | Espace | Objectif |
|---|---|---|---|---|'

roadmap_block="<!-- roadmap:table -->
$header
$table
<!-- /roadmap:table -->"

readme_block="<!-- roadmap:readme -->
**Version actuelle : \`$current\`** — détail de chaque release dans [ROADMAP.md](ROADMAP.md).

$header
$table
<!-- /roadmap:readme -->"

# Remplace un bloc balise par son nouveau contenu, sans toucher au reste.
replace_block() {
  local file="$1" start="$2" end="$3" content="$4"
  grep -q "^$start\$" "$file" || { echo "erreur : balise $start absente de $file" >&2; exit 1; }
  BLOCK="$content" awk -v s="$start" -v e="$end" '
    $0 == s { print ENVIRON["BLOCK"]; skip = 1; next }
    $0 == e { skip = 0; next }
    !skip { print }
  ' "$file"
}

new_roadmap="$(replace_block ROADMAP.md '<!-- roadmap:table -->' '<!-- /roadmap:table -->' "$roadmap_block")"
new_readme="$(replace_block README.md '<!-- roadmap:readme -->' '<!-- /roadmap:readme -->' "$readme_block")"

if [ "$check" = 1 ]; then
  stale=0
  [ "$new_roadmap" = "$(cat ROADMAP.md)" ] || { echo "ROADMAP.md n'est pas à jour avec VERSION ($current)"; stale=1; }
  [ "$new_readme" = "$(cat README.md)" ] || { echo "README.md n'est pas à jour avec VERSION ($current)"; stale=1; }
  [ "$stale" = 0 ] || { echo "Lancer : make roadmap"; exit 1; }
  echo "Roadmap à jour (version $current)."
  exit 0
fi

printf '%s\n' "$new_roadmap" > ROADMAP.md
printf '%s\n' "$new_readme" > README.md
echo "Roadmap mise à jour (version $current)."
