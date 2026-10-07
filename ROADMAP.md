# Roadmap

Piilot est l'outil de gestion de projet de l'agence. Il a **trois espaces**,
un par population :

| Espace | Rôle | Ce qu'on y fait |
|---|---|---|
| **Admin** | `admin` | Piloter l'agence : clients et pipeline, budgets et rentabilité, rapports de temps, comptes et droits, paramètres |
| **Team** | `team` | Produire : ses tâches, ses tickets, ses livrables, son temps, les projets sur lesquels on intervient |
| **Client** | `client` | Suivre ses projets, valider les livrables, déposer et suivre ses tickets |

La V1 est publiée. Le cap des trois prochains mois : **un portail client
incontournable** — que le client y trouve ce qu'il venait demander par
e-mail — sans laisser de côté ce qui rend la production sûre (sauvegardes,
RGPD, audit) ni la reprise des données de l'ancienne plateforme (import CSV).
Les trois avancent en parallèle, par petites versions, publiées au fil de
l'eau.

## État actuel

**Version : 1.0.0**, publiée le 2026-10-07.

Les trois espaces couvrent leur usage quotidien. La 1.0 a été tournée vers le
temps de l'équipe : palette Cmd+K, tâches en une ligne, chrono, filtres
mémorisés, adresse et SIRET remplis par les API publiques, intégration GitHub
et GitLab par webhook. Restent ouverts, parmi les critères de sortie annoncés
pour la V1 : sauvegardes testées, export et suppression RGPD, journal
d'audit, tests de bout en bout, recette avec des clients pilotes. Ils sont
répartis dans les versions ci-dessous.

---

## Roadmap

Chaque version apporte des fonctionnalités visibles, et prend sa part des
chantiers de fiabilisation. Les versions sont un ordre, pas des dates : une
version sort quand ce qu'elle contient est prêt.

<!-- roadmap:table -->
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
| ✅ Livrée | 1.1.0 | Portail : livrables et prochaine étape | Client | Livrables sur la fiche projet, validation depuis l'e-mail, prochaine étape, interlocuteurs ; sauvegardes |
| 📍 Actuelle | **1.2.0** | E-mail ↔ tickets | Client | Répondre et créer un ticket par e-mail (IMAP et webhook) ; journal d'audit |
| ⏳ À venir | 1.3.0 | Import et RGPD | Admin | Import CSV clients, contacts, projets, temps ; export et suppression RGPD |
| ⏳ À venir | 1.4.0 | Rapports | Admin | Temps par client et projet en CSV, activité de l'équipe, compte rendu PDF pour le client |
| ⏳ À venir | 1.5.0 | Portail : documents et santé du site | Client | Documents classés avec accusé, historique des mises en ligne, disponibilité et certificat |
| ⏳ À venir | 1.6.0 | Fin de l'ancienne plateforme | Tous | Actions en masse, tests de bout en bout, recette, guide utilisateur |
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
- Écrans réservés : comptes (0.5), modèles de projet et interactions CRM
  (0.7). Les rapports de temps sont livrés.
- Le rôle `team` voit le même back-office qu'`admin` → 0.6.
- Le portail client n'a aucun écran fonctionnel → 0.8 et 0.9.

### 0.4.0 — Temps et budgets

> Piloter l'agence depuis Piilot. Remplace les calculs manuels et les
> feuilles de calcul par des données toujours à jour.

**Nouveautés**

- **Rapports de temps** ✅ : par projet, personne, service, client et période ;
  facturable et non facturable ; export CSV.
- **Budgets réels** ✅ : heures vendues, consommées (depuis la saisie du temps),
  restantes. Alerte « à surveiller » à 80 % du budget, « hors budget » au-delà.
  Filtre et tri par budget dans la liste des projets.
- **Tableau de bord** ✅ : compteurs réels (projets, clients, heures, services),
  à la place des widgets factices. Activité de l'année, charge par projet,
  temps de chaque personne aujourd'hui et sur la semaine. L'agenda factice est
  retiré, il revient avec le planning en 0.7.

**Technique**

- Les cumuls de temps et budgets sont **précalculés** à chaque saisie, jamais
  recalculés à l'affichage.

