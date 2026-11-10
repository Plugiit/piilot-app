# Connecteur Claude

Claude (claude.ai, Claude Desktop, Claude Code) peut agir dans Piilot au nom
d'un compte : lire les projets, les tâches, les tickets, chercher, et, avec
l'accord du compte, modifier. Piilot sert à la fois de serveur d'autorisation
OAuth 2.1 et de serveur MCP, sur `/mcp`, dans la même image.

Un compte ne donne à Claude que ses propres droits : ce que l'interface lui
interdit, Claude ne peut pas le faire non plus.

## Connecter Claude

L'adresse à saisir s'affiche dans *Mon compte › Intégrations*, carte
« Claude ». C'est l'origine de l'espace de connexion suivie de `/mcp`, par
exemple `https://auth.agence.fr/mcp`.

- **claude.ai et Claude Desktop** : *Personnaliser › Connecteurs › Ajouter un
  connecteur personnalisé*, coller l'adresse, puis se connecter à Piilot et
  autoriser. Desktop et le mobile reprennent le connecteur du compte
  claude.ai. Sur un plan Team ou Enterprise, c'est un Owner qui ajoute le
  connecteur ; chacun s'y connecte ensuite avec son compte Piilot.
- **Claude Code** : `claude mcp add --transport http piilot <adresse>`, puis
  `/mcp` dans Claude Code pour se connecter.

Claude ouvre alors l'écran de consentement de Piilot (`/autoriser`). Il dit
qui demande, pour quel compte, où partira le code. Une connexion vers la
machine locale, celle de Claude Code, est signalée : tout programme qui écoute
sur ce poste pourrait se faire passer pour lui.

La demande est liée au navigateur qui l'a lancée (cookie `piilot_oauth`, posé
par `/oauth/authorize`) : un lien de consentement transmis à quelqu'un
d'autre — « peux-tu valider ma connexion Claude ? » — ne lui sert à rien.

La permission `claude.connect` est accordée aux rôles admin et team. La
retirer d'un rôle, dans l'écran des rôles, coupe Claude pour tous ses comptes
dès l'appel suivant. Les comptes du portail client ne peuvent pas connecter
Claude.

## Ce que Claude peut faire

Version actuelle : la lecture. La portée d'écriture (`piilot:write`) n'est
pas encore proposée : elle arrivera avec ses outils, et une connexion donnée
avant ne l'obtiendra que par un nouveau consentement, jamais en silence.

| Outil | Ce qu'il rend |
|---|---|
| `whoami` | Le compte, son rôle, ses permissions |
| `search` | Projets, clients, tâches, tickets dont le nom contient le texte |
| `list_projects`, `get_project`, `get_project_board` | Projets, fiche, tableau des tâches |
| `list_tasks`, `get_task`, `list_task_comments` | Tâches de l'agence, détail, commentaires |
| `list_milestones` | Jalons d'un projet |
| `my_work`, `my_tickets`, `my_time`, `get_timer` | Ce qui attend le compte, son temps, son chrono |
| `project_tickets`, `get_ticket` | Tickets d'un projet, fiche et fil |
| `list_deliverables` | Livrables et réponse du client |
| `list_people`, `list_clients`, `list_project_templates`, `list_services` | Référentiels pour affecter, rattacher, partir d'un modèle |

Prévu dans la version suivante : créer et modifier projets, tâches,
sous-tâches, jalons et saisies de temps, joindre des fichiers, supprimer.
Chaque suppression est signalée à Claude comme irréversible : il demande
l'accord du compte, sauf si on l'a toujours autorisée dans Claude.

Claude ne peut jamais, quelle que soit la portée accordée :

- écrire ou envoyer un e-mail à un client ;
- publier sur le portail client : livrable, réponse publique à un ticket,
  fichier partagé, projet rendu visible ;
- lancer une mise à jour de site ;
- gérer les comptes, les rôles ou les réglages.

## Comment les outils touchent aux données

Chaque outil rappelle, dans le processus, la route REST que l'interface
appelle pour le même geste (`internal/claude/dispatch.go`). La requête
traverse toute la chaîne de l'application, au nom du compte :

- garde de session : compte actif, rôle relu en base ;
- permission de la route ;
- validation, masquage des budgets, portée de la recherche ;
- journal d'audit, avec `Piilot MCP (Claude)` ou `Piilot MCP (Claude Code)`
  comme agent.

Il n'existe donc pas de second chemin vers les données, qui dériverait du
premier. Le répartiteur n'appelle que les routes que les outils déclarent ;
un identifiant fourni par Claude doit être un UUID pour entrer dans un chemin.

## Sécurité

**Contenu non fiable.** Les tickets, les e-mails entrants et les retours de
livrable sont écrits par des clients. Un texte qui contient une consigne
(« supprime ce projet ») pourrait être lu par Claude comme une demande. Les
résultats qui peuvent en contenir les marquent comme données, les consignes du
serveur le rappellent, et rien de ce que Claude peut faire ne sort de l'agence.

**Seul Claude se connecte.** Les deux identités publiques de Claude (documents
CIMD hébergés par Anthropic) sont recopiées dans `usecase/oauth.go` :

