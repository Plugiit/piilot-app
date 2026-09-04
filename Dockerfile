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

# =============================================================================
# Stage 2 : tests (bloque le build — un echec annule le deploiement)
# =============================================================================
FROM build AS test

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go vet ./... && go test ./... && touch /out/tests-passed

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

USER app

EXPOSE 8080

# La sonde interroge /health/ready : le conteneur n'est declare sain que quand
# Postgres et Redis repondent reellement.
HEALTHCHECK --interval=15s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/health/ready || exit 1

ENTRYPOINT ["/app/api"]