**Terminé quand** : un mois se clôture (temps, budgets) sans ouvrir une
feuille de calcul.

### 0.5.0 — Gestion des comptes

> Plus aucun compte ne se crée en shell. L'admin gère les comptes et les
> rôles depuis Piilot.

**Nouveautés**

- **Écran Comptes** ✅ : liste, création, changement de rôle, désactivation,
  réactivation. Désactiver révoque les sessions.
- **Invitations par e-mail** ✅ : lien à usage unique, qui expire. La personne
  invitée choisit son mot de passe.
- **Mot de passe oublié** ✅ : jeton à usage unique, courte durée, limitation de
  débit. Réponse identique que l'adresse existe ou non.
- **Écran Rôles** ✅ : consulter et modifier les permissions de chaque rôle.

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

- **Page « Mon travail »** ✅ : tâches en retard, assignations, tickets,
  livrables à déposer. Lien vers chaque projet où on intervient.
- **Feuille de temps hebdomadaire** ✅ : pointage à la semaine, en plus de la
  saisie par jour.
- **Navigation selon les permissions** ✅ : le front lit les droits retournés
  par `/auth/me` et n'affiche que les entrées accessibles. Pas de tableau de
  bord d'agence, pas de pipeline commercial, pas de budgets, pas de rapports.
  L'équipe garde les fiches clients et les contacts, dont elle a besoin pour
  travailler.
- **Notifications** ✅ : étendues aux tickets (assignation, réponse, changement
  de statut) et aux livrables (validation, retours).

**Technique**

- Nouvelles permissions pour ce que `team` ne doit plus voir ✅ :
  `dashboard.read`, `budgets.read`, `pipeline.read`. `time.read` devient la
  lecture des rapports de toute l'équipe ; `time.write` protège enfin la
  saisie.

**Terminé quand** : un chef de projet passe une semaine complète dans son
espace sans onglet d'admin.

### 0.7.0 — Projets et CRM avancés

> Gérer les projets complets et le pipeline commercial sans passer par
> l'ancienne plateforme.

**Nouveautés**

- **Jalons** ✅ : étapes datées du projet, reliées aux livrables, avec le
  compte des livrables validés. Leur affichage dans le portail client vient
  avec la 0.8.0, qui ouvre l'API du portail.
- **Planning** ✅ : vue calendrier des jalons, des échéances de projets et des
  tâches de chacun, remplaçant le lien vers l'agenda partagé. En lecture
  seule, sans synchronisation avec un agenda extérieur.
- **Modèles de projet** ✅ : créer un projet avec ses tâches, jalons et
  services pré-remplis.
- **Interactions CRM** ✅ : journal des notes, appels, rendez-vous et e-mails
  saisis à la main, et des événements automatiques (projet créé, livrable
  validé, ticket ouvert).

**Terminé quand** : tous les projets en cours ont leurs jalons et livrables,
et le pipeline commercial se gère en kanban.

### 0.8.0 — Portail client : suivi

> Un client suit l'avancement de son projet et valide un livrable sans
> e-mail ni appel.

**Nouveautés**

- **Mes projets** ✅ : statut, avancement, prochains jalons, dernière activité.
- **Projet** ✅ : jalons, livrables et fichiers explicitement partagés. Un
  fichier est interne par défaut.
- **Validation des livrables** ✅ : consulter, approuver ou demander des
  retours. Notifications à l'équipe, e-mail au client à chaque version.

**Technique**

- API dédiée `/api/v1/client/*` ✅, isolée par le client de l'appelant, côté
  SQL. Tests d'isolation bloquants en CI : un compte client ne lit, ne
  modifie et ne devine jamais une donnée d'un autre client.

**Qualité** : utilisable sur mobile ✅.

**Terminé quand** : un client pilote a validé un livrable réel dans le
portail.

### 0.9.0 — Portail client : tickets

> Le support client passe par Piilot, pas par la boîte mail.

**Nouveautés**

- **Dépôt de ticket** ✅ : projet, type (anomalie, évolution, assistance),
  description, pièces jointes. Priorité restreinte (basse, normale, haute).
- **Suivi** ✅ : fil de discussion avec les réponses. Les messages internes de
  l'équipe ne sont jamais exposés.
