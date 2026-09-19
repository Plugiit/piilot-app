.DEFAULT_GOAL := help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -w -s -X main.version=$(VERSION) -X main.commit=$(COMMIT)

# sqlc et migrate ne sont pas installes en local : on les execute via Docker
# pour que la version de l'outil soit la meme partout.
SQLC    := docker run --rm -v $(PWD):/src -w /src sqlc/sqlc:latest
MIGRATE := docker run --rm -v $(PWD)/migrations:/migrations --network host migrate/migrate:latest

DATABASE_URL ?= postgres://plugiit:plugiit@localhost:5432/plugiit?sslmode=disable

.PHONY: help
help: ## Affiche cette aide
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: up
up: ## Demarre Postgres + Redis
	docker compose -f ../docker-compose.dev.yml up -d

.PHONY: down
down: ## Arrete Postgres + Redis
	docker compose -f ../docker-compose.dev.yml down

.PHONY: dev
dev: ## Lance l'API avec rechargement a chaud (air)
	@command -v air >/dev/null || go install github.com/air-verse/air@latest
	air

.PHONY: run
run: ## Lance l'API une fois
	go run ./cmd/api

.PHONY: seed
seed: ## Cree le premier compte : make seed EMAIL=moi@plugiit.com [ROLE=admin]
	@test -n "$(EMAIL)" || (echo "usage: make seed EMAIL=... [ROLE=admin]" && exit 1)
	go run ./cmd/seed -email=$(EMAIL) $(if $(ROLE),-role=$(ROLE),)

.PHONY: build
build: ## Compile le binaire dans bin/api
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/api ./cmd/api

.PHONY: test
test: ## Lance les tests
	go test -race ./...

.PHONY: test-integration
test-integration: ## Tests d'integration (exige make up et les migrations)
	TEST_DATABASE_URL="$(DATABASE_URL)" go test -tags=integration -count=1 ./...

.PHONY: cover
cover: ## Lance les tests avec couverture
	go test -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

.PHONY: check
check: ## fmt + vet + tests (a lancer avant de pousser)
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...
	go test ./...

.PHONY: sqlc
sqlc: ## Regenere internal/repository/db depuis queries/*.sql
	$(SQLC) generate

.PHONY: migrate-new
migrate-new: ## Cree une migration : make migrate-new name=add_projects
	@test -n "$(name)" || (echo "usage: make migrate-new name=..." && exit 1)
	$(MIGRATE) create -ext sql -dir /migrations -seq $(name)

.PHONY: migrate-up
migrate-up: ## Applique les migrations en attente
	$(MIGRATE) -path /migrations -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Annule la derniere migration
	$(MIGRATE) -path /migrations -database "$(DATABASE_URL)" down 1
