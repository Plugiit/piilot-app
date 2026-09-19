# piilot-app

API Go et application React dans un seul dépôt, livrées en **une seule image**.

```
api/   API Go — Fiber v3, Postgres, Redis (voir api/README.md)
web/   SPA React — back-office /admin/*, portail /client/*, /login (voir web/README.md)
```

En production, le binaire Go sert l'API sous `/api/v1` et le build du front
pour tout le reste. Front et API partagent la même origine : pas de CORS, pas
d'URL d'API figée dans le bundle, un seul domaine.

## Développement local

```bash
make up        # Postgres + Redis (docker-compose.dev.yml)
make dev-api   # API sur :8080, rechargement à chaud
make dev-web   # front sur :5173, proxy /api vers :8080
make check     # vérification complète des deux côtés
```

En local, `STATIC_DIR` reste vide : c'est Vite qui sert le front.

## Déploiement Coolify

1. **New Resource → Public/Private Repository → Build Pack : Docker Compose**,
   branche `master`, fichier `/docker-compose.yml`.
2. Sur le service **app** : *Domains* → `https://app.example.fr`
   (Coolify ajoute `:8080` en interne grâce à `SERVICE_URL_APP_8080`).
3. Onglet **Environment Variables** : tout y est déjà, pré-rempli ou généré.
   Rien n'est obligatoire à saisir pour un premier déploiement.
4. Cocher *Include Source Commit in Build* pour que le binaire porte
   le commit déployé.
5. **Deploy.** Les migrations s'appliquent au démarrage.
6. Créer le premier compte depuis le terminal du conteneur `app` :

   ```sh
   SEED_PASSWORD='...' /app/seed -email=moi@exemple.fr -firstname=Prénom -lastname=Nom -role=admin
   ```

### Variables

Générées par Coolify au premier déploiement, puis conservées :

| Variable | Rôle |
|---|---|
| `SERVICE_URL_APP` | URL publique — alimente `PUBLIC_BASE_URL` et `ADMIN_ORIGINS` |
| `SERVICE_USER_POSTGRES` / `SERVICE_PASSWORD_POSTGRES` | Identifiants Postgres |
| `SERVICE_PASSWORD_REDIS` | Mot de passe Redis |
| `SERVICE_PASSWORD_64_JWT` | Secret de signature des jetons (64 caractères) |

Modifiables, avec leur valeur par défaut :

| Variable | Défaut | Rôle |
|---|---|---|
| `APP_ENV` | `production` | `production`, `staging` ou `development` |
| `LOG_LEVEL` | `info` | `debug` pour plus de détail |
| `COOKIE_DOMAIN` | *(vide)* | Vide = cookie lié au seul domaine de l'app |
| `POSTGRES_DB` | `piilot` | Nom de la base |
| `RUN_MIGRATIONS` | `true` | Migrations au démarrage |
| `MAX_UPLOAD_MIB` | `25` | Taille maximale d'une pièce jointe |
| `ACCESS_TOKEN_TTL` | `15m` | Durée du jeton d'accès |
| `REFRESH_TOKEN_TTL` | `720h` | Durée du rafraîchissement |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` | `30s` | Timeouts HTTP |
| `SHUTDOWN_TIMEOUT` | `15s` | Fenêtre d'arrêt gracieux |

> **Ne pas régénérer `SERVICE_PASSWORD_POSTGRES` après le premier
> déploiement** : Postgres n'applique le mot de passe qu'à l'initialisation
> du volume. Le changer côté Coolify casse la connexion sans changer celui
> de la base.

### Volumes

| Volume | Contenu |
|---|---|
| `piilot-postgres` | Données Postgres |
| `piilot-redis` | Données Redis (AOF) |
| `piilot-files` | Pièces jointes des projets — **à sauvegarder avec la base** |

### Sondes

- `/health/live` — `HEALTHCHECK` du conteneur, ne touche aucune dépendance.
- `/health/ready` — 200 seulement si Postgres **et** Redis répondent ; à
  brancher sur la supervision.

## Construire l'image à la main

```bash
make docker-build
```

Le build échoue si le lint, le typage ou les tests du front, ou gofmt, vet et
les tests de l'API échouent — aucune image n'est produite sur du rouge. La CI
(`.github/workflows/ci.yml`) construit cette même image et lance en plus les
tests d'intégration de l'API contre un vrai Postgres.
