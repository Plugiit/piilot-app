# Changelog

Toutes les versions notables de Piilot. Format inspiré de
[Keep a Changelog](https://keepachangelog.com/fr/1.1.0/), numérotation
[SemVer](https://semver.org/lang/fr/). La feuille de route est dans
[ROADMAP.md](ROADMAP.md).

Chaque section est rédigée au moment de la release, à partir d'un brouillon
de Claude Code relu avant publication, puis reprise telle quelle sur la page
de release GitHub. Voir « Publier une version » dans le README.

<!-- releases -->

## [0.3.1] — Temps facturable · 2026-09-19

Cette version distingue le temps facturable du temps interne, et le tableau de bord affiche désormais la répartition réelle des heures. Elle corrige aussi le logo de la page de connexion et simplifie la création du premier compte.

### Nouveautés

#### Admin

- **Projet interne** : une case dans les paramètres d'un projet le marque comme interne (site de l'agence, outils, formation). Le temps saisi dessus est non facturable, y compris celui déjà saisi : changer le statut d'un projet reclasse tout son temps.
- **Temps facturable** : le bloc du tableau de bord affiche les heures facturables (projets clients), non facturables (projets internes) et restantes sur les heures vendues. Il se met à jour après chaque saisie de temps et chaque changement de statut d'un projet. Le montant en euros, jusqu'ici fictif, est retiré : Piilot ne connaît pas encore de taux horaire.

### Corrections

- La page de connexion affichait un logo générique au-dessus du formulaire, et l'onglet du navigateur un « P » qui n'était pas celui de l'agence. Les deux reprennent le logo Plugiit.

### Technique

- Commande `create-admin` dans l'image : crée un compte administrateur en posant les questions, mot de passe masqué et confirmé. La commande `seed` reste disponible pour les scripts et les autres rôles.
- Roadmap réorganisée par fonctionnalités, avec le détail des versions déjà livrées.

### À savoir pour le déploiement

- La migration `000027_project_internal` s'applique au démarrage : elle ajoute l'indicateur « projet interne », désactivé pour tous les projets existants. Tout le temps déjà saisi reste donc facturable jusqu'à ce qu'un projet soit coché.
- Premier compte : `create-admin` depuis le terminal du conteneur `app` (voir le README).

## [0.3.0] — Back-office PM et CRM · 2026-09-19

Première version publiée de Piilot. Elle regroupe les jalons internes 0.1 (socle) et 0.2 (gestion de projet), et livre le back-office de l'agence : gestion de projet (PM) et relation client (CRM). L'espace Admin couvre les projets, les tâches, les tickets, les livrables et la saisie du temps, ainsi que le suivi des clients et des contacts. Tous les comptes disposent de la connexion sécurisée et de leur page de compte.

### Points forts

- Projets et tâches en un seul endroit : liste filtrable, fiche projet, kanban avec glisser-déposer et panneau de détail de la tâche.
- CRM avec un pipeline commercial en kanban, les contacts et le contact principal de chaque client.
- Tickets d'anomalie, d'évolution et d'assistance, avec un numéro unique, un fil de messages et un historique des changements.
- Livrables versionnés, avec la décision (validé ou retours) et son commentaire enregistrés à chaque version.
- Saisie du temps par projet, tâche et service, qui alimente les heures consommées des projets.

### Nouveautés

#### Admin

- **Tableau de bord** : nombre de projets, de clients, heures vendues et avancement des tâches par service, calculés sur les données réelles.
- **Liste des projets** : filtres, tri et mise en favori personnelle. Chaque projet porte une description, une priorité et une échéance.
- **Fiche projet** : onglets Vue d'ensemble et Tâches, équipe du projet, pièces jointes et tickets du projet. Trois liens de travail sont nommés : maquette Figma, production et préproduction.
- **Paramètres du projet** : écrans dédiés pour modifier les informations, les liens et l'équipe.
- **Tâches** : vue kanban avec glisser-déposer, vue en liste et écran regroupant toutes les tâches. Chaque tâche a une priorité et un ou plusieurs services.
- **Panneau de tâche** : sous-tâches, commentaires, pièces jointes et affectations, sans quitter le tableau. Les cartes du kanban affichent les compteurs de sous-tâches, de commentaires et de pièces jointes.
- **Notifications en temps réel** : vous êtes prévenu quand une tâche est créée, change de statut ou d'échéance, vous est affectée ou retirée, ou reçoit un commentaire. C'est aussi le cas à la création d'un projet. Les notifications se marquent comme lues une par une ou toutes ensemble.
- **Clients** : liste et fiche client avec site, téléphone, adresse, SIRET, numéro de TVA, chargé de compte et nombre de projets actifs.
- **Pipeline commercial** : les clients sont rangés en kanban par étape (lead, devis, actif, veille, perdu) et changent d'étape par glisser-déposer. L'ancienneté dans l'étape permet de repérer les dossiers qui stagnent.
- **Contacts** : liste des contacts avec fonction, e-mail et téléphone. Un contact peut exister avant d'être rattaché à un client. Chaque client désigne son contact principal.
- **Tickets** : création, statut du backlog jusqu'au déploiement, priorité et affectation. L'écran « Mes tickets » existe en liste et en tableau.
- **Fil d'un ticket** : messages et notes internes, qui ne sont jamais destinées au client. Un journal date chaque changement de statut, de priorité, de type, d'affectation ou de sujet.
- **Livrables** : suivi par projet avec des versions successives, sous forme de fichier ou de lien. Chaque version reçoit une décision, validée ou à reprendre, avec son commentaire. La validation par le client lui-même arrivera avec le portail.
- **Saisie du temps** : pointage par jour, en minutes, sur un projet, avec la tâche, le service et une note facultatifs. Les heures consommées du projet se mettent à jour automatiquement.
- **Services** : référentiel des prestations de l'agence, avec une couleur. Un projet ou une tâche peut relever de plusieurs services.
- **Applications du rail** : les outils affichés dans la barre latérale se gèrent depuis les paramètres, sans redéploiement. Le logo est récupéré automatiquement depuis l'adresse de l'outil, ou déposé à la main.
- **Navigation** : barre latérale organisée par module, fil d'Ariane et affichage adapté au mobile avec un menu en tiroir.

#### Tous les espaces

- **Connexion** : connexion, déconnexion et session prolongée automatiquement.
- **Compte** : modification du profil (nom, sexe, téléphone, adresse postale), de la photo et du mot de passe.

### Technique

- Authentification par jetons en cookie httpOnly avec rotation. La réutilisation d'un jeton déjà utilisé révoque toutes les sessions qui en dérivent.
- Mots de passe hachés avec bcrypt (coût 12). Les jetons de session sont stockés hachés.
- Rôles et permissions en base, relus à chaque requête : le retrait d'un droit prend effet immédiatement.
- Limitation des tentatives de connexion à 30 par IP et 5 par compte sur 15 minutes. Elle s'appuie sur Redis ; si Redis ne répond pas, les connexions sont acceptées et l'incident est journalisé.
- Purge quotidienne des jetons de session expirés depuis plus de 7 jours.
- Spécification OpenAPI publiée sur `/openapi.json`. Un test vérifie qu'elle correspond aux routes montées.
- Sondes `/health/live` et `/health/ready`.
- Image Docker unique : l'API sert aussi l'interface, sur la même origine.
- Pièces jointes, avatars et logos stockés sur disque local.
- Intégration continue GitHub Actions (lint, types, tests, tests d'intégration contre Postgres et Redis) et workflow de release.

### À savoir pour le déploiement

- Les migrations `000001` à `000026` s'appliquent au démarrage (`RUN_MIGRATIONS=true` par défaut) :
  - `000001_init` à `000003` : comptes, sessions, rôles et permissions. Le rôle `admin` reçoit 17 permissions, `team` 15 et `client` 5.
  - `000004_pm` à `000008` : clients, projets, tâches, pièces jointes, favoris et liens de projet.
  - `000009` à `000010` : champs du profil utilisateur et notifications.
  - `000011` à `000015` : comptes du portail rattachés à un client, contacts, pipeline commercial et date d'entrée dans l'étape.
  - `000016` à `000019` : tickets, avec leurs messages et leur journal.
  - `000020` à `000023` : livrables, services et rattachement de plusieurs services aux projets et aux tâches.
  - `000024` à `000025` : applications du rail et récupération automatique des favicons.
  - `000026` : saisie du temps.
- Les migrations reprennent aussi des données existantes :
  - les interlocuteurs déjà saisis sur les clients deviennent des contacts ;
  - les clients qui ont un projet passent à l'étape « actif » ;
  - quatre applications sont créées dans le rail : Google Drive, Coolify, Uptime Kuma et Proxmox.
- Prérequis Postgres :
  - Postgres 15 minimum ;
  - droits suffisants pour créer les extensions `citext`, `pg_trgm` et `uuid-ossp`.
- Variables obligatoires :
  - `DATABASE_URL` ;
  - `JWT_SECRET`, d'au moins 32 caractères.
- Variables facultatives, avec leur valeur par défaut :
  - `APP_ENV` (`production` dans l'image ; `development` ou `staging` sinon) ;
  - `REDIS_URL` ;
  - `PUBLIC_BASE_URL` ;
  - `COOKIE_DOMAIN` ;
  - `ADMIN_ORIGINS` ;
  - `ACCESS_TOKEN_TTL` (15 min) ;
  - `REFRESH_TOKEN_TTL` (30 jours) ;
  - `FILES_DIR` (`./data/files`) ;
  - `MAX_UPLOAD_MIB` (25) ;
  - `STATIC_DIR` ;
  - `RUN_MIGRATIONS`.
- Le répertoire `FILES_DIR` contient les fichiers déposés : il doit être conservé d'un déploiement à l'autre.
- Hors Coolify, copier `.env.example` en `.env`, remplacer les valeurs `A_GENERER`, puis utiliser `docker-compose.selfhost.yml`.
- Action manuelle : créer le premier compte administrateur avec la commande `seed`.

