# Changelog

Toutes les versions notables de Piilot. Format inspiré de
[Keep a Changelog](https://keepachangelog.com/fr/1.1.0/), numérotation
[SemVer](https://semver.org/lang/fr/). La feuille de route est dans
[ROADMAP.md](ROADMAP.md).

Une section se rédige à la main avant la release, ou se génère depuis les
commits : voir « Publier une version » dans le README.

<!-- releases -->

## [0.3.0] — Back-office PM et CRM

Première version publiée. Elle regroupe tout ce qui a été construit depuis le
début du projet. Rien n'est encore déployé.

### Nouveautés

- **Tickets** : anomalies, évolutions et assistance rattachées à un projet ;
  cycle de vie en sept statuts, discussion avec messages internes, historique ;
  vues tableau, par projet et par statut.
- **Livrables** : dépôt, versions successives, décision « validé » ou
  « retours ».
- **Saisie du temps** : pointage à la journée par projet, tâche et service.
- **Services** : référentiel des prestations, rattachées aux projets et aux
  tâches.
- **CRM** : clients, contacts, pipeline en kanban (piste, devis, actif,
  veille, perdu), fiche client avec projets et comptes de portail.
- **Apps de la barre latérale**, avec récupération automatique des logos par
  une tâche de fond.

### Technique

- Monorepo `piilot-app` : `api/` (Go) et `web/` (React), historique des deux
  dépôts d'origine conservé.
- Une seule image Docker : le binaire Go sert l'API et le front sur la même
  origine.
- Déploiement Coolify en Docker Compose (app, Postgres, Redis), variables
  éditables dans l'interface ; installation hors Coolify documentée.
- CI unique et workflow de release déclenché par tag.

## [0.2.0] — Gestion de projet (jalon non taggé)

- Projets : liste filtrable, favoris, fiche, équipe, paramètres.
- Tâches : kanban et liste, panneau de détail, sous-tâches, commentaires,
  pièces jointes, affectations.
- Tableau de bord sur les compteurs réels (projets, clients, heures vendues,
  avancement par service).
- Notifications en temps réel (flux SSE) sur les événements des tâches.
- Compte : profil, photo, mot de passe.

## [0.1.0] — Socle (jalon non taggé)

- Connexion, déconnexion, rafraîchissement de session à rotation avec
  détection de rejeu.
- Rôles et permissions en base (`admin`, `team`, `client`, 17 permissions).
- Limitation de débit sur la connexion, contrat OpenAPI, sondes de santé.
- Front : routage par espace (`/client/*` et back-office), interface shadcn/ui.
