# Piilot

**L'outil de pilotage de l'agence Plugiit** : projets, tâches, temps passé,
livrables, tickets et relation client dans une seule application, doublée
d'un portail pour les clients de l'agence.

![Piilot — tableau de bord (maquette)](docs/images/maquette-tableau-de-bord.jpg)

<sub>Maquette de design. Les captures de la suite du document viennent de
l'application réelle, sur une instance remplie de données fictives.</sub>

---

- [Ce que fait Piilot](#ce-que-fait-piilot)
- [État d'avancement](#état-davancement)
- [Roadmap](#roadmap)
- [Publier une version](#publier-une-version)
- [Architecture](#architecture)
- [Installation avec Coolify](#installation-avec-coolify)
- [Installation sans Coolify](#installation-sans-coolify)
- [Premier compte](#premier-compte)
- [Configuration](#configuration)
- [Exploitation](#exploitation)
- [Développement](#développement)
- [Structure du dépôt](#structure-du-dépôt)

---

## Ce que fait Piilot

Piilot remplace, module par module, la plateforme interne historique de
l'agence. Les modules sont **repensés et réécrits**, pas portés : chaque écran
correspond à un seul appel d'API, qui renvoie exactement ce que l'écran affiche.

Une seule application, trois espaces :

| Espace | Pour qui | Contenu |
|---|---|---|
| **Back-office** | L'équipe de l'agence (rôles `admin` et `team`) | Gestion de projet, CRM, paramètres |
| **Portail client** | Les clients de l'agence (rôle `client`) | Suivi de leurs projets et tickets |
| **Connexion** | Tout le monde | Une page commune, qui redirige chacun vers son espace |

![Page de connexion](docs/images/login.jpg)

### Tableau de bord

Les chiffres clés de l'agence (projets, clients, heures vendues) et
l'avancement des tâches par service. La barre latérale donne des raccourcis
vers les projets favoris.

> La carte d'activité, le temps facturable et l'agenda de cet écran sont
> encore des emplacements factices. Ils seront retirés en 0.4, puis branchés
> sur des données réelles en 0.7 (voir la [roadmap](ROADMAP.md)).

![Tableau de bord](docs/images/dashboard.jpg)

### Projets

Tous les projets de l'agence en cartes : statut (cadrage, production, attente
client, livré), équipe, échéance, priorité et avancement. Recherche, filtres
par statut et par client, tri, pagination côté serveur.

![Liste des projets](docs/images/projets.jpg)

La fiche projet rassemble le client et son contact principal, les services
vendus, les dates, l'avancement, les documents et les liens (Figma,
production, préproduction). Les tâches et les tickets du projet s'affichent
dessous, en liste ou en kanban.

![Fiche projet](docs/images/projet-taches.jpg)

Les paramètres du projet couvrent l'équipe, le budget en heures vendues, le
planning et la suppression.

### Tâches

Chaque tâche a ses responsables, son échéance, sa priorité, ses services, sa
charge, ses pièces jointes, ses sous-tâches et un fil de commentaires. Elle
s'ouvre dans un panneau latéral, sans quitter la liste.

![Détail d'une tâche](docs/images/projet-tache-detail.jpg)

La vue **Tâches** rassemble les tâches de tous les projets dans un kanban :
à faire, en cours, en revue, terminé. Une échéance dépassée s'affiche en rouge.

![Kanban des tâches](docs/images/taches-kanban.jpg)

### Suivi du temps

Chacun pointe son temps à la journée : projet, tâche, service, durée et note.
Le total du jour s'affiche en haut à droite.

![Saisie du temps](docs/images/temps-saisie.jpg)

### Livrables

L'agence dépose un livrable (maquette, prototype, recette), qui est ensuite
validé ou renvoyé avec des retours. Chaque nouvelle soumission crée une
version, et l'historique est conservé. La validation par le client lui-même
arrivera avec le portail.

![Livrables](docs/images/livrables.jpg)

### Tickets

Anomalies, évolutions et demandes d'assistance, rattachées à un projet. Chaque
ticket suit un cycle de vie complet (backlog → à faire → en cours → en revue
→ prêt à déployer → terminé). Il a un fil de discussion avec des messages
internes que le client ne voit pas, et un historique des changements. Vues en
tableau, par projet ou par statut.

![Détail d'un ticket](docs/images/ticket-detail.jpg)

### CRM

Les clients de l'agence et leurs contacts. Le pipeline suit chaque client de
la piste jusqu'au client actif, en veille ou perdu, en kanban ou en liste,
filtrable par chargé de compte.

![Pipeline clients](docs/images/clients-kanban.jpg)

La fiche client regroupe ses contacts, ses projets, son identité d'entreprise
(SIRET, TVA, adresse) et les comptes de portail ouverts à ses équipes.

![Fiche client](docs/images/client-fiche.jpg)

### Et aussi

- **Notifications en temps réel**, poussées par un flux SSE : tâche créée,
  assignée, commentée, changement de statut ou d'échéance, nouveau projet.
- **Services** : le référentiel des prestations vendues (design, développement,
  SEO…), avec une couleur par service, réutilisé partout.
- **Apps de la barre latérale** : les outils de l'agence en un clic. Leur logo
  est récupéré automatiquement par une tâche de fond.
- **Compte** : profil, photo, mot de passe.
- **Rôles et permissions en base** : trois rôles système (`admin`, `team`,
  `client`) et 17 permissions. Un droit retiré prend effet immédiatement, sans
  attendre l'expiration d'une session.

## État d'avancement

| Module | État |
|---|---|
| Connexion, sessions, rôles et permissions | ✅ En place |
| Projets, tâches, sous-tâches, commentaires, pièces jointes | ✅ En place |
| Tickets | ✅ En place |
| Livrables et validation | ✅ En place |
| Saisie du temps | ✅ En place |
| CRM : clients, contacts, pipeline | ✅ En place |
| Services, apps de la barre latérale, notifications | ✅ En place |
| Rapports de temps | 🚧 Écran réservé, pas encore développé |
| CRM : interactions | 🚧 Écran réservé, pas encore développé |
| Modèles de projet, gestion des comptes | 🚧 Écrans réservés, pas encore développés |
| Planning | ↗️ Renvoie vers l'agenda partagé de l'agence |
| **Portail client** | 🚧 Espace et connexion en place, écrans de suivi à venir |

Hors périmètre, et donc absents par choix : facturation et comptabilité,
monitoring SEO, CMS et blog, RH, multi-agence, application mobile.

## Roadmap

Étapes jusqu'à la V1, release par release. Ce tableau est recalculé depuis le
fichier `VERSION` à chaque release : il indique toujours la version en cours.
Historique détaillé dans [CHANGELOG.md](CHANGELOG.md).

<!-- roadmap:readme -->
**Version actuelle : `0.2.0`** — détail de chaque release dans [ROADMAP.md](ROADMAP.md).

| Statut | Version | Nom | Espace | Objectif |
|---|---|---|---|---|
| ✅ Livrée | 0.1.0 | Socle | Tous | Connexion, sessions, rôles et permissions |
| 📍 Actuelle | **0.2.0** | Gestion de projet | Admin | Projets, tâches, notifications, compte |
| ⏳ À venir | 0.3.0 | Back-office PM et CRM | Admin | Tickets, livrables, temps, CRM, image unique |
| ⏳ À venir | 0.4.0 | En production | Admin | Utiliser Piilot en interne, sur des données réelles et sauvegardées |
| ⏳ À venir | 0.5.0 | Comptes et e-mails | Tous | Inviter, réinitialiser, gérer les comptes sans shell |
| ⏳ À venir | 0.6.0 | Espace team | Team | Une journée de production sans passer par les écrans d'admin |
| ⏳ À venir | 0.7.0 | Pilotage | Admin | Rapports, budgets, jalons : ne plus dépendre de l'ancienne plateforme |
| ⏳ À venir | 0.8.0 | Portail : suivi et validation | Client | Un client suit son projet et valide un livrable dans Piilot |
| ⏳ À venir | 0.9.0 | Portail : tickets | Client | Le support client passe par Piilot |
| ⏳ À venir | 0.10.0 | Conformité et durcissement | Tous | RGPD, audit, sécurité du portail, tests de bout en bout |
| ⏳ À venir | 1.0.0-rc.N | Recette | Tous | Clients pilotes, corrections |
| ⏳ À venir | 1.0.0 | V1 | Tous | Critères de sortie remplis |
<!-- /roadmap:readme -->

## Publier une version

La version courante est dans le fichier `VERSION`. Le binaire l'embarque à la
compilation et `/health/live` l'affiche.

```bash
make release V=minor ARGS=--dry-run   # aperçu : version visée et notes, rien n'est modifié
make release V=minor                  # publie
```

`V` vaut `patch`, `minor`, `major` ou une version explicite (`0.4.0`,
`1.0.0-rc.1`). Le script :

1. vérifie qu'on est sur `master`, que l'arbre est propre, à jour avec
   `origin`, et que le tag n'existe pas ;
2. lance `make check`, qui exige les dépendances du front
   (`cd web && npm ci` une fois) ;
3. prépare les notes de version, puis te les soumet : publier, éditer,
   régénérer ou abandonner ;
4. met à jour `VERSION`, `CHANGELOG.md` et le statut des releases dans la
   [roadmap](#roadmap), committe `chore(release): vX.Y.Z` et pose le tag
   annoté ;
5. pousse `master` et le tag ensemble.

### Notes de version

Elles viennent, par ordre de priorité :

1. d'une section `## [X.Y.Z]` déjà rédigée dans `CHANGELOG.md`, reprise
   telle quelle ;
2. sinon, d'un **brouillon rédigé par Claude Code** (`claude -p`), en
   français. Claude reçoit les commits depuis la dernière version, les
   fichiers modifiés, les migrations ajoutées, les routes d'API ajoutées ou
   retirées, les changements de configuration et l'objectif de la version
   dans la roadmap. Il n'a accès à aucun outil : il rédige à partir de ce
   contexte, sans lire ni modifier le dépôt. La consigne de rédaction est
   dans [`scripts/release-notes.md`](scripts/release-notes.md) ;
3. sans Claude Code, ou avec `ARGS=--no-ai`, d'une liste des commits groupée
   par type.

Le brouillon suit toujours la même structure : introduction, points forts,
nouveautés par espace (Admin, Team, Client), améliorations, corrections,
technique, et ce qu'il faut savoir pour le déploiement. **Il se relit avant
publication** : Claude peut se tromper, et le texte validé devient
définitif, dans le `CHANGELOG` comme sur GitHub. `CLAUDE_MODEL=opus make
release V=minor` change le modèle utilisé.

### Sur GitHub

Le tag déclenche `.github/workflows/release.yml`. Le workflow vérifie que le
tag correspond à `VERSION` et reconstruit l'image, ce qui rejoue toutes les
vérifications. Il publie ensuite la release GitHub :

- **titre** : `Piilot v0.4.0 — En production`, repris de la roadmap ;
- **corps** : la section du `CHANGELOG`, suivie d'un lien de comparaison
  avec la version précédente et de la liste repliable des commits.

**Une version dont l'image ne se construit pas n'a pas de release.** Une
pré-version (`-rc.N`) est marquée comme telle.

Autres options : `--skip-checks` (ne relance pas `make check`), `--no-push`
(tag local seulement).

## Architecture

```
                 ┌──────────────────────────── conteneur app ─┐
  navigateur ──▶ │  binaire Go (Fiber v3)                     │
     HTTPS       │   ├── /api/v1/*     API JSON               │──▶ PostgreSQL 17
  (Traefik ou    │   ├── /health/*     sondes                 │
   reverse       │   ├── /openapi.json contrat de l'API       │──▶ Redis 8
   proxy)        │   └── /*            front React (fichiers) │
                 └────────────────────────────────────────────┘
```

**Une seule image.** Le binaire Go sert l'API et le build du front. Front et
API partagent la même origine : pas de CORS, pas d'URL d'API figée dans le
bundle, un seul domaine à déclarer. La même image vaut pour n'importe quel
domaine.

| Côté | Choix |
|---|---|
| API | Go 1.26, Fiber v3, pgx v5, sqlc, golang-migrate (migrations embarquées) |
| Front | React 19, Vite 8, TanStack Router et Query, shadcn/ui, Tailwind CSS v4 |
| Données | PostgreSQL 17, Redis (limitation de débit, bus de notifications) |
| Contrat | OpenAPI servi sur `/openapi.json` ; le front en génère ses types |

Sécurité :

- **Session** : jeton JWT dans un cookie httpOnly, jamais en localStorage.
  Le rafraîchissement fait tourner le jeton et détecte le rejeu : réutiliser
  un jeton déjà consommé révoque toutes les sessions du compte.
- **Mots de passe** : bcrypt (coût 12).
- **Connexion** : limitation de débit, 30 tentatives par IP et 5 par compte
  sur 15 minutes.
- **Conteneur** : utilisateur non-root, binaire statique sur Alpine.
- **Build** : l'image n'est produite que si les vérifications du front
  (lint, types, tests) et de l'API (gofmt, vet, tests) passent.

## Installation avec Coolify

C'est la cible de déploiement. Coolify construit l'image depuis le dépôt,
génère les secrets et les expose dans son interface. **Aucune valeur n'est à
écrire dans un fichier.**

**1. Créer la ressource**

*Projects → votre projet → + New → Private Repository (GitHub App)* ou
*Public Repository* :

| Champ | Valeur |
|---|---|
| Repository | `Plugiit/piilot-app` |
| Branch | `master` |
| Build Pack | **Docker Compose** |
| Docker Compose Location | `/docker-compose.yml` |

Coolify lit le fichier et crée trois services : `app`, `postgres` et `redis`.

**2. Attribuer le domaine**

Sur le service **app**, dans *Domains* : `https://piilot.example.fr:8080`.

Le suffixe `:8080` n'est pas le port public : il indique à Traefik le port
interne du conteneur. Les visiteurs arrivent sur `https://piilot.example.fr`,
en 443. Coolify s'occupe du certificat Let's Encrypt.

**3. Vérifier les variables**

Onglet *Environment Variables* : toutes les variables du compose y sont,
pré-remplies ou générées. Rien n'est obligatoire à saisir pour un premier
déploiement. Le détail est dans [Configuration](#configuration).

**4. Déployer**

*Deploy*. Le build prend quelques minutes : il installe les dépendances,
vérifie et teste les deux côtés, compile, puis démarre. Les migrations
s'appliquent au démarrage de l'API.

Si le build échoue, aucune image n'est produite et la version précédente reste
en ligne. Les logs du build indiquent l'étape en cause.

**5. Créer le premier compte**

Voir [Premier compte](#premier-compte).

**Mises à jour** : activer *Auto Deploy* (webhook GitHub) pour redéployer à
chaque push sur `master`, ou déclencher *Redeploy* à la main.

> **Ne jamais régénérer `SERVICE_PASSWORD_POSTGRES` après le premier
> déploiement.** Postgres n'applique le mot de passe qu'à la création de son
> volume : le changer côté Coolify casse la connexion sans changer celui de
> la base.

## Installation sans Coolify

Il suffit d'un serveur avec Docker et Docker Compose v2, et d'un reverse proxy
pour le HTTPS.

**1. Récupérer le dépôt et préparer l'environnement**

```bash
git clone https://github.com/Plugiit/piilot-app.git
cd piilot-app
cp .env.example .env
```

Remplir `.env`. Chaque secret se génère avec `openssl` :

```bash
openssl rand -hex 16   # SERVICE_PASSWORD_POSTGRES, SERVICE_PASSWORD_REDIS
openssl rand -hex 32   # SERVICE_PASSWORD_64_JWT
```

Renseigner `SERVICE_URL_APP` et `SERVICE_URL_APP_8080` avec l'URL publique
finale, par exemple `https://piilot.example.fr`.

**2. Démarrer**

```bash
docker compose -f docker-compose.yml -f docker-compose.selfhost.yml up -d --build
```

`docker-compose.selfhost.yml` ajoute la seule chose que Coolify fait à sa
place : il publie le port de l'application sur `127.0.0.1:8080`.

Vérifier :

```bash
curl -s http://127.0.0.1:8080/health/ready
# {"checks":{"database":"ok","redis":"ok"},"status":"ok"}
```

**3. Mettre un reverse proxy devant**

L'application parle HTTP en clair. En production, ses cookies de session sont
`Secure` : **sans HTTPS, la connexion échoue.** Avec Caddy, le certificat est
automatique :

```caddyfile
piilot.example.fr {
    reverse_proxy 127.0.0.1:8080
}
```

Avec nginx, penser à désactiver la mise en tampon pour le flux de
notifications :

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_buffering off;          # flux SSE /api/v1/admin/notifications/stream
    client_max_body_size 30m;     # pièces jointes (MAX_UPLOAD_MIB + marge)
}
```

**Mettre à jour**

```bash
git pull
docker compose -f docker-compose.yml -f docker-compose.selfhost.yml up -d --build
```

### Variante : l'image seule, avec Postgres et Redis existants

L'image n'a besoin que de variables d'environnement et d'un volume :

```bash
docker build -t piilot-app .

docker run -d --name piilot --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v piilot-files:/app/data/files \
  -e PUBLIC_BASE_URL=https://piilot.example.fr \
  -e ADMIN_ORIGINS=https://piilot.example.fr \
  -e DATABASE_URL='postgres://piilot:MOT_DE_PASSE@db.interne:5432/piilot?sslmode=require' \
  -e REDIS_URL='redis://:MOT_DE_PASSE@redis.interne:6379/0' \
  -e JWT_SECRET="$(openssl rand -hex 32)" \
  piilot-app
```

> Générer `JWT_SECRET` une fois et le conserver : le changer déconnecte tout
> le monde.

## Premier compte

Une installation neuve a une table `users` vide. La commande d'amorçage est
livrée dans l'image. Depuis un terminal ouvert sur le conteneur `app` (dans
Coolify : *Terminal* → conteneur `app`) :

```sh
SEED_PASSWORD='un-mot-de-passe-solide' /app/seed \
    -email=moi@exemple.fr -firstname=Prénom -lastname=Nom -role=admin
```

Hors Coolify :

```bash
docker compose exec -e SEED_PASSWORD='un-mot-de-passe-solide' app \
    /app/seed -email=moi@exemple.fr -firstname=Prénom -lastname=Nom -role=admin
```

- Le mot de passe passe par `SEED_PASSWORD` et non par un drapeau, pour ne pas
  apparaître dans `ps`. Sans lui, un mot de passe aléatoire est généré et
  affiché **une seule fois**.
- La commande refuse d'écraser un compte existant.
- Rôles possibles : `admin` (toutes les permissions), `team`, `client`.

## Configuration

Toute la configuration passe par des variables d'environnement, validées au
démarrage. Une valeur invalide arrête l'API avec un message explicite plutôt
que de la laisser répondre 500 à la première requête.

### Générées par Coolify

À fournir dans `.env` hors Coolify.

| Variable | Rôle |
|---|---|
| `SERVICE_URL_APP` / `SERVICE_URL_APP_8080` | URL publique : alimente `PUBLIC_BASE_URL` et `ADMIN_ORIGINS` |
| `SERVICE_USER_POSTGRES` / `SERVICE_PASSWORD_POSTGRES` | Identifiants Postgres |
| `SERVICE_PASSWORD_REDIS` | Mot de passe Redis |
| `SERVICE_PASSWORD_64_JWT` | Secret de signature des jetons |

### Modifiables

| Variable | Défaut | Rôle |
|---|---|---|
| `APP_ENV` | `production` | `production`, `staging` ou `development` (cookies non `Secure`, logs verbeux) |
| `LOG_LEVEL` | `info` | `debug` pour plus de détail |
| `COOKIE_DOMAIN` | *(vide)* | Vide : cookie lié au seul domaine de l'app, le réglage le plus sûr |
| `POSTGRES_DB` | `piilot` | Nom de la base |
| `RUN_MIGRATIONS` | `true` | Applique les migrations au démarrage |
| `MAX_UPLOAD_MIB` | `25` | Taille maximale d'une pièce jointe |
| `ACCESS_TOKEN_TTL` | `15m` | Durée du jeton d'accès |
| `REFRESH_TOKEN_TTL` | `720h` | Durée d'une session sans reconnexion (30 jours) |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` | `30s` | Timeouts HTTP |
| `SHUTDOWN_TIMEOUT` | `15s` | Délai laissé aux requêtes en cours à l'arrêt |

### Fixées par l'image

À ne changer qu'en connaissance de cause : `PORT=8080`,
`STATIC_DIR=/app/public` (build du front), `FILES_DIR=/app/data/files`
(pièces jointes), `TZ=Europe/Paris`.

## Exploitation

### Volumes

| Volume | Contenu | À sauvegarder |
|---|---|---|
| `piilot-postgres` | Base de données | **Oui** |
| `piilot-files` | Pièces jointes, photos de profil, logos | **Oui** |
| `piilot-redis` | Compteurs de connexion, bus de notifications | Non, reconstructible |

### Sauvegardes

La base et les fichiers vont ensemble : une base restaurée sans ses pièces
jointes pointe vers des fichiers absents.

```bash
# Base
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -Fc "$POSTGRES_DB"' \
    > piilot-$(date +%F).dump

# Pièces jointes
docker run --rm -v piilot-app_piilot-files:/data -v "$PWD":/out alpine \
    tar czf /out/piilot-files-$(date +%F).tar.gz -C /data .
```

Le nom réel des volumes est préfixé par le nom du projet Compose
(`piilot-app_` par défaut hors Coolify, un identifiant de ressource sous
Coolify) : `docker volume ls | grep piilot` le donne. Ces commandes se
planifient avec cron, et les archives doivent partir hors du serveur.

Une sauvegarde n'a de valeur que si sa restauration a été testée.

### Sondes

| Route | Réponse | Usage |
|---|---|---|
| `/health/live` | 200 dès que le processus répond | `HEALTHCHECK` du conteneur. Ne touche aucune dépendance : redémarrer l'API ne réparerait pas une base injoignable |
| `/health/ready` | 200 seulement si Postgres **et** Redis répondent | Supervision (Uptime Kuma…) |

### Logs

JSON structuré sur la sortie standard, lisibles dans Coolify ou avec
`docker compose logs -f app`.

## Développement

Prérequis : Go 1.26, Node 24, Docker.

```bash
make up        # Postgres + Redis en conteneurs (docker-compose.dev.yml)
make dev-api   # API sur :8080, rechargement à chaud
make dev-web   # front sur :5173, proxy /api vers l'API
```

Au premier lancement, copier `api/.env.example` en `api/.env` et y mettre un
`JWT_SECRET` d'au moins 32 caractères. En développement, c'est Vite qui sert
le front (`STATIC_DIR` reste vide).

| Commande | Effet |
|---|---|
| `make check` | Vérification complète : gofmt, vet, tests Go, routes, lint, types, tests front |
| `make docker-build` | Construit l'image de production, vérifications incluses |
| `make -C api sqlc` | Régénère le code Go des requêtes SQL |
| `make -C api migrate-new name=...` | Crée une migration |
| `make -C api test-integration` | Tests d'intégration contre la base locale |
| `cd web && npm run api:types` | Régénère les types du front depuis `/openapi.json` |

La CI (`.github/workflows/ci.yml`) construit l'image de production, ce qui
exécute toutes les vérifications. Elle lance aussi les tests d'intégration de
l'API contre un vrai Postgres.

Principes à respecter en contribuant :

- **Un endpoint par écran**, jamais par entité.
- **Toute liste est paginée** côté serveur.
- **Aucun appel externe pendant une requête HTTP** : une tâche de fond remplit
  une table, l'endpoint la lit.
- **Les agrégats sont précalculés**, jamais recalculés par `COUNT(*)` à
  l'affichage.

## Structure du dépôt

```
api/                        API Go
├── cmd/api/                point d'entrée
├── cmd/seed/               création du premier compte
├── internal/               config, domaine, handlers, usecases, repository…
├── migrations/             SQL versionné, embarqué dans le binaire
├── queries/                requêtes SQL (source de sqlc)
└── openapi/                contrat OpenAPI
web/                        front React
├── src/routes/             écrans (routage par fichiers)
├── src/features/           logique par module
└── src/components/         composants partagés (shadcn/ui)
scripts/release.sh          publication d'une version
scripts/roadmap.sh          statut des releases, recalculé depuis VERSION
docs/images/                captures du README
Dockerfile                  image unique : front + API
docker-compose.yml          déploiement : app + Postgres + Redis (Coolify)
docker-compose.selfhost.yml publication du port hors Coolify
docker-compose.dev.yml      Postgres + Redis pour le développement
.env.example                variables pour une installation hors Coolify
VERSION                     version courante, tenue par le script de release
CHANGELOG.md                historique des versions
ROADMAP.md                  étapes jusqu'à la V1
```
