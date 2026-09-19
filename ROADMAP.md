# Roadmap vers la V1

Piilot est l'outil de gestion de projet de l'agence. Il a **trois espaces**,
un par population :

| Espace | Rôle | Ce qu'on y fait |
|---|---|---|
| **Admin** | `admin` | Piloter l'agence : clients et pipeline, budgets et rentabilité, rapports de temps, comptes et droits, paramètres |
| **Team** | `team` | Produire : ses tâches, ses tickets, ses livrables, son temps, les projets sur lesquels on intervient |
| **Client** | `client` | Suivre ses projets, valider les livrables, déposer et suivre ses tickets |

La **V1** est atteinte quand chacun des trois espaces couvre son usage
quotidien, sans écran factice ni recours à l'ancienne plateforme.

## État actuel

**Version : 0.3.0**, publiée le 2026-09-19.

L'espace **Admin** porte le back-office complet : projets, tâches, tickets,
livrables, saisie du temps, CRM avec pipeline et contacts. Le socle
d'authentification et de rôles est en place. Les deux autres espaces sont des
coquilles vides : **Team** voit les mêmes écrans qu'Admin avec des droits
réduits, et **Client** a les routes de connexion mais aucun endpoint d'API.

---

## Roadmap : 0.4 → V1

Chaque version apporte des fonctionnalités visibles. Les chantiers purement
techniques (tests E2E, revue de sécurité, RGPD) et l'exploitation
(déploiement, sauvegardes) sortent en correctifs au fil de l'eau ou
deviennent des critères de sortie de la V1.

<!-- roadmap:table -->
| Statut | Version | Nom | Espace | Objectif |
|---|---|---|---|---|
| ✅ Livrée | 0.1.0 | Socle | Tous | Connexion, sessions, rôles et permissions |
| ✅ Livrée | 0.2.0 | Gestion de projet | Admin | Projets, tâches, notifications, compte |
| 📍 Actuelle | **0.3.0** | Back-office PM et CRM | Admin | Tickets, livrables, temps, CRM, image unique |
| ⏳ À venir | 0.4.0 | Temps et budgets | Admin | Rapports, budgets consommés, tableau de bord réel |
| ⏳ À venir | 0.5.0 | Gestion des comptes | Admin | Création, invitations, rôles, mot de passe oublié |
| ⏳ À venir | 0.6.0 | Espace team | Team | « Mon travail », navigation par droits, notifications |
| ⏳ À venir | 0.7.0 | Projets et CRM avancés | Admin | Jalons, planning, modèles de projet, interactions |
| ⏳ À venir | 0.8.0 | Portail : suivi | Client | Projets, jalons, validation des livrables |
| ⏳ À venir | 0.9.0 | Portail : tickets | Client | Dépôt et suivi des tickets |
| ⏳ À venir | 1.0.0 | V1 | Tous | Recette et critères de sortie |
<!-- /roadmap:table -->

---

## Détail des versions

### 0.1.0 — Socle ✅

> Se connecter, avec le bon rôle et les bons droits.

- **Connexion** : connexion, déconnexion, session prolongée automatiquement.
  Jetons en cookie httpOnly, rotation avec détection de rejeu.
- **Rôles et permissions en base** : `admin`, `team`, `client`,
  17 permissions relues à chaque requête. Un droit retiré s'applique tout de
  suite.
- **Protection** : limitation des tentatives de connexion (30 par IP, 5 par
  compte sur 15 minutes), mots de passe en bcrypt.
- **Deux espaces** : back-office et portail client, chacun avec sa garde de
  rôle et sa navigation.
- **Contrat d'API** OpenAPI, sondes de santé, migrations appliquées au
  démarrage.

### 0.2.0 — Gestion de projet ✅

> Suivre les projets et les tâches de l'agence.

- **Projets** : liste avec filtres, tri et favoris ; fiche projet (vue
  d'ensemble, équipe, pièces jointes, liens Figma, production et
  préproduction) ; écrans de paramètres.
- **Tâches** : kanban avec glisser-déposer, vue en liste, écran de toutes les
  tâches ; panneau de détail avec sous-tâches, commentaires, pièces jointes
  et affectations.
- **Tableau de bord** : nombre de projets et de clients, heures vendues,
  avancement des tâches par service.
- **Notifications en temps réel** sur les tâches et les projets.
- **Compte** : profil, photo, mot de passe.

### 0.3.0 — Back-office PM et CRM 📍

> Tout le travail de production et le suivi commercial dans le même outil.

**Tickets**

