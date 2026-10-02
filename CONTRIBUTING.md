# Contribuer à Piilot

Merci de l'intérêt porté au projet. Piilot est l'outil de pilotage de
l'agence Plugiit, publié en open source. Il reste **l'outil d'une seule
agence** : une contribution est la bienvenue si elle sert cet usage, et sera
refusée si elle l'élargit vers autre chose (voir [Périmètre](#périmètre)).

Ce document décrit comment proposer un changement et les conventions que le
code suit. L'installation et l'architecture sont dans le [README](README.md),
la feuille de route dans [ROADMAP.md](ROADMAP.md).

---

- [Avant de commencer](#avant-de-commencer)
- [Périmètre](#périmètre)
- [Signaler un bug, proposer une idée](#signaler-un-bug-proposer-une-idée)
- [Signaler une faille de sécurité](#signaler-une-faille-de-sécurité)
- [Proposer un changement](#proposer-un-changement)
- [Commits](#commits)
- [Principes d'architecture](#principes-darchitecture)
- [Conventions de code](#conventions-de-code)
- [Base de données](#base-de-données)
- [Sécurité](#sécurité)
- [Licence des contributions](#licence-des-contributions)

---

## Avant de commencer

- **Langue** : le projet est en français. Issues, pull requests, commits,
  commentaires, documentation et textes de l'interface s'écrivent en
  français. Les identifiants de code (variables, fonctions, routes, colonnes)
  restent en anglais.
- **Ouvrir une issue avant tout changement important** : nouvel écran,
  nouvelle table, nouvelle dépendance, changement de contrat d'API. Un
  désaccord sur le fond se règle mieux avant d'écrire le code qu'après.
- Les corrections de bugs, de fautes ou de documentation peuvent arriver
  directement en pull request.

## Périmètre

Piilot couvre la gestion de projet, le CRM et le portail client d'une agence.
Sont **hors périmètre, par choix**, et les pull requests qui les ajoutent
seront fermées :

- facturation et comptabilité ;
- monitoring SEO ;
- CMS et blog ;
- module RH ;
- multi-agence ou multi-tenant : Piilot n'est pas un SaaS ;
- application mobile.

L'ordre des fonctionnalités suit la [roadmap](ROADMAP.md). Une proposition
qui s'inscrit dans une version à venir a plus de chances d'être retenue
qu'une proposition qui ouvre un nouveau chantier.

## Signaler un bug, proposer une idée

Par les [issues GitHub](https://github.com/Plugiit/piilot-app/issues). Pour un
bug, indiquer :

- la version (fichier `VERSION`, ou `/health/live` sur une instance) ;
- les étapes pour le reproduire ;
- le résultat attendu et le résultat obtenu ;
- le `request_id` de la réponse en erreur s'il y en a un : c'est lui qui
  relie l'incident à sa trace dans les logs.

Ne jamais coller de données réelles (noms de clients, adresses, contenus de
tickets) ni de secret dans une issue.

## Signaler une faille de sécurité

**Pas d'issue publique.** Utiliser le signalement privé de GitHub : onglet
*Security* du dépôt, puis *Report a vulnerability*. Le signalement n'est
visible que des mainteneurs. Décrire la faille, son impact et la façon de la
reproduire ; la correction est publiée avant toute divulgation.

## Proposer un changement

1. Forker le dépôt et créer une branche depuis `master`, nommée d'après le
   changement : `feat/rapports-export`, `fix/kanban-echeance`.
2. Installer l'environnement de développement (voir « Développement » dans
   le [README](README.md#développement)) : Go 1.26, Node 24, Docker.
3. Écrire le changement **et ses tests**.
4. Lancer `make check` : gofmt, vet, tests Go, routes, lint, types et tests
   du front. Une pull request qui ne passe pas `make check` ne sera pas
   relue.
5. Ouvrir la pull request vers `master` en expliquant **pourquoi** le
   changement est nécessaire, pas seulement ce qu'il fait. Ajouter une
   capture pour tout changement visible.

La CI reconstruit l'image de production, ce qui rejoue toutes les
vérifications, et lance les tests d'intégration contre un vrai Postgres. Elle
doit être verte avant la fusion.

Une pull request = un sujet. Un correctif de bug ne s'accompagne pas d'un
remaniement sans rapport : il se propose à part.

Ne pas modifier `VERSION`, `CHANGELOG.md` ni le statut des releases dans
`ROADMAP.md` : ils sont tenus par le script de publication.

## Commits

Format [Conventional Commits](https://www.conventionalcommits.org/fr/), en
français, à l'impératif présent, sans majuscule ni point final :

```
type(portée): description courte
```

```
feat(temps): rapports de temps de l'équipe, avec export CSV
fix(login): logo au-dessus du formulaire et en favicon
migration(db): table des livrables et de leurs versions
```

| Type | Usage |
|---|---|
| `feat` | Nouvelle fonctionnalité |
| `fix` | Correction de bug |
| `refactor` | Réorganisation sans changement de comportement |
| `migration` | Ajout d'une migration de base |
| `test` | Ajout ou correction de tests |
| `docs` | Documentation seule |
| `build` | Image, CI, outillage de build |
| `chore` | Code généré, dépendances, maintenance |

La portée désigne le module (`crm`, `temps`, `tickets`) ou la couche
(`api`, `ui`, `db`) touchés. Un commit compile et passe les tests : pas de
commit « wip » dans une pull request prête à relire.

## Principes d'architecture

Ces règles sont la raison d'être du projet. Une pull request qui en enfreint
une sera refusée, quelle que soit la qualité du reste.

- **Un endpoint par écran, jamais par entité.** Chaque route renvoie
  exactement ce que l'écran affiche, en un seul appel.
- **Toute liste est paginée côté serveur.** Aucune requête sans `LIMIT`.
- **Aucun appel externe pendant une requête HTTP.** Une tâche de fond remplit
  une table locale, l'endpoint lit la table.
- **Les agrégats sont précalculés**, jamais recalculés par `COUNT(*)` à
  l'affichage.
- **Le contrat OpenAPI fait foi.** Toute route ajoutée ou modifiée l'est
  d'abord dans `api/openapi/` ; le front régénère ses types avec
  `cd web && npm run api:types`. Les types générés ne se modifient pas à la
  main.
- **Pas de dépendance sans justification**, écrite dans la pull request :
  ce qu'elle apporte, pourquoi la bibliothèque standard ou le code existant
  ne suffisent pas.
- **Du code lisible plutôt que du code malin.** Le code se lit plus souvent
  qu'il ne s'écrit.

## Conventions de code

Un commentaire explique **pourquoi**, jamais **quoi**. Si le quoi n'est pas
clair, c'est le code qu'il faut changer.

### API (Go, `api/`)

- `gofmt` et `go vet` propres.
- Un package, une responsabilité. `domain` ne dépend ni du HTTP ni du SQL.
- Un handler fait trois choses : valider l'entrée, appeler un usecase,
  sérialiser la sortie. Aucune logique métier dedans.
- Erreurs enveloppées avec leur contexte :
  `fmt.Errorf("lecture du projet : %w", err)`. Jamais d'erreur nue.
- Les erreurs métier sont des `*domain.Error` avec un `Code` stable : le
  front branche sur le code, jamais sur le message.
- Aucun détail technique dans une réponse HTTP : la cause part dans les
  logs, le client reçoit un message générique.
- Pas de variable globale mutable. Seul le package `config` lit
  l'environnement ; la configuration est injectée.
- Le contexte de la requête est propagé jusqu'aux appels à la base.
- Le SQL s'écrit à la main dans `api/queries/`, puis `make -C api sqlc`
  régénère le code Go. Pas d'ORM.

### Front (React, `web/`)

- TypeScript strict, avec `noUncheckedIndexedAccess`. Pas de `any` sans
  commentaire qui le justifie.
- Composants fonctionnels uniquement, un fichier par composant, nommé en
  `kebab-case`.
- `src/features/<module>/` porte les `queryOptions` et les composants d'un
  module ; `src/components/ui/` ne contient que des primitives shadcn/ui
  sans logique métier.
- Aucune couleur en dur : tout passe par les tokens de `src/index.css`.
- L'état d'un écran (filtres, tri, pagination, onglet) vit dans l'URL via
  `validateSearch`, pas dans un `useState` : la vue se partage par son lien
  et le bouton Retour fonctionne. La clé de cache TanStack Query en dérive.
- Toute liste de plus de 100 lignes est virtualisée.
- Le back-office occupe la fenêtre : la page ne défile pas, chaque zone de
  contenu gère son propre défilement.

### Contrat d'API

- Versionnement dans le chemin : `/api/v1/...`.
- Chemins en `kebab-case`, collections au pluriel. Champs JSON en
  `snake_case`.
- Collections paginées : `{items, total, page, page_size}`.
- Erreurs au format unique `{code, message, details}` ; `details` est
  toujours un objet, jamais `null`.
- Codes HTTP : 200 lecture, 201 création, 204 suppression, 401 non
  authentifié, 403 interdit, 404 introuvable, 409 conflit, 422 validation,
  429 quota, 500 erreur serveur.

### Tests

- Noms en français et descriptifs : `TestVerifyRejetteUnJetonExpire`.
- Tables de cas quand plusieurs cas partagent la même mécanique.
- Les tests d'intégration (tag `integration`) exigent une vraie base : sans
  `TEST_DATABASE_URL`, ils **échouent**, ils ne sont jamais ignorés en
  silence. `make -C api test-integration` les lance contre la base locale.

## Base de données

- Tables au pluriel, colonnes en `snake_case`.
- Clés primaires `uuid`, `created_at` et `updated_at` en
  `timestamptz NOT NULL DEFAULT now()`.
- Suppression logique par `deleted_at` sur les entités importantes, avec une
  unicité partielle sur les lignes vivantes
  (`CREATE UNIQUE INDEX ... WHERE deleted_at IS NULL`).
- Un index sur chaque colonne qui sert à filtrer ou trier une liste.
- Une migration se crée avec `make -C api migrate-new name=...` et va par
  paire `up` / `down`.
- **Une migration publiée dans une release ne se modifie jamais.** Toute
  correction passe par une nouvelle migration : des instances tournent déjà
  avec l'ancienne.

## Sécurité

Règles absolues, vérifiées à la relecture :

- **Aucun secret dans le dépôt.** `.env` est ignoré ; les `.env.example` ne
  contiennent que des noms et des valeurs factices.
- **Aucune donnée réelle** dans le code, les tests, les fixtures ou les
  captures : uniquement des données fictives.
- Jeton de session en cookie `httpOnly`, `Secure`, `SameSite=Lax`, jamais en
  `localStorage`.
- Algorithme de signature JWT imposé explicitement à la vérification.
- Rôles et permissions vérifiés côté serveur sur chaque endpoint. Le front
  masque des boutons, il ne protège rien.
- `ADMIN_ORIGINS` liste des origines explicites ; jamais de joker avec
  `AllowCredentials`.
- Le portail client est exposé à l'extérieur : tout endpoint qu'il consomme
  filtre sur le client de l'appelant, sans exception.
- Jamais de donnée personnelle ni de secret dans un log. Seuls les 5xx
  partent en `ERROR` ; un 4xx n'est pas une erreur serveur.

## Licence des contributions

Piilot est distribué sous licence [GNU AGPL v3](LICENSE). En proposant une
contribution, vous acceptez qu'elle soit publiée sous cette même licence, et
vous garantissez en avoir le droit : le code est le vôtre, ou sa licence
d'origine est compatible avec l'AGPL.
