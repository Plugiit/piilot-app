# syntax=docker/dockerfile:1

# =============================================================================
# Stage 1 : compilation
# =============================================================================
FROM golang:1.26-alpine AS build

WORKDIR /src

# Manifestes d'abord : cette couche ne rejoue que si les dependances bougent,
# pas a chaque commit.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none

# CGO_ENABLED=0 produit un binaire statique : l'image finale n'a besoin
# d'aucune libc. -w -s retirent la table des symboles et le DWARF (~30 % de
# taille en moins), inutiles en production ou les traces viennent des logs.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /out/api ./cmd/api

# La commande d'amorcage voyage avec l'image. Sans elle, un deploiement neuf
# est inutilisable : la table users est vide et aucune connexion n'est
# possible, meme avec /auth/login parfaitement implemente. Elle n'ouvre aucune
# surface supplementaire — qui peut l'executer a deja un shell dans le
# conteneur, donc DATABASE_URL et JWT_SECRET dans son environnement.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/seed ./cmd/seed

# =============================================================================
# Stage 2 : tests (bloque le build — un echec annule le deploiement)
# =============================================================================
FROM build AS test

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    if [ -n "$(gofmt -l .)" ]; then echo "Fichiers non formates :"; gofmt -l .; exit 1; fi \
    && go vet ./... \
    && go test ./... \
    && touch /out/tests-passed

# =============================================================================
# Stage 3 : image de production
# =============================================================================
FROM alpine:3.22 AS production

# ca-certificates : appels HTTPS sortants (GitHub, Google, Qonto...)
# tzdata        : Europe/Paris pour les dates metier
# curl          : sonde du HEALTHCHECK
RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -g 1000 app \
    && adduser -u 1000 -G app -s /bin/sh -D app

ENV TZ=Europe/Paris

WORKDIR /app

# Porte d'entree : force le stage test a s'executer avant l'assemblage de
# l'image. Si les tests echouent, ce COPY n'a pas de source et le build casse.
COPY --from=test /out/tests-passed /tmp/tests-passed

COPY --from=build /out/api /app/api
COPY --from=build /out/seed /app/seed

USER app

EXPOSE 8080

# Sonde de VIVACITE, donc /health/live : elle ne touche aucune dependance.
# Un echec ici fait redemarrer le conteneur, et redemarrer l'API ne repare pas
# une base injoignable — cela ne fait qu'ajouter une coupure a la panne. La
# sonde de DISPONIBILITE, /health/ready, verifie Postgres et Redis ; c'est
# Coolify qui l'interroge pour decider de router le trafic.
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/health/live || exit 1

ENTRYPOINT ["/app/api"]