- Anomalies, évolutions et demandes d'assistance rattachées à un projet,
  avec un numéro unique.
- Cycle de vie du backlog au déploiement, priorité, affectation.
- Fil de discussion avec notes internes, journal daté de chaque changement.
- Vues : mes tickets en liste et en tableau, par projet, par statut.

**Livrables**

- Suivi par projet, versions successives sous forme de fichier ou de lien.
- Décision par version (validé ou à reprendre) avec son commentaire. La
  validation par le client lui-même arrive avec le portail (0.8).

**Temps**

- Saisie à la journée par projet, avec tâche, service et note facultatifs.
- Heures consommées du projet mises à jour à chaque saisie.

**CRM**

- Clients : liste et fiche (coordonnées, SIRET, TVA, chargé de compte,
  projets actifs, comptes du portail).
- Pipeline commercial en kanban : lead, devis, actif, veille, perdu, avec
  l'ancienneté dans chaque étape.
- Contacts, rattachés ou non à un client, avec un contact principal par
  client.

**Paramètres**

- Services : référentiel des prestations, rattachées aux projets et aux
  tâches.
- Applications de la barre latérale, avec récupération automatique des
  logos.

**Image unique**

- Un seul conteneur sert l'API et l'interface, sur la même origine.
- Déploiement Coolify (app, Postgres, Redis) configurable depuis
  l'interface ; installation hors Coolify documentée.
- Commande `create-admin` pour créer le premier compte.
- Publication des versions avec changelog rédigé en français.

**Reste incomplet dans cette version**, à solder dans les versions
suivantes :

- Tableau de bord : carte d'activité, charge de travail, activité de
  l'équipe et agenda encore factices → 0.4. Le temps facturable est réel
  (projets internes non facturables).
- Écrans réservés : rapports de temps (0.4), comptes (0.5), modèles de
  projet et interactions CRM (0.7).
- Le rôle `team` voit le même back-office qu'`admin` → 0.6.
- Le portail client n'a aucun écran fonctionnel → 0.8 et 0.9.

### 0.4.0 — Temps et budgets

> Piloter l'agence depuis Piilot. Remplace les calculs manuels et les
> feuilles de calcul par des données toujours à jour.

**Nouveautés**

- **Rapports de temps** : par projet, personne, service, période ; export CSV.
- **Budgets réels** : heures vendues, consommées (depuis la saisie du temps),
  restantes. Alerte de dépassement.
- **Tableau de bord** : compteurs réels (projets, clients, heures, services),
  à la place des widgets factices. Activité du jour, charge par personne.

**Technique**

- Les cumuls de temps et budgets sont **précalculés** à chaque saisie, jamais
  recalculés à l'affichage.

**Terminé quand** : un mois se clôture (temps, budgets) sans ouvrir une
feuille de calcul.

### 0.5.0 — Gestion des comptes

> Plus aucun compte ne se crée en shell. L'admin gère les comptes et les
> rôles depuis Piilot.

**Nouveautés**

- **Écran Comptes** : liste, création, changement de rôle, désactivation,
  réactivation. Désactiver révoque les sessions.
- **Invitations par e-mail** : lien à usage unique, qui expire. La personne
  invitée choisit son mot de passe.
- **Mot de passe oublié** : jeton à usage unique, courte durée, limitation de
  débit. Réponse identique que l'adresse existe ou non.
- **Écran Rôles** : consulter et modifier les permissions de chaque rôle.

**Technique**

- E-mails transactionnels : file d'attente en base, tâche de fond, nouvel
  essai en cas d'échec.
- Modèles en français, texte brut et HTML.

**Terminé quand** : un nouveau membre rejoint Piilot par une invitation,
sans aucune commande shell.

### 0.6.0 — Espace team

> Un chef de projet fait sa journée dans Piilot sans voir ce qui ne le
> concerne pas.

**Nouveautés**

- **Page « Mon travail »** : tâches en retard, assignations, tickets,
  livrables à déposer. Lien vers chaque projet où on intervient.
- **Feuille de temps hebdomadaire** : pointage à la semaine, en plus de la
  saisie par jour.
- **Navigation selon les permissions** : le front lit les droits retournés
  par `/auth/me` et n'affiche que les entrées accessibles. Pas de « Clients »,
  pas de « Pipeline commercial », pas de budgets, pas de rapports.
- **Notifications** : étendues aux tickets (assignation, réponse, changement
  de statut) et aux livrables (validation, retours).

**Technique**

