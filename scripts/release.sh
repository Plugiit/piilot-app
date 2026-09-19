#!/usr/bin/env bash
# Publie une version de Piilot : CHANGELOG, VERSION, roadmap, commit, tag annote, push.
#
#   scripts/release.sh patch|minor|major|X.Y.Z[-rc.N] [--dry-run] [--edit] [--skip-checks] [--no-push]
#
# Le tag pousse declenche .github/workflows/release.yml, qui reconstruit
# l'image (donc rejoue toutes les verifications) et ne publie la release
# GitHub que si elle passe. Une version rouge n'a pas de release.
#
# Notes de version : si CHANGELOG.md contient deja une section pour la version
# visee (« ## [X.Y.Z] »), elle est reprise telle quelle — c'est la facon de les
# ecrire a la main. Sinon elles sont generees depuis les messages de commit
# (Conventional Commits) depuis le dernier tag.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

die() { printf 'erreur : %s\n' "$1" >&2; exit 1; }

usage() {
  sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

bump="" dry_run=0 edit=0 skip_checks=0 push=1
for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=1 ;;
    --edit) edit=1 ;;
    --skip-checks) skip_checks=1 ;;
    --no-push) push=0 ;;
    -h|--help) usage ;;
    -*) die "option inconnue : $arg" ;;
    *) [ -z "$bump" ] || die "une seule version à la fois"; bump="$arg" ;;
  esac
done
[ -n "$bump" ] || usage 1

# --- Version visee -----------------------------------------------------------
current="$(tr -d '[:space:]' < VERSION)"
semver='^([0-9]+)\.([0-9]+)\.([0-9]+)(-[0-9A-Za-z.]+)?$'
[[ "$current" =~ $semver ]] || die "VERSION invalide : « $current »"
major="${BASH_REMATCH[1]}" minor="${BASH_REMATCH[2]}" patch="${BASH_REMATCH[3]}" pre="${BASH_REMATCH[4]}"

case "$bump" in
  # Sortie de pre-version : 1.0.0-rc.2 + patch donne 1.0.0, pas 1.0.1.
  patch) if [ -n "$pre" ]; then next="$major.$minor.$patch"; else next="$major.$minor.$((patch + 1))"; fi ;;
  minor) next="$major.$((minor + 1)).0" ;;
  major) next="$((major + 1)).0.0" ;;
  *) next="${bump#v}"; [[ "$next" =~ $semver ]] || die "version invalide : « $bump » (attendu patch, minor, major ou X.Y.Z)" ;;
esac
tag="v$next"

# --- Garde-fous --------------------------------------------------------------
branch="$(git symbolic-ref --short HEAD 2>/dev/null || true)"
[ "$branch" = "master" ] || die "une release part de master (branche actuelle : ${branch:-HEAD détaché})"
[ -z "$(git status --porcelain)" ] || die "l'arbre de travail n'est pas propre : committer ou remiser d'abord"
! git rev-parse -q --verify "refs/tags/$tag" >/dev/null || die "le tag $tag existe déjà"

has_origin=0
if git remote get-url origin >/dev/null 2>&1; then
  has_origin=1
  git fetch --quiet origin master --tags
  git merge-base --is-ancestor origin/master HEAD \
    || die "master est en retard sur origin/master : faire un git pull d'abord"
  ! git rev-parse -q --verify "refs/tags/$tag" >/dev/null || die "le tag $tag existe déjà sur origin"
fi

# --- Notes de version --------------------------------------------------------
last_tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
range="${last_tag:+$last_tag..}HEAD"
notes="$(mktemp)"; trap 'rm -f "$notes"' EXIT

section_exists=0
if grep -q "^## \[$next\]" CHANGELOG.md; then
  section_exists=1
  # Corps de la section existante, sans son titre, jusqu'a la section suivante.
  awk -v v="$next" '
    $0 ~ "^## \\[" v "\\]" { on = 1; next }
    on && /^## \[/ { exit }
    on { print }
  ' CHANGELOG.md > "$notes"
