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
- [Contribuer](#contribuer)
- [Licence](#licence)

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

Les chiffres clés de l'agence (projets, clients, heures vendues), l'activité
de l'année jour par jour (tâches terminées, tickets ouverts, livrables
déposés), le temps facturable et l'avancement des tâches par service.

Deux blocs servent à piloter la charge : les projets en cours les plus avancés
dans leur budget, avec le nombre de projets hors budget ou à surveiller, et le
temps saisi par chaque membre de l'équipe aujourd'hui et depuis lundi. La
barre latérale donne des raccourcis vers les projets favoris.

![Tableau de bord](docs/images/dashboard.jpg)

Le tableau de bord est un outil de direction : il demande la permission
`dashboard.read`, que seul le rôle `admin` détient par défaut.

### Mon travail

L'accueil de l'équipe. Ce qui attend la personne connectée, tous projets
confondus : ses tâches (les retards d'abord), les tickets qu'on lui a confiés,
les livrables à déposer ou à reprendre après des retours du client, ses
projets en cours et le temps saisi depuis lundi.

Le menu ne montre que ce que les droits du compte permettent : un membre de
l'équipe ne voit ni le tableau de bord de l'agence, ni les budgets, ni le
pipeline commercial, ni les rapports de temps, ni l'administration des comptes.

![Mon travail](docs/images/mon-travail.jpg)

### Projets

Tous les projets de l'agence en cartes : statut (cadrage, production, attente
client, livré), équipe, échéance, priorité, avancement et heures consommées
sur heures vendues. Un projet passe « à surveiller » à 80 % de son budget et
« hors budget » au-delà. Recherche, filtres par statut, client et budget, tri,
pagination côté serveur.

![Liste des projets](docs/images/projets.jpg)

La fiche projet rassemble le client et son contact principal, les services
vendus, les dates, l'avancement, les documents et les liens (Figma,
production, préproduction). Les tâches et les tickets du projet s'affichent
dessous, en liste ou en kanban.

![Fiche projet](docs/images/projet-taches.jpg)

Les paramètres du projet couvrent l'équipe, le budget en heures vendues, le
planning et la suppression.

Les **jalons** découpent le projet en étapes datées — cadrage, maquettes
validées, mise en ligne. Chacun rassemble les livrables qui le tiennent et
montre combien sont validés ; on le coche quand il est atteint, il passe en
retard quand son échéance est dépassée.

