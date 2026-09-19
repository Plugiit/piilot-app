# Roadmap vers la V1

Piilot est l'outil de gestion de projet de l'agence. Il a **trois espaces**,
un par population :

| Espace | Rôle | Ce qu'on y fait |
|---|---|---|
| **Admin** | `admin` | Piloter l'agence : clients et pipeline, budgets et rentabilité, rapports de temps, comptes et droits, paramètres |
| **Team** | `team` | Produire : ses tâches, ses tickets, ses livrables, son temps, les projets sur lesquels on intervient |
| **Client** | `client` | Suivre ses projets, valider les livrables, déposer et suivre ses tickets |

La **V1** est atteinte quand chacun des trois espaces couvre son usage
quotidien sans écran factice ni recours à l'ancienne plateforme pour la
gestion de projet et le CRM.

## État des lieux à la 0.3.0

| Espace | État |
|---|---|
| Admin | Largement en place : projets, tâches, tickets, livrables, temps, CRM, paramètres. Manquent la gestion des comptes, les rapports, les budgets consommés et un tableau de bord entièrement réel. |
| Team | **N'existe pas en tant qu'espace.** Le rôle `team` voit le même back-office que `admin`, avec toutes les permissions sauf `users.write` et `roles.write`. |
| Client | **Coquille vide.** Connexion et navigation en place, aucun endpoint côté API, deux écrans d'attente. |

Dettes visibles, à solder avant la V1 :

- **Widgets factices sur le tableau de bord** : la carte d'activité est tirée
  au hasard, le « temps facturable » est codé en dur (1 020 €) et l'agenda est
  vide.
- **Page de connexion** : elle montre la maquette avec des chiffres fictifs et
  cinq étoiles, et un lien « Mot de passe oublié » qui ne mène nulle part.
- **Textes périmés** : la page budget d'un projet dit que le suivi du temps
  « n'est pas encore écrit », et le portail attend « le module PM » alors que
  les deux existent.
- **Écrans réservés** : rapports de temps, interactions CRM, modèles de
  projet, gestion des comptes.
- **Aucun e-mail** : pas d'invitation ni de réinitialisation de mot de passe.
  Un compte se crée aujourd'hui en shell, avec `/app/seed`.

## Vue d'ensemble

Le statut de chaque release et la version actuelle sont calculés depuis le
fichier `VERSION` par `scripts/roadmap.sh`, lancé à chaque release. Le même
tableau est repris dans le README. Pour modifier la roadmap, éditer les
colonnes *Version* à *Objectif* ci-dessous puis lancer `make roadmap`.

<!-- roadmap:table -->
| Statut | Version | Nom | Espace | Objectif |
|---|---|---|---|---|
| ✅ Livrée | 0.1.0 | Socle | Tous | Connexion, sessions, rôles et permissions |
| ✅ Livrée | 0.2.0 | Gestion de projet | Admin | Projets, tâches, notifications, compte |
| 📍 Actuelle | **0.3.0** | Back-office PM et CRM | Admin | Tickets, livrables, temps, CRM, image unique |
| ⏳ À venir | 0.4.0 | En production | Admin | Utiliser Piilot en interne, sur des données réelles et sauvegardées |
| ⏳ À venir | 0.5.0 | Comptes et e-mails | Tous | Inviter, réinitialiser, gérer les comptes sans shell |
| ⏳ À venir | 0.6.0 | Espace team | Team | Une journée de production sans passer par les écrans d'admin |
| ⏳ À venir | 0.7.0 | Pilotage | Admin | Rapports, budgets, jalons : ne plus dépendre de l'ancienne plateforme |
| ⏳ À venir | 0.8.0 | Portail : suivi et validation | Client | Un client suit son projet et valide un livrable dans Piilot |
| ⏳ À venir | 0.9.0 | Portail : tickets | Client | Le support client passe par Piilot |
| ⏳ À venir | 0.10.0 | Conformité et durcissement | Tous | RGPD, audit, sécurité du portail, tests de bout en bout |
| ⏳ À venir | 1.0.0-rc.N | Recette | Tous | Clients pilotes, corrections |
| ⏳ À venir | 1.0.0 | V1 | Tous | Critères de sortie remplis |
<!-- /roadmap:table -->