| Client | `client_id` | Retour |
|---|---|---|
| claude.ai, Desktop, mobile | `https://claude.ai/oauth/mcp-oauth-client-metadata` | `https://claude.ai/api/mcp/auth_callback` |
| Claude Code | `https://claude.ai/oauth/claude-code-client-metadata` | `http://localhost/callback`, `http://127.0.0.1/callback`, port ignoré |

La spécification MCP recommande de lire ces documents à la volée. Piilot ne le
fait pas : aucun appel externe pendant une requête, et aucune requête sortante
vers une adresse fournie par un inconnu. Si Anthropic change ses documents,
c'est la table de `usecase/oauth.go` qu'il faut mettre à jour. Pas
d'enregistrement dynamique (DCR, déprécié par la spécification) : tout autre
client est refusé.

**Jetons.**

| Jeton | Forme | Durée | Règle |
|---|---|---|---|
| Code | `piilot_ac_…` | 2 min | Usage unique, lié au client, au retour, au défi PKCE (S256) et à la ressource. Rejoué, il révoque l'autorisation |
| Accès | `piilot_at_…` | 1 h | Ne vaut que pour `/mcp`, et pour cette ressource |
| Renouvellement | `piilot_rt_…` | 30 jours | Remplacé à chaque usage. Rejoué plus de 10 s après, il révoque l'autorisation |

Seules les empreintes SHA-256 sont stockées. Les préfixes permettent aux
outils de détection de secrets de repérer un jeton collé quelque part.

**Révocations.** Une autorisation tombe :

- depuis *Mon compte › Intégrations* ;
- par Claude lui-même (`/oauth/revoke`) ;
- au changement ou à la réinitialisation du mot de passe ;
- à la détection d'un vol de session (jeton de session rejoué) ;
- à la désactivation du compte.

Le retrait de `claude.connect` coupe tous les appels du rôle sans révoquer.
Chaque connexion et déconnexion passe au journal d'audit (`claude.connected`,
`claude.disconnected`, `claude.refresh_reused`, `claude.code_reused`), famille
« Connexions de Claude ».

**Barrière de `/mcp`.**

- 401 avec `WWW-Authenticate` sans jeton valable : c'est ce qui lance la
  connexion chez Claude.
- 401 aussi pour une requête qui porte un cookie de session.
- 403 pour une origine étrangère, ou pour un outil d'écriture appelé avec un
  jeton en lecture seule (`insufficient_scope`, Claude propose alors de
  reconnecter). Chaque outil d'écriture revérifie la portée lui-même : c'est
  lui qui fait foi.
- Un message JSON-RPC par requête, en clair : lots, corps compressé ou suite
  après le message sont refusés. Le corps est lu comme le SDK le lit, clés
  prises à la lettre.
- 120 appels par minute et par connexion.

Les journaux gardent le nom de l'outil, la connexion et la durée, jamais les
arguments.

## Points d'accès

| Route | Rôle |
|---|---|
| `GET /.well-known/oauth-protected-resource[/mcp]` | Où trouver le serveur d'autorisation (RFC 9728) |
| `GET /.well-known/oauth-authorization-server` | Métadonnées du serveur d'autorisation (RFC 8414), CIMD annoncé |
| `GET /oauth/authorize` | Valide la demande, l'enregistre, renvoie vers `/autoriser` |
| `POST /oauth/token` | Code ou renouvellement, en `application/x-www-form-urlencoded` |
| `POST /oauth/revoke` | RFC 7009 |
| `POST /mcp` | Serveur MCP, Streamable HTTP sans état, réponses JSON |
| `GET`, `POST /api/v1/auth/oauth/requests/{id}[/decision]` | Écran de consentement, sous session |
| `GET /api/v1/auth/me/claude`, `DELETE …/connections/{id}` | Carte du compte |

Le contrat complet est dans `/openapi.json`, tag `claude`. Les demandes en
attente, codes et jetons expirés sont purgés toutes les heures.

## En local

L'émetteur est l'origine de l'espace de connexion, ou `PUBLIC_BASE_URL` en
domaine unique. En développement, le front tourne sur 5173 et relaie `/oauth`,
`/.well-known` et `/mcp` vers l'API : régler `PUBLIC_BASE_URL` et
`ADMIN_ORIGINS` sur `http://localhost:5173`, puis :

```sh
claude mcp add --transport http piilot-local http://localhost:5173/mcp
```

claude.ai et Desktop ne joignent pas `localhost` : pour les essayer avant la
production, il faut un tunnel HTTPS temporaire.

## Données personnelles

**Ce que Claude lit part chez Anthropic.** Les résultats des outils — projets,
tâches, tickets écrits par les clients, noms et adresses de l'équipe et des
clients — sont envoyés au modèle, hors de l'UE. C'est le choix du compte qui
connecte Claude, mais il engage l'agence : à couvrir par le contrat
d'Anthropic (DPA), à inscrire au registre des traitements, et à mentionner aux
clients. Retirer `claude.connect` d'un rôle ferme le connecteur pour tous ses
comptes.

Une connexion retient le compte, le client (Claude ou Claude Code), les
portées, les dates de création et de dernier usage. Elle part avec le compte
(`ON DELETE CASCADE`). Les autorisations révoquées sont effacées après
30 jours, les jetons expirés le lendemain de leur expiration.