![Jalons d'un projet](docs/images/jalons.jpg)

### Planning

Un calendrier du mois : les jalons, les échéances des projets et ses propres
tâches, sur ses projets ou sur toute l'agence. Chaque entrée mène au projet ou
à la tâche. Le planning lit ce que Piilot sait ; il ne se synchronise avec
aucun agenda extérieur.

![Planning](docs/images/planning.jpg)

### Modèles de projet

Un modèle pose les jalons, les tâches et les services d'un projet type, avec
des échéances comptées en jours depuis son début. On le choisit à la création
d'un projet, qui en reçoit une copie à adapter.

![Modèle de projet](docs/images/modele-projet.jpg)

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

Ou à la semaine : une grille projet par jour, où l'on remplit les cases d'un
Tab à l'autre. Une case qui regroupe plusieurs saisies mène à sa journée pour
les reprendre une à une.

![Feuille de la semaine](docs/images/temps-semaine.jpg)

### Livrables

L'agence dépose un livrable (maquette, prototype, recette), qui est ensuite
validé ou renvoyé avec des retours. Chaque nouvelle soumission crée une
version, et l'historique est conservé. Le client valide ou renvoie ses
retours lui-même, depuis son portail ou directement depuis l'e-mail de dépôt ;
l'agence peut aussi enregistrer une réponse reçue ailleurs.

Les livrables se déposent depuis l'écran global ou depuis l'onglet
**Livrables** de la fiche du projet, où le projet est déjà choisi et le
prochain jalon à atteindre proposé.

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

Son **journal** raconte la relation : les notes, appels, rendez-vous et
e-mails qu'on y saisit, et les événements qui s'y inscrivent seuls — projet
créé, livrable validé, ticket ouvert. L'écran *Interactions* montre le même
fil pour tous les clients.

![Journal d'un client](docs/images/journal-client.jpg)

### Portail client

L'espace des clients de l'agence, pensé d'abord pour le téléphone. Chaque
client y suit ses projets — avancement, prochaine étape, dernière activité —
et voit d'emblée ce qui attend sa réponse.

![Portail client](docs/images/portail-accueil.jpg)

La page d'un projet s'ouvre sur la **prochaine étape** — le prochain jalon,
ce qui attend le client, où en est l'équipe — puis montre ses étapes, ses
livrables, ses **interlocuteurs** (le chargé de compte et l'équipe du projet,
avec e-mail, téléphone et lien de rendez-vous) et les documents que l'agence a
choisi de partager : un fichier de projet est interne tant qu'on ne le partage
pas d'un clic. Sur un livrable, le client ouvre la version, la valide ou
demande des retours en disant ce qui doit changer ; l'équipe du projet est
notifiée aussitôt, et le client reçoit un e-mail à chaque nouvelle version.

Cet e-mail porte deux boutons, **Valider** et **Faire un retour**, qui
répondent sans se connecter : des liens signés, personnels, valables trente
jours et pour la seule version déposée. Ouvrir le lien ne décide rien — un
filtre anti-spam qui suit les liens d'un e-mail ne valide pas un livrable —,
c'est le geste sur la page qui enregistre la réponse, au nom du compte du
destinataire et par le même chemin que depuis le portail.

<p>
  <img src="docs/images/portail-projet-mobile.jpg" alt="Projet dans le portail, sur mobile" width="300">
  <img src="docs/images/portail-livrable-mobile.jpg" alt="Validation d'un livrable, sur mobile" width="300">
</p>

Le **support** passe aussi par le portail. Le client dépose une demande —
un problème, une évolution, une question — sur l'un de ses projets, avec ses
captures d'écran, puis suit la conversation avec l'agence comme dans une
messagerie. Chaque réponse part dans les deux sens par e-mail : l'agence est
prévenue d'une nouvelle demande et des réponses du client, le client des
réponses de l'agence et des changements de statut. Les notes internes de
l'équipe ne quittent jamais le back-office, ni dans le portail ni dans un
e-mail ; les tickets que l'agence ouvre pour elle-même restent internes.

<p>
  <img src="docs/images/support-demande-mobile.jpg" alt="Nouvelle demande, sur mobile" width="300">
  <img src="docs/images/support-fil-mobile.jpg" alt="Conversation sur une demande, sur mobile" width="300">
</p>

Le portail a son API, `/api/v1/client/*`, réservée au rôle `client` et isolée
par le client de l'appelant dans chaque requête SQL : un identifiant qui
appartient à un autre client répond « introuvable », comme un identifiant qui
n'existe pas. Des tests d'isolation tournent à chaque intégration continue.

### Et aussi

- **Notifications en temps réel**, poussées par un flux SSE : tâche créée,
  assignée, commentée, changement de statut ou d'échéance, nouveau projet ;
  ticket confié, réponse ou changement de statut ; retours ou validation d'un
  livrable.
- **Services** : le référentiel des prestations vendues (design, développement,
  SEO…), avec une couleur par service, réutilisé partout.
- **Apps de la barre latérale** : les outils de l'agence en un clic. Leur logo
  est récupéré automatiquement par une tâche de fond.
- **Compte** : profil, photo, mot de passe, et pour l'équipe un lien de
  rendez-vous (Cal.com, Calendly, Google Agenda…) que les clients voient sur
  le portail.
- **Rôles et permissions en base** : trois rôles système (`admin`, `team`,
  `client`) et 21 permissions. Un droit retiré prend effet immédiatement, sans
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
| Rapports de temps | ✅ En place |
| Budgets et alertes de dépassement | ✅ En place |
| CRM : journal des interactions | ✅ En place |
| Comptes, invitations, rôles, mot de passe oublié | ✅ En place |
| Espace équipe : « Mon travail », feuille de la semaine, menu selon les droits | ✅ En place |
| Modèles de projet | ✅ En place |
| Jalons et planning | ✅ En place |
| Planning | ↗️ Renvoie vers l'agenda partagé de l'agence |
| **Portail client** : suivi des projets, validation des livrables, documents partagés | ✅ En place |
| **Portail client** : support et tickets | ✅ En place |

Hors périmètre, et donc absents par choix : facturation et comptabilité,
monitoring SEO, CMS et blog, RH, multi-agence, application mobile.

## Roadmap

Étapes jusqu'à la V1, release par release. Ce tableau est recalculé depuis le
fichier `VERSION` à chaque release : il indique toujours la version en cours.
Historique détaillé dans [CHANGELOG.md](CHANGELOG.md).

<!-- roadmap:readme -->
**Version actuelle : `1.1.0`** — détail de chaque release dans [ROADMAP.md](ROADMAP.md).

| Statut | Version | Nom | Espace | Objectif |
|---|---|---|---|---|
| ✅ Livrée | 0.1.0 | Socle | Tous | Connexion, sessions, rôles et permissions |
| ✅ Livrée | 0.2.0 | Gestion de projet | Admin | Projets, tâches, notifications, compte |
| ✅ Livrée | 0.3.0 | Back-office PM et CRM | Admin | Tickets, livrables, temps, CRM, image unique |
| ✅ Livrée | 0.4.0 | Temps et budgets | Admin | Rapports, budgets consommés, tableau de bord réel |
| ✅ Livrée | 0.5.0 | Gestion des comptes | Admin | Création, invitations, rôles, mot de passe oublié |
| ✅ Livrée | 0.6.0 | Espace team | Team | « Mon travail », navigation par droits, notifications |
| ✅ Livrée | 0.7.0 | Projets et CRM avancés | Admin | Jalons, planning, modèles de projet, interactions |
| ✅ Livrée | 0.8.0 | Portail : suivi | Client | Projets, jalons, validation des livrables |
| ✅ Livrée | 0.9.0 | Portail : tickets | Client | Dépôt et suivi des tickets |
| ✅ Livrée | 1.0.0 | V1 | Tous | Le temps de l'équipe : palette, chrono, Git, SIRET, adresse |
| 📍 Actuelle | **1.1.0** | Portail : livrables et prochaine étape | Client | Livrables sur la fiche projet, validation depuis l'e-mail, prochaine étape, interlocuteurs ; sauvegardes |
| ⏳ À venir | 1.2.0 | E-mail ↔ tickets | Client | Répondre et créer un ticket par e-mail (IMAP et webhook) ; journal d'audit |
| ⏳ À venir | 1.3.0 | Import et RGPD | Admin | Import CSV clients, contacts, projets, temps ; export et suppression RGPD |
| ⏳ À venir | 1.4.0 | Rapports | Admin | Temps par client et projet en CSV, activité de l'équipe, compte rendu PDF pour le client |
| ⏳ À venir | 1.5.0 | Portail : documents et santé du site | Client | Documents classés avec accusé, historique des mises en ligne, disponibilité et certificat |
| ⏳ À venir | 1.6.0 | Fin de l'ancienne plateforme | Tous | Actions en masse, tests de bout en bout, recette, guide utilisateur |
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
                 ┌─ app ─────────┐     ┌──────────────────────────── server ─┐
  navigateur ──▶ │  passerelle   │ ──▶ │  binaire Go (Fiber v3)              │
     HTTPS       │  (ne s'arrête │     │   ├── /api/v1/*     API JSON        │──▶ PostgreSQL 17
  (Traefik ou    │   jamais)     │     │   ├── /health/*     sondes          │
   reverse       └───────────────┘     │   ├── /openapi.json contrat         │──▶ Redis 8
   proxy)                              │   └── /*            front React     │
                                       └─────────────────────────────────────┘
```

**La passerelle** (`app`) est le seul service exposé. Elle transmet chaque
requête à une instance saine du serveur, et c'est elle qui rend les mises à
jour invisibles : voir [Mise à jour sans coupure](#mise-à-jour-sans-coupure).

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

C'est la cible de déploiement. Coolify lit le `docker-compose.yml` du dépôt,
tire l'image publiée, génère les secrets et les expose dans son interface. **Aucune valeur n'est à
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

Coolify lit le fichier et crée cinq services : `app` (la passerelle),
`server` (l'application), `updater`, `postgres` et `redis`.

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

*Deploy*. Coolify tire l'image `ghcr.io/plugiit/piilot-app:latest`, déjà
construite et testée par la CI, puis démarre. Les migrations s'appliquent au
démarrage de l'API.

**5. Créer le premier compte**

Voir [Premier compte](#premier-compte).

**Mises à jour** : depuis l'interface, par le bouton que voient les
administrateurs (voir [Mise à jour depuis l'interface](#mise-à-jour-depuis-linterface)).
*Redeploy* dans Coolify fonctionne aussi : il tire la dernière image.

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
docker compose -f docker-compose.yml -f docker-compose.selfhost.yml up -d
```

L'image est tirée depuis `ghcr.io` : rien n'est compilé sur le serveur.
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

**Mettre à jour** : depuis l'interface (voir
[Mise à jour depuis l'interface](#mise-à-jour-depuis-linterface)), ou à la main :

```bash
docker compose -f docker-compose.yml -f docker-compose.selfhost.yml pull
docker compose -f docker-compose.yml -f docker-compose.selfhost.yml up -d
```

Pour construire depuis les sources plutôt que tirer l'image publiée, ajouter
`-f docker-compose.build.yml` et `--build`. La mise à jour depuis l'interface
n'est alors pas disponible : il n'y a pas d'image plus récente à tirer.

### Variante : l'image seule, avec Postgres et Redis existants

L'image n'a besoin que de variables d'environnement et d'un volume. Chaque
release est publiée sur `ghcr.io/plugiit/piilot-app`, pour amd64 et arm64
(serveurs ARM, Mac Apple Silicon), avec trois tags : la version exacte
(`0.4.1`), la mineure (`0.4`, qui suit les correctifs) et `latest`. En
production, épingler une version plutôt que `latest`.

```bash
docker run -d --name piilot --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v piilot-files:/app/data/files \
  -e PUBLIC_BASE_URL=https://piilot.example.fr \
  -e ADMIN_ORIGINS=https://piilot.example.fr \
  -e DATABASE_URL='postgres://piilot:MOT_DE_PASSE@db.interne:5432/piilot?sslmode=require' \
  -e REDIS_URL='redis://:MOT_DE_PASSE@redis.interne:6379/0' \
  -e JWT_SECRET="$(openssl rand -hex 32)" \
  ghcr.io/plugiit/piilot-app:0.4
```

Pour construire l'image soi-même plutôt que la récupérer :
`docker build -t piilot-app .`

Sans Docker Compose, la mise à jour depuis l'interface n'est pas disponible :
l'updater retrouve l'application par les étiquettes que Compose pose sur ses
conteneurs.

> Générer `JWT_SECRET` une fois et le conserver : le changer déconnecte tout
> le monde.

## Premier compte

Une installation neuve n'a aucun compte. Le premier se crée en ligne de
commande ; tous les suivants s'invitent ensuite depuis Piilot (voir
[Comptes et invitations](#comptes-et-invitations)).

La commande `create-admin`, livrée
dans l'image, crée un administrateur en posant les questions. Le mot de
passe est masqué pendant la saisie et demandé deux fois.

**Sous Coolify** : ouvrir la ressource, onglet *Terminal*, choisir le
conteneur `server`, puis taper :

```sh
create-admin
```

```
Création d'un compte admin

Adresse e-mail : admin@example.fr
Prénom : Maxence
Nom : Mahieux
Mot de passe (12 caractères minimum) :
Confirmation :

Compte créé
  id    558da275-9cc5-4cc7-995f-f152e7ec61bf
  email admin@example.fr
  rôle  admin
```

**Hors Coolify** :

```bash
docker compose exec server create-admin
```

**En développement**, contre la base locale : `make create-admin`.

Chaque réponse invalide est redemandée : adresse mal formée ou déjà utilisée,
mot de passe trop court (moins de 12 caractères) ou trop long (plus de
72 octets), confirmation différente. La commande n'écrase jamais un compte
existant.

### Sans question, pour un script

La même commande, sous le nom `seed`, accepte tout en paramètres et peut créer
n'importe quel rôle :

```sh
SEED_PASSWORD='un-mot-de-passe-solide' seed \
    -email=lea@example.fr -firstname=Léa -lastname=Bernard -role=team
```

Le mot de passe passe par `SEED_PASSWORD`, jamais par un paramètre, pour ne
pas apparaître dans `ps` ni dans l'historique du shell. Sans lui, un mot de
passe aléatoire est généré et affiché **une seule fois**. Rôles possibles :
`admin`, `team`, `client`.

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
| `COOKIE_DOMAIN` | *(vide)* | Vide : cookie lié au seul domaine de l'app, le réglage le plus sûr. Avec un domaine par espace, se déduit du parent commun |
| `AUTH_URL` / `TEAM_URL` / `ADMIN_URL` / `CLIENT_URL` | *(vides)* | Un domaine par espace, voir « Un domaine par espace ». Tous les quatre ou aucun |
| `POSTGRES_DB` | `piilot` | Nom de la base |
| `RUN_MIGRATIONS` | `true` | Applique les migrations au démarrage |
| `MAX_UPLOAD_MIB` | `25` | Taille maximale d'une pièce jointe |
| `ACCESS_TOKEN_TTL` | `15m` | Durée du jeton d'accès |
| `REFRESH_TOKEN_TTL` | `720h` | Durée d'une session sans reconnexion (30 jours) |
| `READ_TIMEOUT` / `WRITE_TIMEOUT` | `30s` | Timeouts HTTP |
| `SHUTDOWN_TIMEOUT` | `15s` | Délai laissé aux requêtes en cours à l'arrêt |
| `DRAIN_DELAY` | `5s` | À l'arrêt, temps pendant lequel l'instance se déclare en arrêt et sert encore, que la passerelle l'écarte sans faire échouer de requête |
| `DELIVERABLE_REMINDER_AFTER` | `72h` | Un livrable sans réponse du client est relancé par e-mail, une fois, au-delà de ce délai (SMTP requis). `0` pour ne jamais relancer |
| `GIT_WEBHOOK_SECRET` | *(vide)* | Secret des webhooks GitHub/GitLab. Vide : Piilot en tire un, visible dans *Paramètres > Dépôts Git*. Renseigné : c'est lui, et l'écran ne permet plus de le changer |
| `BACKUP_STALE_AFTER` | `36h` | Sans sauvegarde réussie depuis ce délai, les admins sont prévenus dans la cloche. `0` pour une instance sauvegardée autrement |
| `BACKUP_AT` | `03:00` | Heure (Europe/Paris) de la sauvegarde quotidienne du service `backup` |
| `BACKUP_KEEP_DAILY` / `BACKUP_KEEP_WEEKLY` / `BACKUP_KEEP_MONTHLY` | `7` / `4` / `3` | Rétention des sauvegardes, sur le disque comme sur S3 |
| `BACKUP_S3_ENDPOINT` / `BACKUP_S3_BUCKET` / `BACKUP_S3_ACCESS_KEY` / `BACKUP_S3_SECRET_KEY` | *(vides)* | Copie des sauvegardes vers un stockage S3 compatible. Les quatre vont ensemble |
| `BACKUP_S3_REGION` / `BACKUP_S3_PREFIX` | `fr-par` / `piilot/` | Région de signature et préfixe des clés dans le seau |
| `PIILOT_TAG` | `latest` | Tag de l'image : `latest` suit toutes les versions, `0.4` les seuls correctifs de la 0.4, `0.4.1` fige la version |
| `UPDATE_CHECK` | `true` | Vérifie les nouvelles versions sur GitHub |
| `UPDATE_CHECK_INTERVAL` | `15m` | Intervalle entre deux vérifications, 5 minutes au minimum |
| `SMTP_HOST` | *(vide)* | Serveur SMTP des e-mails (invitations, mot de passe oublié). Vide : aucun e-mail, les liens se copient depuis l'écran des comptes |
| `SMTP_PORT` | `587` | Port du serveur SMTP |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | *(vide)* | Identifiants SMTP |
| `SMTP_FROM` | *(vide)* | Expéditeur, `Piilot <piilot@votre-agence.fr>`. Obligatoire avec `SMTP_HOST` |
| `SMTP_SECURITY` | `starttls` | `starttls` (port 587), `tls` (port 465) ou `none` (serveur local de test) |

### Un domaine par espace

Par défaut, Piilot tient sur un seul domaine : la connexion sur `/login`, le
back-office à la racine, le portail sur `/client`. Il peut aussi donner à
chaque espace son domaine :

| Variable | Exemple | Espace |
|---|---|---|
| `AUTH_URL` | `https://auth.agence.fr` | Connexion, invitation, mot de passe oublié |
| `TEAM_URL` | `https://team.agence.fr` | Back-office de l'équipe : Mon travail, projets, tâches, CRM, temps |
| `ADMIN_URL` | `https://admin.agence.fr` | Back-office des administrateurs : tout ce que voit l'équipe, plus le tableau de bord de l'agence et les paramètres |
| `CLIENT_URL` | `https://client.agence.fr` | Portail client |

Les quatre se renseignent ensemble et partagent un domaine parent (ici
`agence.fr`) : le cookie de session y est posé, et une seule connexion vaut
sur tous les espaces. C'est toujours la même application : chaque rôle a son
domaine — un administrateur fait tout son travail sur celui de
l'administration, sans jamais en changer ; un membre de l'équipe, sur le sien.
Une adresse ouverte sur le mauvais domaine est renvoyée vers le bon, et les
e-mails pointent vers le domaine de leur destinataire.

Avec Coolify, renseignez les quatre variables, puis listez les quatre domaines
sur le service « app » (Configuration > General > Domains), séparés par des
virgules et suivis du port interne :
`https://auth.agence.fr:8080,https://team.agence.fr:8080,https://admin.agence.fr:8080,https://client.agence.fr:8080`.
Chaque domaine doit pointer vers le serveur dans votre DNS — un enregistrement
`*.agence.fr` suffit pour les quatre.

En multi-domaines, les référentiels (services, modèles de projet) relèvent de
l'administration : seuls les administrateurs les modifient.

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
| `piilot-backups` | Archives du service `backup` | À copier hors du serveur (S3 ou autre), voir ci-dessous |

### Sauvegardes

Le service **`backup`** du `docker-compose.yml` sauvegarde chaque nuit la
base et les pièces jointes dans **une seule archive**, car les deux vont
ensemble : une base restaurée sans ses fichiers pointe vers des fichiers
absents. Chaque passage est noté en base : *Paramètres > Sauvegardes* montre
la dernière réussie et le journal, et les administrateurs sont prévenus dans
la cloche quand aucune sauvegarde n'a réussi depuis `BACKUP_STALE_AFTER`
(36 h). Au premier démarrage, la première sauvegarde part tout de suite.

- `BACKUP_AT` (`03:00`, heure de Paris) : l'heure quotidienne.
- Rétention `BACKUP_KEEP_DAILY` / `WEEKLY` / `MONTHLY` (7 / 4 / 3) : toutes
  celles des sept derniers jours, puis une par semaine, puis une par mois.
- Les archives sont dans le volume `piilot-backups`
  (`/app/data/backups/piilot-AAAAMMJJ-HHMMSS.tar`, qui contient `db.dump` au
  format custom de `pg_dump` et `files.tar.gz`).

**Les mettre à l'abri hors du serveur** : donner un seau S3 compatible,
hébergé dans l'UE (Scaleway, OVH, Hetzner, Garage, MinIO…), et la même
rétention s'y applique.

```bash
BACKUP_S3_ENDPOINT=https://s3.fr-par.scw.cloud
BACKUP_S3_BUCKET=sauvegardes-agence
BACKUP_S3_REGION=fr-par
BACKUP_S3_ACCESS_KEY=…
BACKUP_S3_SECRET_KEY=…
BACKUP_S3_PREFIX=piilot/      # facultatif, pour partager le seau
```

Sans S3, copier le volume ailleurs par un autre moyen (rsync, restic…) : des
sauvegardes sur le disque du serveur ne protègent pas de la perte du serveur.

**Restaurer** — la base cible doit exister (c'est le cas de la base de la
pile) ; son contenu est remplacé, les fichiers sont ajoutés à ceux présents :

```bash
# La plus récente du volume
docker compose exec backup backup restore

# Une archive précise, ou une archive rapatriée depuis S3
docker compose cp ./piilot-20261007-030000.tar backup:/tmp/
docker compose exec backup backup restore /tmp/piilot-20261007-030000.tar

# Puis redémarrer l'application, qui rejoue ses migrations si l'archive est
# plus ancienne que la version installée
docker compose restart server
```

`docker compose exec backup backup now` fait une sauvegarde sur-le-champ,
`backup list` liste les archives du volume. **La restauration est testée à
chaque intégration continue** : une base migrée et remplie est sauvegardée par
cette commande, restaurée dans une base vide, et l'API démarre dessus.

Le client Postgres embarqué dans l'image (`pg_dump`, `pg_restore`) est en
version 17, comme le service `postgres` du compose : un `pg_dump` ne lit pas un
serveur plus récent que lui.

### Comptes et invitations

*Paramètres → Comptes et rôles* rassemble trois onglets :

- **Comptes** : qui a accès à Piilot, avec quel rôle, et sa dernière
  connexion. Un administrateur peut changer le rôle d'un compte, le
  désactiver (il est déconnecté sur-le-champ et ne peut plus se connecter)
  ou le réactiver, et créer un lien de réinitialisation de mot de passe.
- **Invitations** : les invitations en attente ou expirées, à renvoyer ou à
  annuler.
- **Rôles** : ce que chaque rôle permet. Un droit retiré s'applique à la
  requête suivante, sans attendre que la personne se reconnecte. Le rôle
  `admin` garde toutes les permissions ; gérer les comptes, les rôles et les
  mises à jour reste réservé aux administrateurs.

**Inviter** : le bouton *Inviter* envoie un lien valable sept jours. La
personne choisit son nom et son mot de passe, puis arrive connectée dans son
espace. Pour un compte du portail, on désigne le client dont elle suivra les
projets.

**E-mails** : invitations et liens de réinitialisation partent par le serveur
SMTP configuré (voir [Configuration](#configuration)), via une file d'attente
en base, avec de nouvelles tentatives si le serveur ne répond pas. Sans SMTP,
rien ne bloque : les liens s'affichent à l'écran, à transmettre soi-même, et
la page « mot de passe oublié » oriente vers un administrateur.

**Garde-fous** : un administrateur ne peut ni se désactiver ni changer son
propre rôle, et il reste toujours au moins un administrateur actif.

### Mise à jour depuis l'interface

Quand une nouvelle version est publiée, les administrateurs voient une carte
**Nouvelle version** au bas du menu latéral. Son bouton **Installer la mise à
jour** ouvre les notes de version et, après
confirmation, met l'application à jour en une à deux minutes. Pour tous les
autres comptes, un bandeau propose ensuite de recharger la page.

C'est le service **`updater`** du `docker-compose.yml` qui s'en charge. Il ne
dépend d'aucun outil de déploiement : Coolify, Docker Compose seul, peu
importe.

1. L'admin confirme : l'application enregistre une demande en base.
2. L'updater la prend dans les dix secondes, tire la nouvelle image depuis
   `ghcr.io` et démarre un second conteneur `server` sur la nouvelle image, à
   l'identique (variables, volumes, réseaux, étiquettes), **à côté** de
   l'ancien.
3. Il attend que la nouvelle version réponde à sa sonde de santé. Si elle ne
   démarre pas, il la supprime : l'ancienne n'a jamais cessé de servir.
4. La passerelle bascule le trafic vers la nouvelle version, puis l'updater
   arrête l'ancienne.

#### Mise à jour sans coupure

La passerelle (`app`) se place devant l'application et ne s'arrête jamais.
Elle sonde chaque seconde les instances de `server` et n'envoie le trafic
qu'à une instance saine — la plus récente quand deux tournent ensemble.

- **Bascule.** La nouvelle version reçoit le trafic dès qu'elle est saine.
  L'ancienne, au signal d'arrêt, se déclare en arrêt et sert encore
  `DRAIN_DELAY` (5 s) : la passerelle l'écarte avant qu'elle ne ferme ses
  connexions, et ses requêtes en cours se terminent.
- **Relance.** Une requête qui n'a pas pu joindre une instance — connexion
  refusée, rien n'est parti — est renvoyée à une autre. Une requête déjà
  transmise ne se rejoue jamais : elle a pu être traitée.
- **Attente.** Sans instance saine, une requête attend jusqu'à 15 s
  (`HOLD_TIMEOUT` de la passerelle) qu'il y en ait une.
- **Maintenance.** Au-delà, la passerelle sert une page « Mise à jour en
  cours » qui se recharge d'elle-même, et un `503 MAINTENANCE` aux appels
  d'API ; un onglet déjà ouvert affiche le même écran. Ce cas ne se produit
  pas pendant une mise à jour : seulement si le serveur tombe, ou à un
  redéploiement de toute la pile.
- **Onglets ouverts.** Un onglet chargé avant la mise à jour qui demande un
  morceau du front disparu se recharge de lui-même sur la nouvelle version.

Rien ne dépend de Coolify ni de Traefik : la bascule a lieu dans la
passerelle, quel que soit le proxy devant elle.

**Règle pour les migrations.** Pendant la bascule, l'ancienne version tourne
quelques secondes sur le schéma de la nouvelle. Une migration ajoute
(table, colonne nullable ou avec défaut, index) mais ne supprime ni ne
renomme dans la même version : une colonne à retirer cesse d'être lue dans
une version, et disparaît dans la suivante.

**Ce qui coupe encore.** Le *Redeploy* de Coolify ou un `docker compose up`
qui recrée la pile (passerelle comprise), une mise à jour de Postgres ou de
Redis, un redémarrage du serveur. La mise à jour courante se fait depuis
l'interface.

**Installations antérieures à la passerelle.** Le passage à la passerelle
change la composition de la pile : il se fait une fois, par un *Redeploy*
(ou `git pull && docker compose up -d --remove-orphans`), avec une courte
coupure. Les mises à jour suivantes sont sans coupure.

**Sécurité.** L'updater est le seul conteneur qui reçoit le socket Docker,
c'est-à-dire un accès équivalent à root sur le serveur. L'application, exposée
sur Internet, ne l'a jamais. L'updater n'ouvre aucun port et ne reçoit d'ordre
que par la base, et il ne sait faire qu'une chose : mettre à jour le service
`server` de son projet vers l'image publiée. Pour se passer de la mise à jour
depuis l'interface, retirer le service `updater` : rien d'autre ne change.

À savoir :

- **Faire une sauvegarde avant.** La mise à jour applique les migrations de
  la nouvelle version, qui ne se défont pas. Le retour arrière remet l'ancienne
  version en route, pas l'ancien schéma de base.
- Le bouton n'apparaît que si l'updater donne signe de vie. Sinon, le
  dialogue dit ce qui l'en empêche (socket Docker non monté, conteneur
  introuvable…).
- `PIILOT_TAG` doit suivre les versions : `latest`, ou `0.4` pour les seuls
  correctifs. Avec une version figée (`0.4.1`), l'updater n'a rien à tirer.
- L'updater et la passerelle passent sur la nouvelle image au prochain
  `docker compose up` ou *Redeploy* : ils changent rarement, et ne peuvent
  pas se remplacer sans s'interrompre.
- Chaque instance vérifie elle-même les nouvelles versions, tous les quarts
  d'heure (`UPDATE_CHECK_INTERVAL`), sans rien à configurer côté GitHub. Les
  requêtes sont conditionnelles : quand rien n'a changé, GitHub répond 304,
  ce qui ne compte pas dans sa limite d'appels. Une nouvelle version s'annonce
  une fois dans la cloche des admins, et le menu du compte propose
  *Rechercher une mise à jour* pour vérifier sans attendre.
- La vérification des versions interroge l'API publique de GitHub. Pour une
  instance qui ne doit rien appeler au-dehors : `UPDATE_CHECK=false`.
- Seuls les comptes qui ont la permission `system.update` voient le bouton :
  le rôle `admin`, par défaut.

### Dépôts Git : pull requests et mises en ligne

Piilot se branche sur GitHub et GitLab par **webhook** : ils poussent leurs
événements, Piilot écrit, rien ne sort vers eux et aucun jeton d'API n'est
stocké. Un seul secret pour toute l'installation, quel que soit le nombre
d'organisations ou de dépôts.

**Réglage, une fois.** *Paramètres > Dépôts Git* affiche l'adresse du webhook
(`https://votre-piilot/api/v1/hooks/git`) et le secret. À copier :

- **GitHub** : *Settings > Webhooks* de l'organisation (un seul webhook pour
  tous ses dépôts) ou d'un dépôt. Content type `application/json`, le secret,
  et les événements *Pull requests*, *Releases*, *Pushes* et *Deployment
  statuses*.
- **GitLab** : *Settings > Webhooks* du groupe ou du projet. Le secret dans
  *Secret token*, et les déclencheurs *Merge request*, *Tag push*, *Releases*
  et *Deployment*.

**Par projet.** Dans *Paramètres > Général* du projet, l'adresse de son dépôt
(`https://github.com/org/depot` ou `git@gitlab.agence.fr:groupe/depot.git`).
C'est elle qui relie un événement reçu au bon projet.

**Rien à lier à la main.** Une pull request nomme ce qu'elle fait avancer par
son numéro, dans son titre, sa branche ou sa description : `#47` pour le
ticket 47, `T-123` (ou `task-123`) pour la tâche 123. Le numéro d'une tâche
s'affiche dans son panneau. Un numéro qui ne correspond à rien dans le projet
du dépôt est ignoré.

Ce que ça produit :

| Événement | Ticket nommé | Tâche nommée |
|---|---|---|
| Pull request ouverte | passe *En revue* | passe *En revue* |
| Pull request fusionnée | passe *Prêt à déployer* | passe *Terminée* |
| Mise en ligne (release publiée, tag poussé, déploiement réussi) | les tickets *Prêt à déployer* dont la PR est fusionnée passent *Terminé*, avec un mot visible du client et l'e-mail habituel | — |

Une mise en ligne atteint aussi le jalon du projet nommé « Mise en ligne »
(ou « Mise en production », « Déploiement », « Release », « Lancement »,
« Livraison ») s'il ne l'est pas encore, s'inscrit au journal du client et
s'affiche sur le portail. Les pull requests liées se lisent sur la fiche du
ticket et dans le panneau de la tâche.

### Sondes

| Route | Réponse | Usage |
|---|---|---|
| `/health/live` | 200 dès que le processus répond, 503 quand il s'arrête | `HEALTHCHECK` du conteneur et sonde de la passerelle. Ne touche aucune dépendance : redémarrer l'API ne réparerait pas une base injoignable |
| `/health/gateway` | Toujours 200 ; `ready` dit si une instance peut servir | Sonde de la passerelle elle-même, et de la page de maintenance |
| `/health/ready` | 200 seulement si Postgres **et** Redis répondent | Supervision (Uptime Kuma…) |

### Logs

JSON structuré sur la sortie standard, lisibles dans Coolify ou avec
`docker compose logs -f server` (la passerelle : `app`).

## Développement

Prérequis : Go 1.26, Node 24, Docker.

```bash
make up        # Postgres, Redis et Mailpit en conteneurs (docker-compose.dev.yml)
make dev-api   # API sur :8080, rechargement à chaud
make dev-web   # front sur :5173, proxy /api vers l'API
```

Au premier lancement, copier `api/.env.example` en `api/.env` et y mettre un
`JWT_SECRET` d'au moins 32 caractères. Mailpit intercepte les e-mails sans rien envoyer :
ils se lisent sur http://localhost:8025 (dans `api/.env` : `SMTP_HOST=localhost`,
`SMTP_PORT=1025`, `SMTP_SECURITY=none`, `SMTP_FROM=Piilot <piilot@example.fr>`). En développement, c'est Vite qui sert
le front (`STATIC_DIR` reste vide).

| Commande | Effet |
|---|---|
| `make create-admin` | Crée un compte admin dans la base locale, en interactif |
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
├── cmd/seed/               création de comptes (create-admin, seed)
├── cmd/updater/            mise à jour de l'application (service updater)
├── cmd/gateway/            passerelle devant l'application (service app)
├── cmd/backup/             sauvegarde et restauration (service backup)
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
docker-compose.yml          déploiement : passerelle, serveur, updater, backup, Postgres, Redis
docker-compose.build.yml    construction de l'image depuis les sources
docker-compose.selfhost.yml publication du port hors Coolify
docker-compose.dev.yml      Postgres + Redis pour le développement
.env.example                variables pour une installation hors Coolify
VERSION                     version courante, tenue par le script de release
CHANGELOG.md                historique des versions
ROADMAP.md                  étapes jusqu'à la V1
```

## Contribuer

Les contributions sont bienvenues : voir [CONTRIBUTING.md](CONTRIBUTING.md)
pour les conventions et la marche à suivre. Une faille de sécurité ne se
signale pas en issue publique : voir [SECURITY.md](SECURITY.md).

## Licence

Piilot est distribué sous licence [GNU Affero General Public License v3](LICENSE).

Vous pouvez l'utiliser, le modifier et le redistribuer librement. Si vous
faites tourner une **version modifiée** accessible à d'autres personnes par le
réseau, vous devez leur donner accès au code source de cette version, sous la
même licence.
