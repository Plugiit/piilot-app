#!/usr/bin/env bash
# Publie une version de Piilot : CHANGELOG, VERSION, roadmap, commit, tag annote, push.
#
#   scripts/release.sh patch|minor|major|X.Y.Z[-rc.N] [--dry-run] [--no-ai] [--skip-checks] [--no-push]
#
# Le tag pousse declenche .github/workflows/release.yml, qui reconstruit
# l'image (donc rejoue toutes les verifications) et ne publie la release
# GitHub que si elle passe. Une version rouge n'a pas de release.
#
# Notes de version, par ordre de priorite :
#   1. une section « ## [X.Y.Z] » deja ecrite dans CHANGELOG.md, reprise telle quelle ;
#   2. un brouillon redige par Claude Code (claude -p) depuis les commits, les
#      fichiers modifies, les migrations et la roadmap — consigne dans
#      scripts/release-notes.md ;
#   3. a defaut (--no-ai, ou claude absent), une liste groupee des commits.
# Les verifications (make check) passent d'abord ; les notes sont ensuite
# toujours relues avant publication : publier, editer, regenerer ou abandonner.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

die() { printf 'erreur : %s\n' "$1" >&2; exit 1; }

usage() {
  sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

bump="" dry_run=0 ai=1 skip_checks=0 push=1
for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=1 ;;
    --no-ai) ai=0 ;;
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

# --- Verification ------------------------------------------------------------
# Avant les notes : inutile de faire rediger puis relire des notes pour une
# version qui ne passe pas ses propres verifications.
if [ "$skip_checks" = 0 ]; then
  [ -d web/node_modules ] \
    || die "dépendances du front absentes : lancer « cd web && npm ci », ou passer --skip-checks (la CI revérifie tout)"
  make check
fi

# --- Contexte ----------------------------------------------------------------
last_tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
range="${last_tag:+$last_tag..}HEAD"
# Premiere release : on compare a l'arbre vide.
base="${last_tag:-$(git hash-object -t tree /dev/null)}"

[ -n "$(git log --no-merges --format=%h "$range" | head -n 1)" ] \
  || die "aucun changement depuis ${last_tag:-le début du dépôt}"

notes="$(mktemp)" context="$(mktemp)" draft="$(mktemp)" entry="$(mktemp)"
trap 'rm -f "$notes" "$context" "$draft" "$entry"' EXIT

# Nom de la release dans le tableau de la roadmap (« En production »).
roadmap_name() {
  awk -v v="$1" '
    /<!-- roadmap:table -->/ { on = 1; next }
    /<!-- \/roadmap:table -->/ { on = 0 }
    on && /^\|/ {
      n = split($0, c, "|"); for (i = 1; i <= n; i++) gsub(/^ +| +$/, "", c[i])
      ver = c[3]; gsub(/\*/, "", ver)
      if (ver == v || (v ~ /-rc\./ && ver ~ /-rc\.N$/)) { print c[4]; exit }
    }
  ' ROADMAP.md
}

# Liste groupee des commits : le repli quand Claude n'est pas disponible.
commit_notes() {
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
  '
}

# Contexte donne a Claude : tout ce qui permet de decrire la version sans
# qu'il ait a explorer le depot.
release_context() {
  local noise=(':(exclude)**/package-lock.json' ':(exclude)api/internal/repository/db/**'
               ':(exclude)web/src/types/api-generated.ts' ':(exclude)web/src/routeTree.gen.ts'
               ':(exclude)docs/images/**')
  local m

  printf '# Version visée : %s\n' "$next"
  printf '# Version précédente : %s\n\n' "${last_tag:-aucune, première release publiée}"

  printf '## Objectif dans la roadmap\n\nNom : %s\n\n' "$(roadmap_name "$next")"
  awk -v v="$next" '
    $0 ~ "^## " v " " { on = 1; print; next }
    on && /^## / { exit }
    on { print }
  ' ROADMAP.md

  printf '\n## Commits, du plus ancien au plus récent\n\n'
  git log --no-merges --reverse --format='### %s%n%b' "$range" | grep -v '^### chore(release)' || true

  printf '\n## Fichiers modifiés, hors code généré\n\n'
  git diff --stat=160 --stat-count=200 "$base" HEAD -- . "${noise[@]}"

  printf '\n## Migrations de base ajoutées\n\n'
  for m in $(git diff --name-only --diff-filter=A "$base" HEAD -- 'api/migrations/*.up.sql'); do
    printf '### %s\n\n```sql\n' "$m"
    git show "HEAD:$m" | head -n 60 || true
    printf '```\n\n'
  done

  printf "\n## Routes d'API ajoutées (+) ou retirées (-)\n\n"
  if command -v python3 >/dev/null; then
    python3 - "$base" <<'PY' || true
import json, subprocess, sys

def routes(rev):
    res = subprocess.run(["git", "show", f"{rev}:api/openapi/openapi.json"], capture_output=True)
    if res.returncode != 0:
        return set()
    paths = json.loads(res.stdout).get("paths", {})
    return {f"{m.upper()} {p}" for p, ops in paths.items()
            for m in ops if m in ("get", "post", "put", "patch", "delete")}

old, new = routes(sys.argv[1]), routes("HEAD")
for r in sorted(new - old):
    print(f"+ {r}")
for r in sorted(old - new):
    print(f"- {r}")
PY
  fi

  printf "\n## Configuration : variables d'environnement et compose\n\n\`\`\`diff\n"
  git diff "$base" HEAD -- api/internal/config/config.go docker-compose.yml .env.example | head -n 150 || true
  printf '```\n\n## Dernière entrée du changelog, pour le ton\n\n'
  awk '/^## \[/ { n++ } n == 1 { print }' CHANGELOG.md | head -n 80 || true
}

