.DEFAULT_GOAL := help

VERSION := $(shell cat VERSION)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)

.PHONY: help
help: ## Affiche cette aide
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: up
up: ## Demarre Postgres + Redis pour le developpement
	docker compose -f docker-compose.dev.yml up -d

.PHONY: down
down: ## Arrete Postgres + Redis
	docker compose -f docker-compose.dev.yml down

.PHONY: dev-api
dev-api: ## API sur :8080, rechargement a chaud
	$(MAKE) -C api dev

.PHONY: dev-web
dev-web: ## Front sur :5173, proxy /api vers :8080
	cd web && npm run dev

.PHONY: check
check: ## Verification complete des deux cotes
	$(MAKE) -C api check
	cd web && npm run check

.PHONY: docker-build
docker-build: ## Construit l'image de production (tests inclus)
	docker build --build-arg SOURCE_COMMIT=$(COMMIT) -t piilot-app:$(VERSION) .

.PHONY: release
release: ## Publie une version : make release V=patch|minor|major|X.Y.Z [ARGS=--dry-run]
	@test -n "$(V)" || (echo "usage: make release V=patch|minor|major|X.Y.Z [ARGS=--dry-run]" && exit 1)
	scripts/release.sh $(V) $(ARGS)
