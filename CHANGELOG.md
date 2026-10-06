# Changelog

Toutes les versions notables de Piilot. Format inspiré de
[Keep a Changelog](https://keepachangelog.com/fr/1.1.0/), numérotation
[SemVer](https://semver.org/lang/fr/). La feuille de route est dans
[ROADMAP.md](ROADMAP.md).

Chaque section est rédigée au moment de la release, à partir d'un brouillon
de Claude Code relu avant publication, puis reprise telle quelle sur la page
de release GitHub. Voir « Publier une version » dans le README.

<!-- releases -->

## [0.9.5] — Mise à jour sans coupure et clients particuliers · 2026-10-06

Cette version met Piilot à jour sans interruption de service et distingue, à la création d'un client, un particulier d'un professionnel dont le SIRET est vérifié au registre des entreprises.

### Points forts

- Mise à jour sans coupure : la nouvelle version démarre à côté de l'ancienne, et le trafic bascule de l'une à l'autre sans qu'une requête échoue.
- Clients particuliers ou professionnels, avec vérification du SIRET au registre des entreprises.
- Une carte « Nouvelle version » au bas du menu pour installer la mise à jour.

### Nouveautés

#### Admin et Team

- **Client particulier ou professionnel** : deux cartes pour choisir à la création. Pour un professionnel, le SIRET est vérifié au registre des entreprises (API publique de l'État, gratuite) : établissement actif, fermé ou inconnu. Le registre remplit la forme juridique, l'adresse et le numéro de TVA, et la raison sociale saisie y est comparée, avec de quoi reprendre celle du registre d'un clic. Pour un particulier : prénom, nom, e-mail, téléphone et adresse ; la personne devient le contact principal.
- **Fiche client** : raison sociale, forme juridique et SIRET marqué « Vérifié » pour un professionnel ; un particulier n'affiche que ses coordonnées. Un SIRET changé à la modification est revérifié.

#### Admin

- **Carte « Nouvelle version »** au bas du menu latéral, à la place du bouton de l'en-tête : la version disponible, celle installée, et un bouton Installer. Pendant l'installation, elle en suit l'avancement.

#### Toute l'application

- **Mise à jour sans coupure** : une passerelle se place devant l'application et ne s'arrête jamais. Pendant une mise à jour, la nouvelle version démarre à côté de l'ancienne ; la passerelle bascule le trafic dès qu'elle est saine, puis l'ancienne s'arrête après avoir fini ses requêtes. Si la nouvelle version ne démarre pas, l'ancienne n'a jamais cessé de servir. Rien ne dépend de Coolify ni de Traefik.
- **Page de maintenance** : si le serveur ne répond plus pendant plus de quelques secondes, la passerelle sert une page « Mise à jour en cours » qui se recharge d'elle-même, et un onglet ouvert affiche le même écran.

### Améliorations

- Un onglet ouvert avant une mise à jour se recharge de lui-même quand il lui manque un morceau de la nouvelle version, au lieu d'échouer en changeant d'écran.

### Corrections

- Le champ téléphone de « Mon compte » est de nouveau utilisable : le sélecteur d'indicatif occupait toute la ligne.
- La limite de tentatives de connexion par adresse IP valait pour tout le monde derrière un proxy : l'application voyait l'adresse du proxy, et non celle de l'appelant.
- La mise à jour depuis l'interface échouait sur les serveurs Docker récents (stockage d'images containerd) : l'image en cours devenait illisible après le téléchargement de la nouvelle.

### À savoir pour le déploiement

- **Redéployer une fois** (*Redeploy* dans Coolify, ou `git pull && docker compose up -d --remove-orphans`). La pile passe à cinq services : `app` devient la passerelle et garde le domaine configuré, l'application devient `server`. Ce redéploiement coupe brièvement ; les mises à jour suivantes, non. Une mise à jour depuis l'interface sans ce redéploiement fonctionne, mais sans passerelle, donc avec coupure.
- `create-admin` se lance désormais dans le conteneur `server`.
- Nouvelle variable `DRAIN_DELAY` (5 s par défaut) : le temps pendant lequel une instance qui s'arrête sert encore.
- Migration 000036 : type de client, raison sociale, forme juridique et date de vérification du SIRET. Les clients existants deviennent professionnels.
- Les migrations suivent désormais une règle : elles ajoutent sans supprimer ni renommer dans la même version (voir le README).

## [0.9.4] — Agenda et finitions · 2026-10-06

Cette version remet l'agenda sur le tableau de bord, cette fois avec les vraies échéances, et soigne plusieurs écrans du quotidien.

### Nouveautés

#### Admin et Team

- **Agenda du tableau de bord** : le panneau de droite revient. Il montre la semaine du jour choisi avec les jalons et les échéances des projets, et ses propres tâches, filtrables ; un point marque les jours chargés, et chaque entrée mène au projet, à ses jalons ou à la tâche. Il se replie toujours depuis l'en-tête.
- **Client d'un nouveau projet** : une liste déroulante des clients, avec recherche, remplace le champ texte. Un client absent se crée depuis la même liste, marqué « nouveau ».

### Améliorations

- Les chiffres de « Mon travail » prennent la carte du tableau de bord.
- Sans photo de profil, le bas du rail affiche les initiales du compte sur fond orange.

### Corrections

- La fenêtre de création de projet défile : sur un écran de 900 px de haut, le bouton de création restait hors d'atteinte.

## [0.9.3] — Nouvelles versions détectées · 2026-10-06

Cette version fait apparaître une nouvelle version de Piilot le jour même de sa sortie, sur toutes les installations, sans rien configurer côté GitHub.

### Points forts

- Vérification des nouvelles versions tous les quarts d'heure, au lieu de toutes les six heures.
- « Rechercher une mise à jour » dans le menu du compte, pour vérifier sans attendre.
- Une nouvelle version s'annonce dans la cloche des admins.

### Nouveautés

#### Admin

- **Rechercher une mise à jour** (menu du compte, en bas du rail) : la réponse arrive en quelques secondes — « Piilot est à jour », ou « Piilot 0.9.4 est disponible » avec le bouton « Mettre à jour » dans l'en-tête.
- **Annonce dans la cloche** : chaque admin est prévenu d'une nouvelle version, une seule fois.
- **Bouton de l'en-tête** : il apparaît dans les deux minutes qui suivent la détection d'une version.

### Technique

- Chaque instance interroge GitHub elle-même, en requête conditionnelle : quand rien n'a changé, GitHub répond 304, ce qui ne compte pas dans sa limite d'appels. Aucun webhook à configurer, donc valable pour toute installation auto-hébergée.
- `UPDATE_CHECK_INTERVAL` règle l'intervalle (15 minutes par défaut, 5 au minimum) ; `UPDATE_CHECK=false` coupe toujours tout appel.
- `POST /api/v1/admin/system/update/check` : la vérification part de la tâche de fond, dans les quinze secondes, jamais pendant la requête.
- Migration `000035` : empreinte de la dernière réponse, demande de vérification, dernière version annoncée.
- Les instances en 0.9.2 ou avant vérifient encore toutes les six heures : un Redeploy les met sur ce nouveau rythme.

## [0.9.2] — Un domaine par rôle · 2026-10-06

Cette version corrige le passage d'un domaine à l'autre en multi-domaines : un administrateur fait désormais tout son travail sur le domaine d'administration, sans jamais en changer.

### Corrections

- **Back-office sur le domaine du rôle** : le domaine d'administration sert tout le back-office — Mon travail, projets, tâches, CRM, temps — en plus du tableau de bord de l'agence et des paramètres. Un administrateur y navigue sans changer de domaine ni recharger la page ; l'équipe garde le sien, sans les écrans d'administration.
- **Ramené sur son domaine** : un administrateur qui ouvre une adresse du domaine de l'équipe arrive sur la même page du domaine d'administration, et inversement pour l'équipe.
- **Après la connexion**, un administrateur va directement sur le domaine d'administration.
- **E-mails de tickets** : le lien envoyé à un administrateur pointe vers le domaine d'administration.

## [0.9.1] — Un domaine par espace · 2026-10-06

Cette version permet de donner à chaque espace de Piilot son propre domaine — connexion, équipe, administration, portail client — et rend l'ajout d'une interaction CRM plus évident.

### Points forts

- Un domaine par espace, au choix : `auth`, `team`, `admin` et `client`, avec une seule connexion pour tous.
- Ajout d'une interaction depuis un bouton dédié, en deux temps : le client, puis ce qui s'est dit.

### Nouveautés

#### Tous les espaces

- **Un domaine par espace** (facultatif) : avec `AUTH_URL`, `TEAM_URL`, `ADMIN_URL` et `CLIENT_URL`, la connexion, le travail de l'équipe, l'administration et le portail ont chacun leur domaine. Une adresse ouverte sur le mauvais domaine est renvoyée vers le bon, et les e-mails pointent vers le domaine de leur espace. Sans ces variables, tout reste sur un seul domaine.

#### Admin et Team

- **Ajouter une interaction** (CRM › Interactions) : un bouton à droite, séparé des filtres, ouvre une fenêtre qui demande d'abord le client, cherché par son nom, puis affiche le formulaire seul. Sa hauteur s'anime d'une étape à l'autre.
- **Administration réservée aux administrateurs** en multi-domaines : l'équipe arrive sur « Mon travail » et ne voit plus les paramètres ; les référentiels (services, modèles de projet) se modifient depuis l'administration.

### Technique

- Les quatre domaines partagent un domaine parent, sur lequel se pose le cookie de session : `COOKIE_DOMAIN` s'en déduit. L'API refuse au démarrage une configuration incomplète, sans parent commun ou en double.
- Le serveur écrit les domaines dans une balise meta d'`index.html` ; le front s'en sert pour changer de domaine quand une navigation quitte l'espace courant.
- Avec Coolify : renseigner les quatre variables et lister les quatre domaines sur le service `app` (voir le README).

## [0.9.0] — Portail client : tickets · 2026-10-03

Cette version fait passer le support client par Piilot plutôt que par la boîte mail : le client dépose sa demande dans son portail, suit la conversation avec l'agence, et chacun est prévenu par e-mail de ce que l'autre écrit.

### Points forts

- Nouvelle demande depuis le portail : projet, nature, description, priorité, captures d'écran.
- Une conversation par demande, comme une messagerie, avec les changements de statut.
- E-mails dans les deux sens : nouvelle demande et réponse du client pour l'agence, réponse et statut pour le client.
- Les notes internes de l'équipe ne sortent jamais du back-office.

### Nouveautés

#### Client

- **Support** : les demandes en cours et les demandes résolues ou fermées, chacune avec son statut et sa dernière mise à jour.
- **Nouvelle demande** : le projet concerné, la nature (un problème, une évolution, une question), un titre, une description guidée, la priorité (basse, normale ou haute) et jusqu'à dix pièces jointes.
- **Conversation** : les messages de l'agence et du client en fil, les changements de statut en mots simples (reçue, en cours de traitement, prête à être mise en ligne, résolue, fermée), la réponse et l'ajout de fichiers depuis la même page.
- **E-mails** : une réponse publique de l'agence ou un changement de statut part par e-mail à la personne qui a ouvert la demande, avec le lien direct.

#### Admin et Team

- **Demandes du portail** : un ticket ouvert par un client porte la pastille « Visible du client », montre ses pièces jointes, et le rédacteur rappelle si une réponse part chez le client.
- **E-mails** : une nouvelle demande ou une réponse du client part par e-mail à la personne qui traite le ticket, ou aux administrateurs tant que personne ne l'a pris. Les notifications dans l'application suivent comme pour les autres tickets.

### Technique

- Routes `/api/v1/client/tickets*` réservées au rôle `client`, isolées par le client de l'appelant. La requête des messages écarte elle-même les notes internes ; seuls les changements de statut sont exposés, en regroupant les étapes internes.
- Les urgences et criticités se décident côté agence : le portail n'accepte que les priorités basse, normale et haute.
- Migration `000034` : `tickets.client_visible` (les tickets déjà ouverts par un compte client le deviennent), `attachments.ticket_id`.
- Tests d'isolation dans la suite d'intégration : notes internes absentes du fil et des e-mails, tickets internes et tickets d'un autre client introuvables, pièces jointes protégées.

## [0.8.0] — Portail client : suivi · 2026-10-03

Cette version ouvre le portail aux clients de l'agence : ils suivent leurs projets, consultent les documents partagés et valident un livrable — ou disent ce qui doit changer — sans e-mail ni appel, y compris depuis leur téléphone.

### Points forts

- Mes projets : avancement, prochaine étape et livrables à valider, d'un coup d'œil.
- Validation des livrables dans le portail, avec retours écrits et historique des versions.
- Documents partagés fichier par fichier : un fichier de projet reste interne par défaut.
- Une API du portail isolée par client dans chaque requête SQL, sous tests d'isolation.

### Nouveautés

#### Client

- **Mes projets** : chaque projet avec son statut, son avancement, sa prochaine étape et sa dernière activité. Un bandeau en tête rassemble les livrables qui attendent une réponse.
- **Projet** : les livrables à valider d'abord, puis les étapes du projet, les autres livrables et les documents partagés à télécharger.
- **Livrable** : ouvrir la version, la valider d'un geste ou demander des retours en écrivant ce qui doit changer. Les versions précédentes restent lisibles, avec leurs réponses.
- **E-mail à chaque version** : un nouveau livrable ou une nouvelle version à valider part par e-mail, avec le lien direct vers la page.
- **Nouvelle présentation du portail**, pensée pour le téléphone : une barre en haut et deux entrées, Mes projets et Support.

#### Admin et Team

- **Partager un fichier avec le client** : sur la fiche projet, chaque fichier porte une pastille « Interne » ou « Partagé », qui se bascule d'un clic.
- La réponse du client notifie l'équipe du projet (déjà en place depuis la 0.6.0) et s'inscrit au journal du client quand il valide.

### Corrections

- Une seconde réponse sur une version déjà tranchée était acceptée en silence et notifiait l'équipe une nouvelle fois : elle est maintenant refusée.

### Technique

- API `/api/v1/client/*`, réservée au rôle `client` : chaque requête part du compte de l'appelant et de son client. Un identifiant d'un autre client rend 404, comme un identifiant inconnu ; un brouillon et un fichier interne ne sortent jamais.
- Tests d'isolation dans la suite d'intégration (exécutée par la CI) et tests de garde sur les routes : un compte interne n'entre pas dans le portail, un compte client n'entre pas dans le back-office.
- Migration `000033` : colonne `attachments.shared_with_client`, index des projets par client pour le portail.

## [0.7.0] — Projets et CRM avancés · 2026-10-02

Cette version donne aux projets leurs étapes et à la relation client sa mémoire : des jalons reliés aux livrables, un planning qui les montre, des modèles pour démarrer un projet en un geste, et un journal par client qui se remplit en partie tout seul.

### Points forts

- Jalons : les étapes datées d'un projet, avec les livrables qui les tiennent.
- Planning : un calendrier du mois avec les jalons, les échéances et ses tâches.
- Modèles de projet : jalons, tâches et services posés dès la création.
- Journal client : notes, appels, rendez-vous et e-mails, plus les événements automatiques.

### Nouveautés

#### Admin et Team

- **Jalons** (onglet du projet) : une frise des étapes, chacune avec son échéance, sa description et ses livrables. Le nombre de livrables validés se lit sur chaque jalon ; on le coche quand il est atteint, il passe « en retard » quand son échéance est dépassée. Un livrable se rattache à un jalon à son dépôt ou depuis la frise, et la liste des livrables affiche son jalon.
- **Planning** : le mois en calendrier, avec les jalons, les échéances des projets et ses propres tâches. Bascule entre ses projets et toute l'agence (l'équipe démarre sur ses projets). Chaque entrée mène au projet, à ses jalons ou à la tâche.
- **Modèles de projet** (Paramètres) : un nom, des services, des jalons et des tâches dont les échéances se comptent en jours depuis le début du projet. Tout s'édite sur une page et s'enregistre d'un bloc.
- **Partir d'un modèle** : à la création d'un projet, le modèle choisi lui donne ses jalons, ses tâches et ses services.
- **Journal client** (fiche client et écran Interactions) : notes, appels, rendez-vous et e-mails saisis à la main, datés après coup si besoin ; projet créé, livrable validé et ticket ouvert s'y inscrivent seuls. Filtres par client et par source, chargement des entrées plus anciennes.

### Technique

- Migration `000032` : tables `milestones`, `project_templates` (et leurs jalons, tâches et services), `client_interactions`, colonne `deliverables.milestone_id`. Les compteurs de livrables des jalons sont tenus par déclencheur.
- Les événements du journal s'écrivent dans la transaction du geste qui les provoque.
- Planning borné à 62 jours par requête, en une seule requête SQL.
- Nouveaux endpoints : jalons, `PUT /deliverables/:id/milestone`, `GET /planning`, modèles de projet, interactions CRM.

## [0.6.0] — Espace team · 2026-10-02

Cette version donne à l'équipe un espace à elle : une page qui dit par quoi commencer la journée, une feuille de temps à la semaine, un menu débarrassé des outils de direction, et des notifications qui suivent aussi les tickets et les livrables.

### Points forts

- « Mon travail » : tâches en retard, tickets confiés, livrables à reprendre et projets en cours, sur une seule page.
- Feuille de temps à la semaine, qu'on remplit d'une case à l'autre.
- Le menu ne montre que ce que les droits du compte permettent.
- Notifications pour les tickets (confié, réponse, statut) et les livrables (validation, retours).

### Nouveautés

#### Team

- **Mon travail** : l'accueil de l'équipe. Les tâches assignées, les retards d'abord ; les tickets ouverts confiés, les plus urgents d'abord ; les livrables à déposer ou à reprendre après des retours, avec l'extrait du retour ; les projets en cours avec leur avancement et leur échéance ; le temps saisi depuis lundi.
- **Feuille de la semaine** (Suivi du temps) : une grille projet et tâche par jour. Une case vide ou à une seule saisie se modifie sur place (90, 1h30 ou 1,5, puis Entrée ou Tab) ; la vider retire la saisie. Une case qui en regroupe plusieurs mène à sa journée. « Ajouter une ligne » ouvre un projet qui n'a encore rien cette semaine.
- **Menu selon les droits** : les entrées, onglets et filtres qu'un compte ne peut pas ouvrir disparaissent, et une adresse saisie à la main ramène vers un écran permis. Un membre de l'équipe arrive sur « Mon travail » plutôt que sur le tableau de bord.

#### Tous les espaces internes

- **Notifications des tickets** : ticket ouvert ou confié, réponse, changement de statut. Elles mènent à la fiche du ticket.
- **Notifications des livrables** : validation ou retours du client, pour l'équipe du projet et la personne qui a déposé la version. Elles mènent aux livrables du projet.

#### Admin

- **Rôles** : un groupe « Pilotage » avec le tableau de bord de l'agence et les budgets ; le pipeline commercial rejoint le CRM.

### Améliorations

- Les notifications des tâches arrivent de nouveau en temps réel : leur diffusion n'était jamais branchée.
- La saisie du temps suit enfin la permission « Saisir du temps passé », que l'écran des rôles proposait sans effet.

### Technique

- Trois permissions : `dashboard.read`, `budgets.read`, `pipeline.read`, accordées au rôle `admin`. `time.read` devient la lecture des rapports de toute l'équipe. Le rôle `team` perd `time.read` et `roles.read` à la migration.
- Sans `budgets.read`, l'API retire les heures vendues et l'état du budget des projets, ignore le filtre et le tri sur le budget, et refuse de fixer des heures vendues.
- `GET /api/v1/admin/me/work` : la page « Mon travail » en un appel, cinq requêtes bornées.
- Les notifications portent désormais `ticket_id` et `deliverable_id`.

## [0.5.0] — Gestion des comptes · 2026-10-02

Cette version met fin à la création des comptes en ligne de commande : les administrateurs invitent, gèrent et désactivent les comptes depuis Piilot, et règlent ce que chaque rôle permet. Chacun peut aussi retrouver l'accès à son compte après un mot de passe oublié.

### Points forts

- Invitation par e-mail : la personne choisit son mot de passe et arrive connectée dans son espace.
- Mot de passe oublié, avec un lien à usage unique valable une heure.
- Écran des rôles : les permissions de chaque rôle se règlent en quelques cases.
- Un compte désactivé ou un rôle modifié prend effet immédiatement, sans attendre la reconnexion.

### Nouveautés

#### Admin

- **Comptes et rôles** (Paramètres) : la liste des comptes de l'agence et du portail, avec leur rôle, leur client et leur dernière connexion, filtrable par rôle et par état.
- **Inviter** : une adresse, un rôle (administrateur, équipe ou client, avec son client) et, si l'on veut, un nom. Le lien d'invitation, valable sept jours, part par e-mail et s'affiche aussi pour être copié.
- **Invitations** : les invitations en attente ou expirées, à renvoyer avec un nouveau lien ou à annuler.
- **Gérer un compte** : passer administrateur ou en équipe, désactiver (le compte est déconnecté de tous ses appareils et ne peut plus se connecter), réactiver, ou créer un lien de réinitialisation du mot de passe.
- **Rôles** : la grille des permissions par rôle. Le rôle administrateur garde tout ; gérer les comptes, les rôles et les mises à jour reste réservé aux administrateurs ; le portail ne reçoit que les permissions du portail.

#### Tous les espaces

- **Activation du compte** : la page de l'invitation dit qui invite et pour quel rôle, puis demande le nom et un mot de passe de douze caractères au moins, avec une jauge pendant la saisie.
- **Mot de passe oublié** : depuis la page de connexion, avec l'adresse déjà saisie. Le changement de mot de passe ferme toutes les sessions du compte et ramène à la connexion, adresse pré-remplie.
- **Compte désactivé** : la connexion l'explique au lieu de répondre « identifiants incorrects ».

### Améliorations

- Les libellés des permissions et du rôle « Équipe » prennent leurs accents.

### Technique

- E-mails par n'importe quel serveur SMTP (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`, `SMTP_SECURITY`), via une file d'attente en base et une tâche de fond, avec de nouvelles tentatives espacées. Les corps des e-mails sont effacés une fois partis : aucun lien valable ne reste en base.
- Jetons d'invitation et de réinitialisation à usage unique, stockés hachés. Pages publiques bornées par adresse IP ; « mot de passe oublié » répond de la même façon que l'adresse existe ou non, et plafonne les demandes par compte.
- La garde relit à chaque requête l'état et le rôle du compte en base : une désactivation ou un changement de rôle s'applique à la requête suivante.
- Mailpit dans `docker-compose.dev.yml` pour lire les e-mails en développement.
- Routes `/api/v1/admin/accounts`, `/api/v1/admin/roles` et, sans session, `/api/v1/auth/invitations/{token}` et `/api/v1/auth/password/…`.

### À savoir pour le déploiement

- Migration `000030_accounts`, appliquée au démarrage : désactivation des comptes, invitations, liens de réinitialisation et file d'envoi des e-mails.
- Pour envoyer les e-mails, renseigner `SMTP_HOST` et `SMTP_FROM` (et les identifiants du fournisseur). Sans eux, tout fonctionne, mais les liens se transmettent à la main depuis l'écran des comptes.
- `PUBLIC_BASE_URL` doit être l'adresse publique de l'application : c'est elle qui figure dans les liens des e-mails.

## [0.4.2] — Mise à jour depuis l'interface · 2026-10-02

Cette version permet aux administrateurs de mettre Piilot à jour depuis l'application, sans passer par le serveur ni par un outil de déploiement. Elle change aussi la façon d'installer Piilot : l'image publiée est désormais tirée telle quelle au lieu d'être reconstruite sur le serveur.

### Points forts

- Bouton **Mettre à jour** pour les administrateurs, dès qu'une nouvelle version est publiée.
- Mise à jour en une à deux minutes, avec retour automatique à la version précédente si la nouvelle ne démarre pas.
- Bandeau proposant à tous de recharger la page quand une nouvelle version est en ligne.

### Nouveautés

#### Admin

- **Mise à jour depuis l'interface** : quand une version plus récente existe, un bouton apparaît dans l'en-tête. Il présente les notes de version et les précautions à prendre, puis lance la mise à jour après confirmation. Son avancement s'affiche étape par étape. Réservé à la nouvelle permission `system.update`, accordée au seul rôle admin.

#### Tous les espaces

- **Nouvelle version en ligne** : un onglet resté ouvert pendant un déploiement affiche un bandeau qui propose de recharger la page. Il évite aussi les écrans qui ne se chargent plus après un déploiement.

### Technique

- Nouveau service **`updater`** dans `docker-compose.yml`, le seul conteneur à recevoir le socket Docker. Il n'ouvre aucun port et ne reçoit ses ordres que par la base : il tire la nouvelle image, recrée le conteneur de l'application à l'identique et revient à l'ancien si le nouveau ne répond pas à sa sonde de santé. Il ne dépend d'aucun outil de déploiement.
- `docker-compose.yml` tire l'image `ghcr.io/plugiit/piilot-app` (`PIILOT_TAG`, `latest` par défaut) au lieu de la construire ; `docker-compose.build.yml` construit depuis les sources.
- Vérification des nouvelles versions auprès des releases GitHub, toutes les 6 heures, en tâche de fond (`UPDATE_CHECK`, actif par défaut).
- Routes `GET` et `POST /api/v1/admin/system/update`.

### À savoir pour le déploiement

- Migration `000029_app_updates`, appliquée au démarrage : crée les tables de suivi des versions et des mises à jour, et la permission `system.update`.
- **Redéployer une fois** avec le nouveau `docker-compose.yml` : il ajoute le service `updater`, qui monte `/var/run/docker.sock`, et tire l'image au lieu de la construire. Les mises à jour suivantes pourront passer par le bouton.
- Nouvelles variables facultatives : `PIILOT_TAG` (`latest` par défaut ; une version figée désactive la mise à jour depuis l'interface) et `UPDATE_CHECK` (`true` par défaut).
- **Faire une sauvegarde avant chaque mise à jour** : le retour automatique remet l'ancienne version de l'application, pas l'ancien schéma de la base.

## [0.4.1] — Image arm64 · 2026-10-02

Cette version publie l'image Docker de Piilot pour les processeurs ARM, en plus des processeurs x86. Elle s'adresse à ceux qui hébergent Piilot sur un serveur ARM ou le font tourner sur un Mac Apple Silicon. L'application elle-même ne change pas.

### Technique

- L'image `ghcr.io/plugiit/piilot-app` est publiée pour `linux/amd64` et `linux/arm64`. Docker choisit seul la bonne architecture au téléchargement.
- Le front, la compilation et les tests ne tournent qu'une fois par build, sur la machine de build ; Go compile directement pour chaque architecture. Seule l'image finale est construite par émulation, ce qui garde un temps de build proche de celui d'une seule architecture.
- La CI construit désormais les deux architectures, pour qu'une image arm64 cassée se voie avant la release.

### À savoir pour le déploiement

- Aucune migration, aucune nouvelle variable d'environnement.

## [0.4.0] — Temps et budgets · 2026-10-02

Cette version termine le pilotage du temps : chaque projet affiche où il en est de son budget, et le tableau de bord ne montre plus que des données réelles. Elle s'adresse aux administrateurs qui suivent la rentabilité des projets et la charge de l'équipe. C'est aussi la première version publiée sous licence libre.

### Points forts

- Budget de chaque projet : heures consommées sur heures vendues, avec une alerte à 80 % et au-delà de 100 %.
- Liste des projets filtrable et triable par budget, pour retrouver d'un coup ceux qui dérivent.
- Tableau de bord entièrement réel : activité de l'année, charge par projet, temps de l'équipe du jour et de la semaine.
- Piilot est distribué sous licence AGPL-3.0, et son image Docker est publiée sur GitHub.

### Nouveautés

#### Admin

- **Budget sur la carte projet** : heures consommées sur heures vendues, à côté de l'échéance. La mention passe en orange « À surveiller » à partir de 80 % du budget consommé, et en rouge « Hors budget » au-delà. Les projets internes et ceux sans heures vendues n'ont pas d'alerte.
- **Budget sur la fiche projet** : une barre sous l'avancement, avec les heures restantes ou dépassées. Un projet sans heures vendues propose un lien vers la page où les saisir.
- **Filtre « Budget »** dans la liste des projets (« À surveiller », « Hors budget ») et tri par part du budget consommée.
- **Charge par projet** (tableau de bord) : les huit projets en cours les plus avancés dans leur budget, avec la part consommée et le dépassement, et le nombre de projets hors budget ou à surveiller. Un clic sur une barre ouvre le projet.
- **Équipe aujourd'hui** (tableau de bord) : pour chaque membre, le temps saisi dans la journée et depuis lundi, la part facturable du jour et le dernier projet sur lequel il a pointé.
- **Activité par jour** (tableau de bord) : une case par jour de l'année, selon le nombre de tâches terminées, de tickets ouverts et de versions de livrables déposées.

### Améliorations

- La page Budget d'un projet recalcule l'alerte pendant la saisie des heures vendues, avec le même seuil que le reste de l'application.
- L'agenda du tableau de bord, qui n'affichait que des événements d'exemple, est retiré. Il reviendra avec le planning.
- **Glisser-déposer animé** dans les kanbans des tâches et le pipeline CRM : la colonne survolée s'allonge pour faire place à la carte, et les cartes voisines glissent au lieu de sauter.
- Chaque colonne de kanban affiche son total réel, et indique quand elle n'est pas affichée en entier (« 50 affichées sur 538, les plus récentes »).

### Corrections

- L'écran **Tâches** n'affichait que des tâches terminées dès que l'agence dépassait 300 tâches : les colonnes « À faire », « En cours » et « En revue » restaient vides, et une carte qu'on y déposait disparaissait. Chaque colonne du kanban est désormais plafonnée séparément (50 cartes, les terminées les plus récentes d'abord), et la vue liste est paginée.

### Technique

- Licence AGPL-3.0, guide de contribution, politique de sécurité et modèles d'issue et de pull request.
- Chaque version publie l'image sur `ghcr.io/plugiit/piilot-app`, avec les tags `X.Y.Z`, `X.Y` et `latest`.
- L'activité quotidienne est précalculée par déclencheur dans une table dédiée : le tableau de bord la lit sans rien recompter.
- `GET /api/v1/admin/tasks` est paginé (`page`, `page_size`) ; nouvelle route `GET /api/v1/admin/tasks/board` pour le kanban de l'écran Tâches. Le tableau d'un projet (`GET /api/v1/admin/projects/{id}/tasks`) renvoie le total de chaque colonne à la place de `total` et `limit`, avec 100 cartes au plus par colonne.
- Paramètre `budget` (`warning`, `over`) sur `GET /api/v1/admin/projects`, champ `budget_state` sur les projets, et nouveaux blocs dans `GET /api/v1/admin/dashboard`.
- Nom technique aligné sur Piilot : cookies de session, émetteur des jetons et base de développement.

### À savoir pour le déploiement

- Migration `000028_daily_activity`, appliquée au démarrage : crée la table d'activité quotidienne et la remplit à partir des tâches terminées, des tickets et des versions de livrables existants.
- Les cookies de session changent de nom : **tout le monde est déconnecté une fois** après la mise à jour.
- Aucune nouvelle variable d'environnement.

## [0.3.2] — Rapports de temps · 2026-09-19

Cette version ouvre l'écran des rapports de temps : le temps de toute l'équipe sur une période, réparti entre facturable et non facturable, filtrable et exportable. Elle s'adresse aux administrateurs qui bouclent le mois ou suivent la charge d'un projet.

### Points forts

- Rapport sur une période au choix, avec des raccourcis (semaine, mois, trimestre, année, 12 derniers mois) ou une plage libre d'un an au plus.
- Répartition du temps par projet, personne, service ou client ; un clic sur une ligne filtre tout l'écran.
- Export CSV prêt pour Excel.

### Nouveautés

#### Admin

- **Rapports de temps** (Suivi du temps → Rapports) : filtres par projet, personne, service, client et facturation, combinables. L'état de l'écran est conservé dans l'adresse : un rapport se partage par son lien.
- **Chiffres clés** : temps saisi et nombre de saisies, temps facturable et non facturable avec leur part du total, nombre de personnes et de projets concernés.
- **Évolution** : barres empilées facturable / non facturable, par jour, semaine ou mois selon la longueur de la période. Les jours sans saisie restent visibles.
- **Répartition** : tableau par projet (avec son client), personne, service ou client, trié du plus gros poste au plus petit.
- **Détail des saisies** : date, personne, projet et client, tâche, service, note et durée, avec la mention « Interne » pour les projets internes.
- **Export CSV** des saisies filtrées : séparateur point-virgule, accents conservés, durées en heures décimales à la virgule.

### Technique

- Trois routes sous `/api/v1/admin/time-reports` (rapport, détail, export), protégées par la permission `time.read`. Le rôle `team` la possède également.
- Les cellules du CSV qui commencent par `=`, `+`, `-` ou `@` sont écrites comme du texte, pour qu'un tableur ne les exécute pas comme des formules.
- Les totaux du rapport sont calculés à la lecture, sur une période bornée à un an : des filtres choisis à la volée ne peuvent pas être précalculés.

### À savoir pour le déploiement

- Aucune migration, aucune nouvelle variable d'environnement.

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

