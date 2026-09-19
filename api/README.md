# plugiit-api-go

API Plugiit — backend Go consommé par `plugiit-app-reactjs`.

## Stack

| Role | Choix |
|---|---|
| HTTP | Fiber v3 |
| Base de donnees | PostgreSQL 17 via pgx v5 |
| Requetes typees | sqlc (SQL ecrit a la main, structs generees) |
| Migrations | golang-migrate, embarquees dans le binaire |
| Cache / files | Redis |
| Auth | JWT HS256 en cookie httpOnly |
| Logs | slog, JSON structure |

## Demarrer

```bash
cp .env.example .env
# JWT_SECRET doit faire 32 octets minimum :
#   openssl rand -base64 48

make up        # Postgres + Redis
make dev       # API avec rechargement a chaud sur :8080
```

Les migrations s'appliquent au demarrage (`RUN_MIGRATIONS=true`).

## Commandes

| Commande | Effet |
|---|---|
| `make dev` | API en rechargement a chaud (air) |
| `make check` | fmt + vet + tests — a lancer avant de pousser |
| `make test` | Tests avec detecteur de courses |
| `make sqlc` | Regenere `internal/repository/db` depuis `queries/*.sql` |
| `make migrate-new name=...` | Cree une paire de fichiers de migration |
| `make docker-build` | Construit l'image de production (tests inclus) |

`sqlc` et `migrate` s'executent via Docker : rien a installer localement.

## Architecture

```
cmd/api/              point d'entree, montage des dependances
internal/
├── config/           chargement et validation de l'environnement
├── domain/           entites et erreurs metier, sans dependance HTTP ni SQL
├── handler/          endpoints : valider, appeler un usecase, serialiser
├── middleware/       erreurs, authentification, CORS
├── repository/       pools Postgres/Redis, migrations, requetes generees
├── security/         signature des jetons
└── usecase/          orchestration metier
migrations/           SQL versionne, embarque dans le binaire
queries/              source sqlc
```

### Conventions

- **Un endpoint par vue.** `GET /admin/projects/:id/tasks` renvoie l'onglet
  Taches, pas l'objet projet complet. C'est ce qui garde 1 a 3 requetes SQL
  par appel et un payload de quelques kilo-octets.
- **Toute liste est paginee** cote serveur. Aucune requete sans `LIMIT`.
- **Aucun appel externe dans le cycle de requete.** Un job remplit une table
  locale, l'endpoint lit la table.
- **Les agregats de dashboard sont precalcules** (Redis ou vue materialisee),
  jamais recalcules par `COUNT(*)` au rendu.
- **Format d'erreur unique** : `{code, message, details}`. Le front branche sur
  `code`, jamais sur `message`.

## Endpoints

| Methode | Chemin | Auth |
|---|---|---|
| GET | `/health/live` | non — sonde de liveness, ne touche aucune dependance |
| GET | `/health/ready` | non — 200 seulement si Postgres et Redis repondent |
| POST | `/api/v1/auth/login` | non |
| POST | `/api/v1/auth/logout` | non |
| POST | `/api/v1/auth/refresh` | non |
| GET | `/api/v1/auth/me` | oui |
| GET | `/api/v1/admin/*` | oui — roles `admin` ou `team` |

Les handlers metier renvoient `501 NOT_IMPLEMENTED` tant qu'ils ne sont pas
ecrits. Un test verifie qu'aucune route sous `/api/v1/admin` n'est joignable
sans authentification.

## Deploiement (Coolify)

Application de type **Dockerfile**, port interne **8080**.

Variables a definir :

| Variable | Exemple |
|---|---|
| `APP_ENV` | `production` |
| `DATABASE_URL` | `postgres://user:pass@postgres:5432/plugiit?sslmode=disable` |
| `REDIS_URL` | `redis://redis:6379/0` |
| `JWT_SECRET` | 32 octets minimum |
| `ADMIN_ORIGINS` | `https://admin.plugiit.com` |
| `COOKIE_DOMAIN` | `.plugiit.com` |
| `RUN_MIGRATIONS` | `true` |

`ADMIN_ORIGINS` n'accepte pas de wildcard : les cookies exigent une origine
explicite.

Le build execute `go vet` et les tests ; l'image n'est produite que s'ils
passent. Le healthcheck interroge `/health/ready`, donc Coolify ne bascule le
trafic que quand la base et Redis repondent.