L'ordre suit les dépendances :

- Rien ne sert tant que l'outil n'est pas en production (0.4).
- Le portail et l'arrivée de l'équipe exigent les invitations par e-mail (0.5).
- Le portail affiche des jalons et des livrables que l'agence doit d'abord
  savoir produire (0.6, 0.7).
- La conformité se vérifie sur un périmètre complet (0.10).

Chaque version est déployable et utilisable seule.

---

## 0.4.0 — En production

> Piilot sert l'agence au quotidien, sur des données réelles, sauvegardées
> et surveillées.

**Infrastructure**

- Trancher le domaine ([D2](#décisions-à-trancher)) et déployer sur Coolify.
- Comptes de l'équipe créés avec `/app/seed`, en attendant les invitations
  de la 0.5.
- Sauvegarde quotidienne de la base et des pièces jointes, envoyée hors du
  serveur (stockage S3 européen). Rétention : 7 jours, plus 4 semaines en
  hebdomadaire.
- **Une restauration testée** sur une instance vierge, et sa procédure écrite.
- Supervision : Uptime Kuma sur `/health/ready`, alertes par webhook.

**Nettoyage**

- Tableau de bord : retirer la carte d'activité aléatoire, le temps facturable
  codé en dur et l'agenda vide. Ils reviennent en 0.7, sur des données réelles.
- Page de connexion : visuel sans chiffres ni avis fictifs. Masquer
  « Mot de passe oublié » jusqu'à la 0.5.
- Corriger les textes périmés (budget d'un projet, portail).
- Nom affiché cohérent : Piilot pour le produit, Plugiit pour l'agence
  ([D3](#décisions-à-trancher)).
- Version affichée dans l'application (menu du compte), lue sur
  `/health/live`.

**Terminé quand** les projets en cours de l'agence sont saisis dans Piilot et
qu'une restauration a été faite avec succès.

## 0.5.0 — Comptes et e-mails

> Plus aucun compte ne se crée en shell.

**E-mails transactionnels**

- Fournisseur européen ([D4](#décisions-à-trancher)), SPF, DKIM et DMARC
  configurés.
- Envoi par une **file d'attente en base** (table d'envoi et tâche de fond).
  Aucun appel au fournisseur pendant une requête HTTP. Nouvel essai en cas
  d'échec, et trace de chaque envoi.
- Modèles en français, texte brut et HTML.

**Comptes**

- *Paramètres → Comptes* (aujourd'hui réservé) : liste, création, changement
  de rôle, désactivation et réactivation. Désactiver un compte révoque ses
  sessions.
- **Invitations** : un lien à usage unique qui expire, envoyé par e-mail. La
  personne invitée choisit elle-même son mot de passe.
- **Mot de passe oublié** : jeton à usage unique et courte durée, limitation
  de débit, même réponse que l'adresse existe ou non.
- *Paramètres → Rôles* : consulter les permissions de chaque rôle et les
  modifier (`roles.write` existe déjà côté API).

**Terminé quand** un nouveau membre de l'équipe rejoint Piilot par une
invitation, sans aucune commande.

## 0.6.0 — Espace team

> Un membre de l'équipe fait sa journée dans Piilot sans voir ce qui ne le
> concerne pas.

**Droits**

- Appliquer [D1](#décisions-à-trancher) par une migration : nouvelles
  permissions pour ce que `team` ne doit plus voir (pipeline commercial,
  montants, budgets).
- **Navigation selon les permissions** : le front lit les droits renvoyés par
  `/auth/me` et n'affiche que les entrées accessibles. L'API reste la seule
  garde.
- Tests d'autorisation : pour chaque route du back-office, ce que `team`
  obtient et ce qui lui est refusé.

**« Mon travail »**, page d'accueil du rôle `team` :

- Mes tâches : en retard, aujourd'hui, cette semaine.
- Mes tickets assignés, les livrables à déposer.
- Mon temps de la semaine.

**Temps**

- Feuille de temps hebdomadaire, en plus de la saisie par jour.
- Pointer directement depuis une tâche.

**Notifications**

- Étendues aux tickets (assignation, réponse, changement de statut) et aux
  livrables (validation, retours).
- Préférences par type, dans l'application ou par e-mail. L'écran existe
  déjà, vide.

**Terminé quand** un membre de l'équipe passe une semaine complète dans son
espace sans écran d'admin.

## 0.7.0 — Pilotage

> L'admin pilote l'agence depuis Piilot. L'ancienne plateforme n'est plus
> ouverte pour la gestion de projet ni le CRM.

**Temps et budgets**

- *Rapports de temps* (aujourd'hui réservé) : par projet, personne, service
  et période ; export CSV.
- Budget d'un projet : heures vendues, consommées et restantes, avec une
  alerte de dépassement.
- Cumuls **précalculés** à chaque saisie, jamais recalculés à l'affichage.

**Tableau de bord réel**

- Activité réelle, charge par personne, projets à risque (échéance proche et
  avancement en retard).
- Remplace les widgets retirés en 0.4.

**Projets**

- **Jalons** : étapes datées d'un projet, reliées aux livrables. Le portail
  s'appuiera dessus en 0.8.
- **Planning** : vue calendrier des jalons et des échéances, en remplacement
  du lien vers l'agenda partagé.
- *Modèles de projet* (aujourd'hui réservé) : créer un projet avec ses
  tâches, ses jalons et ses services pré-remplis.

**CRM**

- *Interactions* (aujourd'hui réservé) : journal par client, avec les notes et
  appels saisis à la main et les événements automatiques (projet créé,
  livrable validé, ticket ouvert).

**Terminé quand** un mois se clôture (temps, budgets, suivi commercial) sans
ouvrir l'ancienne plateforme pour la gestion de projet ni le CRM.

## 0.8.0 — Portail client : suivi et validation

> Un client suit l'avancement de son projet et valide un livrable sans
> e-mail ni appel.

**Isolation, avant tout le reste**

- API dédiée `/api/v1/client/*`, distincte du back-office. Chaque requête est
  filtrée par le client de l'appelant, **côté SQL**.
- Tests d'isolation systématiques et bloquants en CI : un compte client ne
  lit, ne modifie et ne devine jamais une donnée d'un autre client, même en
  forgeant un identifiant.

**Accès**

- Invitation de comptes portail depuis la fiche client (réutilise la 0.5).

**Écrans**

- Accueil : mes projets, avec statut, avancement, prochains jalons et
  dernière activité.
- Projet : jalons, livrables et fichiers **explicitement partagés**. Un
  fichier est interne par défaut ([D6](#décisions-à-trancher)).
- Livrables : consulter, valider, ou demander des retours avec un
  commentaire. L'équipe est prévenue par e-mail, et le client à chaque
  nouvelle version.

**Qualité** : utilisable sur mobile, identité visuelle de l'agence.

**Terminé quand** un client pilote a validé un livrable réel dans le portail.

## 0.9.0 — Portail client : tickets

> Les demandes des clients passent par Piilot, pas par la boîte mail.

- Le client dépose un ticket : projet, type (anomalie, évolution,
  assistance), description, pièces jointes. Il n'a accès qu'à un jeu de
  priorités restreint.
- Suivi et réponse dans le fil de discussion. **Les messages internes ne sont
  jamais exposés**, et un test le garantit.
- E-mails dans les deux sens : nouveau ticket, réponse, changement de statut.
- Côté équipe : la file des tickets clients, avec leur assignation.

**Terminé quand** le support des clients pilotes passe entièrement par le
portail pendant deux semaines.

## 0.10.0 — Conformité et durcissement

> Le portail est une surface exposée à l'extérieur : on le traite comme tel.

**RGPD**

- Export des données d'un compte, d'un contact ou d'un client.
- Suppression, ou anonymisation quand une obligation de conservation
  s'applique.
- Durées de conservation appliquées par tâche de fond, registre des
  traitements, mentions légales et politique de confidentialité du portail.

**Sécurité**

- Journal d'audit des actions sensibles : droits, suppressions, validations,
  exports.
- Revue de sécurité de la surface portail, politique de sécurité du contenu
  (CSP), limitation de débit sur les routes du portail, dépendances à jour.

**Qualité**

- Tests de bout en bout (Playwright) sur les parcours clés des trois espaces,
  en CI.
- Budget de performance vérifié par test : au plus 3 requêtes SQL par
  endpoint, avec une taille de réponse bornée.

**Données**

- Stockage des fichiers tranché ([D6](#décisions-à-trancher)).
- Reprise des données de l'ancienne plateforme, si [D5](#décisions-à-trancher)
  le retient. Les modules concernés sont alors entièrement réécrits : c'est
  la seule condition posée par le principe du projet.

**Documentation** : un guide court par espace.

## 1.0.0 — V1

Passage par `1.0.0-rc.1`, `rc.2`… pendant une recette de deux à quatre
semaines avec deux ou trois clients pilotes. Seules des corrections y entrent.

**Critères de sortie**

- [ ] Les trois espaces couvrent leur usage quotidien, chacun avec des
      permissions testées.
- [ ] Aucun écran réservé ni widget factice : chaque écran est livré ou
      retiré.
- [ ] Isolation du portail couverte par des tests bloquants.
- [ ] Parcours clés couverts par des tests de bout en bout.
- [ ] Sauvegarde quotidienne, restauration testée dans le mois.
- [ ] Export et suppression RGPD opérationnels.
- [ ] La gestion de projet et le CRM ne passent plus par l'ancienne
      plateforme.
- [ ] Documentation d'installation et guide utilisateur à jour.

## Après la V1

À réévaluer une fois la V1 en service, pas avant :

- Recherche globale (le champ existe dans la barre latérale).
- Intégrations : GitHub (commits et déploiements d'un projet), Figma, agenda.
- Tableau de bord client enrichi (temps consommé, si l'agence veut le
  montrer).

Restent hors périmètre, par décision : facturation et comptabilité,
monitoring SEO, CMS et blog, RH, multi-agence, application mobile.

---

## Décisions à trancher

| # | Question | Nécessaire pour | Recommandation |
|---|---|---|---|
| **D1** | Que voit le rôle `team` ? | 0.6 | Tous les projets **en lecture**, mais « Mon travail » par défaut. Pas de pipeline commercial, pas de montants ni de budgets. Une petite agence travaille en entraide ; ce sont les données commerciales et financières qu'il faut isoler, pas les projets. |
| **D2** | Domaine de production | 0.4 | Un sous-domaine unique pour l'application, `COOKIE_DOMAIN` vide. Pas d'alias `admin.` : l'espace se déduit du rôle, pas de l'URL. |
| **D3** | Nom affiché | 0.4 | « Piilot » pour le produit. « Plugiit » reste la marque de l'agence, visible dans le portail client. |
| **D4** | Fournisseur d'e-mails | 0.5 | Un fournisseur hébergé dans l'UE avec API et webhooks de rebond (Brevo ou Scaleway TEM). Choix sur le prix au volume réel. |
| **D5** | Données de l'ancienne plateforme | 0.4 et 0.10 | Saisie manuelle des projets en cours en 0.4. Import scripté seulement si le volume historique le justifie, et seulement vers des modules complets. |
| **D6** | Stockage des fichiers | 0.8 et 0.10 | Disque local sauvegardé tant qu'il n'y a qu'une instance. Un stockage objet européen seulement si le volume ou une seconde instance l'exige. Visibilité par fichier : interne par défaut. |

## Versionnement

- **SemVer.** Avant la V1, chaque version de cette roadmap est une **mineure**
  (`0.x.0`) ; les corrections entre deux sont des **patchs** (`0.x.y`).
- Une version peut être découpée (`0.6.0`, `0.6.1`…) si elle est trop grosse
  pour sortir d'un bloc. Elle n'est jamais publiée à moitié sous son nom.
- Après la V1 : mineure pour une fonctionnalité, patch pour une correction,
  majeure seulement pour un changement qui casse l'existant (migration de
  données irréversible, API modifiée).
- La publication se fait avec `make release` (voir le README).
