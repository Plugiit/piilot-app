# syntax=docker/dockerfile:1

# Une seule image pour tout Piilot : le binaire Go sert l'API sous /api/v1 et
# le build du front pour tout le reste. Front et API partagent donc la meme
# origine — ni CORS, ni URL d'API figee dans le bundle, un seul domaine a
# declarer dans Coolify.
#
# Chaque etape de verification bloque le build : une image n'est produite que
# si le front ET l'API passent, donc un deploiement ne peut pas partir sur du
# rouge. La version precedente reste en ligne.

# =============================================================================
# Stage 1 : front — lint, typage, tests, build
# =============================================================================
FROM node:24-alpine AS web

WORKDIR /src/web

# Manifestes d'abord : la couche d'installation ne rejoue que si les
# dependances changent, pas a chaque commit.
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY web/ ./

# VITE_API_URL vide : le client appelle /api/v1 en relatif, sur l'origine qui
# a servi la page. Aucune URL d'environnement n'entre dans le bundle, la meme
# image vaut pour n'importe quel domaine.
ENV VITE_API_URL=""

RUN npm run routes:gen \
    && npm run lint \
    && npx tsc -b \
    && npm run test \
    && npx vite build

# =============================================================================
# Stage 2 : API — compilation
# =============================================================================
FROM golang:1.26-alpine AS api

WORKDIR /src/api

COPY api/go.mod api/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY api/ ./
# Version publiee, tenue par scripts/release.sh : elle voyage avec le code, donc
# un build Coolify la connait sans build arg a maintenir.
COPY VERSION /src/VERSION

# Fourni par Coolify (build arg SOURCE_COMMIT) quand « Include Source Commit
# in Build » est coche ; sinon la version reste anonyme, rien ne casse.
ARG SOURCE_COMMIT=none

# CGO_ENABLED=0 produit un binaire statique : l'image finale n'a besoin
# d'aucune libc. -w -s retirent la table des symboles et le DWARF.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    VERSION="$(cat /src/VERSION)" \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${SOURCE_COMMIT}" \
    -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" \
    -o /out/seed ./cmd/seed

# =============================================================================
# Stage 3 : API — tests (bloque le build)
# =============================================================================
FROM api AS api-test

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    if [ -n "$(gofmt -l .)" ]; then echo "Fichiers non formates :"; gofmt -l .; exit 1; fi \
    && go vet ./... \
    && go test ./... \
    && touch /out/tests-passed

# =============================================================================
# Stage 4 : image de production
# =============================================================================
FROM alpine:3.22 AS production

# ca-certificates : appels HTTPS sortants (recuperation des logos d'apps)
# tzdata        : Europe/Paris pour les dates metier
# curl          : sonde du HEALTHCHECK
RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -g 1000 app \
    && adduser -u 1000 -G app -s /bin/sh -D app

WORKDIR /app

# Porte d'entree : force le stage de tests a s'executer avant l'assemblage.
# S'ils echouent, ce COPY n'a pas de source et le build casse.
COPY --from=api-test /out/tests-passed /tmp/tests-passed

COPY --from=api /out/api /app/api
# La commande d'amorcage voyage avec l'image : un deploiement neuf a une table
# users vide, et sans elle personne ne peut se connecter.
COPY --from=api /out/seed /app/seed

# Le front appartient a l'utilisateur app : le serveur de fichiers y depose la
# version compressee de chaque asset au premier acces.
COPY --from=web --chown=app:app /src/web/dist /app/public

# Pieces jointes : a monter sur un volume persistant, sinon elles
# disparaissent au prochain deploiement. Le repertoire est cree ici avec le
# bon proprietaire pour qu'un volume neuf en herite.
RUN mkdir -p /app/data/files && chown -R app:app /app/data

# Valeurs par defaut propres a l'image. Tout le reste — secrets, URLs de base,
# origines — vient de l'environnement Coolify.
ENV TZ=Europe/Paris \
    APP_ENV=production \
    PORT=8080 \
    STATIC_DIR=/app/public \
    FILES_DIR=/app/data/files

USER app

EXPOSE 8080

# Sonde de VIVACITE, donc /health/live : elle ne touche aucune dependance.
# Un echec ici fait redemarrer le conteneur, et redemarrer l'API ne repare pas
# une base injoignable — cela ne fait qu'ajouter une coupure a la panne.
# /health/ready (Postgres + Redis) sert a la supervision, pas au redemarrage.
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/health/live || exit 1

ENTRYPOINT ["/app/api"]