- **E-mails bidirectionnels** ✅ : nouveau ticket, réponse, changement de statut.

**Technique**

- Isolation ✅ : les messages internes restent invisibles, test bloquant. Les
  tickets ouverts par l'agence pour elle-même restent internes.

**Terminé quand** : le support client passe entièrement par le portail pendant
deux semaines.

### 1.0.0 — V1 ✅

Publiée le 2026-10-07. Palette Cmd+K, tâches en une ligne, Mon travail qui
agit, chrono, filtres mémorisés, relance des livrables, invitation portail
depuis un contact, adresse autocomplétée, logo par projet, jalon atteint par
ses livrables, intégration GitHub/GitLab, session qui ne s'expire plus.

Critères de sortie encore ouverts, repris ci-dessous : sauvegardes (1.1),
journal d'audit (1.2), export et suppression RGPD (1.3), tests de bout en
bout et recette (1.6).

---

### 1.1.0 — Portail : livrables et prochaine étape

Le client voit ce qui vient et répond sans se connecter.

**Agence**

- Onglet **Livrables** sur la fiche projet : la liste, le statut, « Nouveau
  livrable » avec le projet déjà choisi et le jalon proposé. Aujourd'hui il
  faut passer par l'écran global filtré.
- **Lien de rendez-vous** par membre de l'équipe (Cal.com, Calendly, Google),
  dans Mon compte. Pas de créneaux dans Piilot : rien à synchroniser.

**Portail**

- **Validation depuis l'e-mail** : « Valider » et « Faire un retour » dans
  l'e-mail de dépôt, par un lien signé à durée limitée. Un retour ouvre la
  page avec le champ prêt.
- **Prochaine étape** sur la fiche projet : le prochain jalon daté, ce qui
  attend le client (livrables à valider, questions ouvertes), ce que
  l'agence fait en ce moment.
- **Interlocuteurs** : le chargé de compte et l'équipe du projet, avec
  e-mail, téléphone et lien de rendez-vous.

**Fiabilisation : sauvegardes**

- Service `backup` dans le `docker-compose.yml` : dump quotidien de Postgres
  et des fichiers, rétention (7 jours, 4 semaines, 3 mois), destination
  locale ou S3 compatible (UE).
- Restauration documentée et **testée en CI** : un dump restauré, l'API
  démarre dessus.
- Date de la dernière sauvegarde réussie visible dans Paramètres, alerte
  aux admins quand elle manque.

---

### 1.2.0 — E-mail ↔ tickets

Le client écrit un e-mail, Piilot en fait un ticket ; il répond à l'e-mail,
sa réponse arrive sur le ticket.

- **Réponse par e-mail** : chaque notification de ticket part avec un
  identifiant de fil ; la réponse du client, reconnue par `In-Reply-To` et
  par l'adresse `support+<ticket>@`, s'inscrit comme message du ticket, avec
  ses pièces jointes.
- **Création par e-mail** : un e-mail d'un contact connu, hors fil, crée un
  ticket sur le projet déduit (un seul projet actif pour ce client, sinon
  « à classer »). Un expéditeur inconnu est mis en attente, jamais rejeté
  en silence.
- **Deux branchements**, au choix dans Paramètres : une boîte **IMAP**
  relevée par un job chaque minute (marche partout, rien à exposer), et un
  **webhook entrant** du fournisseur d'e-mail (Brevo, Mailgun, Postmark)
  pour l'instantané. Les deux écrivent en base ; les écrans lisent la base.
- Réponses types sur les tickets, pour les demandes qui reviennent.

**Fiabilisation : journal d'audit**

- Table d'audit en ajout seul : connexions et échecs, changements de rôle et
  de permission, invitations, suppressions, exports, accès au portail,
  changements de secret. Qui, quand, d'où.
- Écran *Paramètres > Audit*, filtrable, exportable en CSV. Rétention
  réglable.

---

### 1.3.0 — Import et RGPD

Ramener les données de l'ancienne plateforme, et savoir les rendre ou les
effacer.