else
  git log --no-merges --format='%s' "$range" | awk '
    /^chore\(release\)/ { next }
    {
      line = $0; type = ""; scope = ""; breaking = 0
      if (match(line, /^[a-z]+(\([^)]*\))?!?: /)) {
        head = substr(line, 1, RLENGTH - 2); msg = substr(line, RLENGTH + 1)
        if (head ~ /!$/) { breaking = 1; sub(/!$/, "", head) }
        type = head; sub(/\(.*/, "", type)
        if (match(head, /\(.*\)/)) scope = substr(head, RSTART + 1, RLENGTH - 2)
      } else { msg = line }
      item = "- " (scope != "" ? "**" scope "** : " : "") msg
      if (breaking)            brk = brk item "\n"
      else if (type == "feat") feat = feat item "\n"
      else if (type == "fix")  fix = fix item "\n"
      else if (type == "perf") perf = perf item "\n"
      else if (type == "")     other = other item "\n"
      else                     tech = tech item "\n"
    }
    function section(title, body) { if (body != "") printf "### %s\n\n%s\n", title, body }
    END {
      section("Changements incompatibles", brk)
      section("Nouveautés", feat)
      section("Corrections", fix)
      section("Performances", perf)
      section("Technique", tech)
      section("Autres", other)
    }
  ' > "$notes"
fi

# Lignes vides en tete et en fin retirees : elles finiraient dans le tag et
# dans la release.
awk 'NF { last = NR } { line[NR] = $0 } END { for (i = 1; i <= last; i++) if (started || line[i] ~ /[^[:space:]]/) { started = 1; print line[i] } }' \
  "$notes" > "$notes.trim" && mv "$notes.trim" "$notes"

[ -s "$notes" ] || die "aucun changement depuis ${last_tag:-le début du dépôt}"

if [ "$edit" = 1 ]; then
  "${EDITOR:-vi}" "$notes"
fi

printf '\n=== %s -> %s (%s) ===\n\n' "$current" "$next" "${last_tag:-première release}"
cat "$notes"

if [ "$dry_run" = 1 ]; then
  printf "\n[dry-run] Rien n'a été modifié.\n"
  exit 0
fi

printf '\nPublier %s ? [o/N] ' "$tag"
read -r answer
[[ "$answer" =~ ^[oOyY]$ ]] || die "abandon, rien n'a été modifié"

# --- Verification ------------------------------------------------------------
if [ "$skip_checks" = 0 ]; then
  make check
fi

# --- Ecriture ----------------------------------------------------------------
if [ "$section_exists" = 0 ]; then
  entry="$(mktemp)"
  { printf '## [%s] — %s\n\n' "$next" "$(date +%F)"; cat "$notes"; } > "$entry"
  # Nouvelle section inseree sous le marqueur, la plus recente en premier.
  awk -v f="$entry" '
    { print }
    /^<!-- releases -->$/ { print ""; while ((getline l < f) > 0) print l }
  ' CHANGELOG.md > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
  rm -f "$entry"
fi
printf '%s\n' "$next" > VERSION
# Statut des releases et version actuelle, dans ROADMAP.md et le README.
scripts/roadmap.sh

git add VERSION CHANGELOG.md ROADMAP.md README.md
git commit --quiet -m "chore(release): $tag"
git tag -a "$tag" -F "$notes"
printf '\nTag %s créé sur %s.\n' "$tag" "$(git rev-parse --short HEAD)"

if [ "$push" = 1 ] && [ "$has_origin" = 1 ]; then
  git push --quiet --atomic origin master "$tag"
  printf "Poussé. La release GitHub sera publiée par la CI si l'image se construit.\n"
else
  printf 'Non poussé. Pour publier : git push --atomic origin master %s\n' "$tag"
fi