- Nouvelles permissions pour ce que `team` ne doit plus voir (pipeline
  commercial, montants, budgets).

**Terminé quand** : un chef de projet passe une semaine complète dans son
espace sans onglet d'admin.

### 0.7.0 — Projets et CRM avancés

> Gérer les projets complets et le pipeline commercial sans passer par
> l'ancienne plateforme.

**Nouveautés**

- **Jalons** : étapes datées du projet, reliées aux livrables. Affichées dans
  le portail client.
- **Planning** : vue calendrier des jalons et des échéances, remplaçant le
  lien vers l'agenda partagé.
- **Modèles de projet** : créer un projet avec ses tâches, jalons et services
  pré-remplis.
- **Interactions CRM** : journal des notes et appels saisis à la main, et des
  événements automatiques (projet créé, livrable validé, ticket ouvert).

**Terminé quand** : tous les projets en cours ont leurs jalons et livrables,
et le pipeline commercial se gère en kanban.

### 0.8.0 — Portail client : suivi

> Un client suit l'avancement de son projet et valide un livrable sans
> e-mail ni appel.

**Nouveautés**

- **Mes projets** : statut, avancement, prochains jalons, dernière activité.
- **Projet** : jalons, livrables et fichiers explicitement partagés. Un
  fichier est interne par défaut.
- **Validation des livrables** : consulter, approuver ou demander des
  retours. Notifications à l'équipe, au client à chaque version.

**Technique**

- API dédiée `/api/v1/client/*`, isolée par le client de l'appelant, côté
  SQL. Tests d'isolation bloquants en CI : un compte client ne lit, ne
  modifie et ne devine jamais une donnée d'un autre client.

**Qualité** : utilisable sur mobile.

**Terminé quand** : un client pilote a validé un livrable réel dans le
portail.

### 0.9.0 — Portail client : tickets

> Le support client passe par Piilot, pas par la boîte mail.

**Nouveautés**

- **Dépôt de ticket** : projet, type (anomalie, évolution, assistance),
  description, pièces jointes. Priorité restreinte.
- **Suivi** : fil de discussion avec les réponses. Les messages internes de
  l'équipe ne sont jamais exposés.
- **E-mails bidirectionnels** : nouveau ticket, réponse, changement de statut.

**Technique**

- Isolation : les messages internes restent invisibles, test bloquant.

**Terminé quand** : le support client passe entièrement par le portail pendant
deux semaines.

### 1.0.0 — V1

**Recette** : deux à quatre semaines avec deux ou trois clients pilotes.
Seules des corrections y entrent.

**Critères de sortie**

- [ ] Admin, Team et Client couvrent leur usage quotidien, permissions
      testées.
- [ ] Aucun écran réservé, aucun widget factice : chaque écran est livré ou
      retiré.
- [ ] Isolation du portail couverte par des tests bloquants.
- [ ] Parcours clés couverts par des tests de bout en bout.
- [ ] Tests d'intégration : budget de performance (3 requêtes SQL max par
      endpoint) vérifié.
- [ ] Sauvegardes quotidiennes configurées, restauration testée.
- [ ] Export et suppression RGPD opérationnels.
- [ ] Journal d'audit des actions sensibles.
- [ ] Gestion de projet et CRM ne passent plus par l'ancienne plateforme.
- [ ] Documentation et guide utilisateur à jour.

---

## Exploitation et correctifs

Sortent en correctifs (0.3.1, 0.3.2…), au fil de l'eau :

- **Infrastructure** : déploiement en production, sauvegardes avec
  restauration testée, supervision (Uptime Kuma), alertes.
- **Sécurité** : revue de la surface portail, CSP, limitation de débit.
- **Nettoyage** : correction des textes périmés, retrait des éléments
  factices qui reviennent plus tard.

---

## Décisions à trancher

| # | Question | Recommandation |
|---|---|---|
| **D1** | Que voit le rôle `team` ? | Tous les projets en lecture. Pas de pipeline commercial, pas de montants. |
| **D2** | Domaine de production | Un sous-domaine unique. `COOKIE_DOMAIN` vide. |
| **D3** | Nom affiché | « Piilot » pour le produit, « Plugiit » pour l'agence. |
| **D4** | Fournisseur d'e-mails | Brevo ou Scaleway TEM (hébergé UE). |
| **D5** | Données de l'ancienne plateforme | Saisie manuelle des projets en cours. Import scripté seulement si le volume le justifie. |
| **D6** | Stockage des fichiers | Disque local tant qu'une seule instance. Visibilité par fichier : interne par défaut. |