- **Import CSV** depuis l'interface, par entité : clients, contacts, projets,
  temps passé. Aperçu des premières lignes, correspondance des colonnes
  mémorisée, détection des doublons (SIRET, e-mail, nom), rapport de ce qui
  est entré et de ce qui a été ignoré, et un import rejouable sans doublon.
  Sert aux autres agences autant qu'à la reprise.
- **Export CSV** des mêmes entités, avec les mêmes colonnes : ce qui entre
  peut ressortir.
- **RGPD** : export de tout ce qui concerne un client ou un compte (JSON et
  fichiers, dans une archive), suppression définitive avec délai de
  rétractation, purge des comptes inactifs selon une durée réglable. Chaque
  export et suppression au journal d'audit.

---

### 1.4.0 — Rapports

Ce que l'agence a fait, pour elle et pour le client.

- **Temps par client et par projet** sur une période, facturable ou non, par
  personne et par prestation, export **CSV** pour la facturation, qui reste
  sur l'ancienne plateforme.
- **Activité de l'équipe** : charge par personne, temps saisi contre temps
  attendu, tâches livrées, retards, tickets traités.
- **Compte rendu PDF** pour le client, mensuel ou à la demande : temps passé,
  tickets traités, livrables validés, mises en ligne. Posé sur le portail
  et envoyé par e-mail. Génération par un job, jamais dans la requête.

---

### 1.5.0 — Portail : documents et santé du site

Le portail devient l'endroit où l'on retrouve tout ce que l'agence a remis.

- **Documents classés** par type — devis, contrat, compte rendu, maquette,
  autre — avec versions, et un **accusé** d'un clic côté client (« lu »,
  « accepté ») horodaté et visible de l'agence. Pas de signature
  électronique : sans prestataire, elle n'aurait pas de valeur ; avec, c'est
  une dépendance payante. Le devis signé reste un PDF déposé.
- **Historique des mises en ligne** : les mises en ligne reçues par Git,
  avec ce qui a changé (tickets clos, pull requests fusionnées).
- **Santé du site** : un job appelle l'URL de production toutes les cinq
  minutes — en ligne ou non, temps de réponse, certificat et sa date
  d'expiration. Alerte à l'équipe au second échec, pastille « tout va bien »
  sur le portail. Pas de SEO : ça reste sur l'ancienne plateforme.

---

### 1.6.0 — Fin de l'ancienne plateforme pour la gestion de projet et le CRM

Ce qui manque encore pour ne plus y retourner, et clore la V1.

- **Actions en masse** sur tâches, tickets et livrables : réassigner,
  décaler, changer le statut, archiver.
- **Tests de bout en bout** en CI sur les parcours clés : connexion, projet,
  ticket, validation client, e-mail entrant, import.
- **Recette** avec deux ou trois clients pilotes sur le portail ; seules des
  corrections entrent pendant ce temps.
- **Guide utilisateur** pour l'équipe et pour le client, servi depuis
  l'application.
- Critères de sortie de la V1 tous cochés ; l'ancienne plateforme ne garde
  que la facturation, le SEO, le CMS et le RH.

---

## Exploitation et correctifs

Sortent en correctifs (1.0.1, 1.0.2…), au fil de l'eau :

- **Infrastructure** : supervision et alertes de l'instance elle-même
  (les sauvegardes sont en 1.1, la santé des sites clients en 1.5).
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
| **D5** | Données de l'ancienne plateforme | Import CSV depuis l'interface (1.3), par entité, rejouable. Pas de lecture directe de l'ancienne base. |
| **D6** | Stockage des fichiers | Disque local tant qu'une seule instance. Visibilité par fichier : interne par défaut. |
| **D7** | E-mail entrant | Les deux : boîte IMAP relevée par un job (par défaut), webhook du fournisseur en option. Jamais dans la requête HTTP. |
| **D8** | Rendez-vous | Un lien externe par membre (Cal.com, Calendly, Google). Pas de créneaux dans Piilot. |
| **D9** | Devis et contrats sur le portail | PDF classés avec accusé « lu / accepté » horodaté. Pas de signature électronique. |
| **D10** | Santé du site | Disponibilité, temps de réponse et certificat, par un job. Pas de SEO. |
| **D11** | Cadence | Au fil de l'eau : une version dès qu'une fonction est prête, dans l'ordre du tableau. |