# Brouillon redige par Claude Code. Aucun outil : il redige a partir du
# contexte fourni, sans lire ni modifier le depot, et ne garde pas de session.
ai_notes() {
  command -v claude >/dev/null || return 1
  printf 'Rédaction des notes par Claude Code…\n' >&2
  release_context > "$context"
  claude -p "$(cat scripts/release-notes.md)" \
    --tools "" --no-session-persistence ${CLAUDE_MODEL:+--model "$CLAUDE_MODEL"} \
    < "$context" > "$draft" || return 1
  [ -s "$draft" ] || return 1
  cp "$draft" "$notes"
}

# Lignes vides en tete et en fin retirees : elles finiraient dans le tag et
# dans la release.
trim_notes() {
  awk 'NF { last = NR } { line[NR] = $0 }
       END { for (i = 1; i <= last; i++) if (started || line[i] ~ /[^[:space:]]/) { started = 1; print line[i] } }' \
    "$notes" > "$draft" && cp "$draft" "$notes"
}

# --- Notes de version --------------------------------------------------------
section_exists=0
if grep -q "^## \[$next\]" CHANGELOG.md; then
  section_exists=1
  source_label="section rédigée dans CHANGELOG.md"
  awk -v v="$next" '
    $0 ~ "^## \\[" v "\\]" { on = 1; next }
    on && /^## \[/ { exit }
    on { print }
  ' CHANGELOG.md > "$notes"
elif [ "$ai" = 1 ] && ai_notes; then
  source_label="brouillon rédigé par Claude Code, à relire"
else
  [ "$ai" = 0 ] || printf 'Claude Code indisponible : liste des commits à la place.\n' >&2
  source_label="liste des commits"
  commit_notes > "$notes"
fi
trim_notes
[ -s "$notes" ] || die "notes de version vides"

# Titre de la section : « ## [0.4.0] — En production · 2026-10-02 ».
title="$(roadmap_name "$next")"
if [ -z "$title" ] && [ "$bump" = patch ]; then title="Correctifs"; fi
heading="## [$next]${title:+ — $title} · $(date +%F)"

while :; do
  printf '\n=== %s -> %s, depuis %s — %s ===\n\n%s\n\n' \
    "$current" "$next" "${last_tag:-le début}" "$source_label" "$heading"
  cat "$notes"

  if [ "$dry_run" = 1 ]; then
    printf "\n[dry-run] Rien n'a été modifié.\n"
    exit 0
  fi

  printf '\nPublier %s ? [o]ui, [e]diter, [r]égénérer, [N]on : ' "$tag"
  read -r answer
  case "$answer" in
    o|O|y|Y) break ;;
    e|E) "${EDITOR:-vi}" "$notes"; trim_notes ;;
    r|R)
      if [ "$section_exists" = 1 ]; then
        printf 'Section écrite à la main : éditer plutôt que régénérer.\n'
      elif ai_notes; then
        trim_notes; source_label="brouillon rédigé par Claude Code, à relire"
      else
        printf 'Régénération impossible : Claude Code indisponible.\n'
      fi ;;
    *) die "abandon, rien n'a été modifié" ;;
  esac
done

# --- Ecriture ----------------------------------------------------------------
{ printf '%s\n\n' "$heading"; cat "$notes"; } > "$entry"
if [ "$section_exists" = 1 ]; then
  # La section existante est remplacee par la version relue, titre date.
  awk -v v="$next" -v f="$entry" '
    $0 ~ "^## \\[" v "\\]" { while ((getline l < f) > 0) print l; print ""; skip = 1; next }
    skip && /^## \[/ { skip = 0 }
    !skip { print }
  ' CHANGELOG.md > CHANGELOG.md.tmp
else
  # Nouvelle section sous le marqueur, la plus recente en premier.
  awk -v f="$entry" '
    { print }
    /^<!-- releases -->$/ { print ""; while ((getline l < f) > 0) print l }
  ' CHANGELOG.md > CHANGELOG.md.tmp
fi
mv CHANGELOG.md.tmp CHANGELOG.md

printf '%s\n' "$next" > VERSION
# Statut des releases et version actuelle, dans ROADMAP.md et le README.
scripts/roadmap.sh

git add VERSION CHANGELOG.md ROADMAP.md README.md
git commit --quiet -m "chore(release): $tag"
# verbatim : les titres « ### » des notes ne sont pas des commentaires.
git tag -a --cleanup=verbatim "$tag" -F "$notes"
printf '\nTag %s créé sur %s.\n' "$tag" "$(git rev-parse --short HEAD)"

if [ "$push" = 1 ] && [ "$has_origin" = 1 ]; then
  git push --quiet --atomic origin master "$tag"
  printf "Poussé. La release GitHub sera publiée par la CI si l'image se construit.\n"
else
  printf 'Non poussé. Pour publier : git push --atomic origin master %s\n' "$tag"
fi
