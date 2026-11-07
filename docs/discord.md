# Bot Discord

Le bot du serveur Discord de l'agence relie un membre à son compte Piilot, et
lui donne le rôle *Client* ou *Équipe* tant que ce compte est actif. Piilot
n'appelle jamais Discord : le bot lui transmet ce que le membre a tapé, puis
relit un flux d'événements pour attribuer ou retirer les rôles.

Trois pièces, côté API :

- un **accès machine** : une clé, sous `/api/v1/bot` seulement ;
- la **liaison par code** : le compte tire un code, le membre le tape dans
  Discord (`/lier code:XXXX-XXXX`) ;
- un **flux d'événements**, écrit dans la transaction de chaque changement.

Le contrat fait foi dans `/openapi.json` (tag `bot`).

## Clés du bot

Elles se créent et se révoquent depuis le terminal du conteneur. Sous
Coolify : la ressource Piilot, onglet *Terminal*, conteneur `server`.

```sh
bot-key create discord-prod   # montrée une seule fois
bot-key list
bot-key revoke <id>
```

Hors Coolify : `docker compose exec server bot-key create discord-prod`.

Une clé commence par `piilot_bot_`, ce qui permet aux outils de détection de
secrets de la repérer. Seule son empreinte SHA-256 est stockée. Plusieurs clés
peuvent être actives : pour en changer sans coupure, créer la nouvelle, la
donner au bot, puis révoquer l'ancienne. Création et révocation passent au
journal d'audit.

La clé porte le rôle système `bot` et ses permissions (`discord.read`,
`discord.redeem`, `events.read`), vérifiées comme celles d'un compte. Ce rôle
ne s'attribue à aucun compte et n'apparaît pas dans l'écran des rôles.

**Cloisonnement.** Une clé n'est lue que sous `/api/v1/bot`. Ailleurs, elle
ne vaut rien. Une requête qui porte un cookie de session y est refusée, même
munie d'une clé valide.

## Ce que le bot reçoit

| Variable du bot | Valeur |
|---|---|
| URL de l'API | Adresse de Piilot sur le réseau interne de Coolify, sans `/api/v1` |
| Clé | La sortie de `bot-key create` |

Pour que le bot joigne Piilot sans passer par Internet, cocher *Connect to
Predefined Network* sur la ressource Piilot. Ses services deviennent alors
joignables sur le réseau `coolify` sous le nom `app-<uuid de la ressource>`,
port 8080. `app` est la passerelle : c'est elle qu'il faut appeler, pas
`server`, pour que les mises à jour de Piilot ne coupent pas le bot.

## Liaison par code

1. Le compte connecté tire un code : `POST /api/v1/auth/me/discord/code`.
   8 caractères en base32 de Crockford, affichés `XXXX-XXXX`, valables
   10 minutes, à usage unique. En tirer un invalide le précédent.
2. Le membre tape `/lier code:XXXX-XXXX` dans Discord.
3. Le bot appelle `POST /api/v1/bot/discord-links` avec le code et
   l'identifiant Discord du membre, en chaîne.

À la saisie, la casse et le tiret sont indifférents ; O vaut 0, I et L
valent 1. Le code est stocké sous HMAC, avec une clé dérivée de `JWT_SECRET`.

Un compte a une liaison au plus, un membre Discord est lié à un compte au
plus. Si le compte était déjà lié à un autre membre, la nouvelle liaison
remplace l'ancienne : la réponse donne l'ancien identifiant
(`replaced_discord_user_id`), pour que le bot lui retire ses rôles.

| Réponse | Cas |
|---|---|
| `201` | Comptes liés, ou déjà liés l'un à l'autre (rien ne change) |
| `404 LINK_CODE_INVALID` | Code inconnu, expiré, déjà utilisé ou remplacé, sans distinction |
| `409 DISCORD_ALREADY_LINKED` | Ce membre est lié à un autre compte actif. Le code est consommé |
| `422 VALIDATION_FAILED` | Code mal formé, ou identifiant qui n'est pas une chaîne de 17 à 20 chiffres |
| `429 RATE_LIMITED` | 5 essais par membre, 60 au total, par quart d'heure (`Retry-After`) |
| `503 SERVICE_UNAVAILABLE` | Redis injoignable : le rachat est refusé, il ne passe pas sans limite |

Un membre lié à un compte désactivé peut se relier à un autre compte : pour le
bot, l'ancienne liaison n'existait déjà plus.

Un rachat réussi ne remet pas le compteur du membre à zéro : sinon, un membre
muni de son propre code relancerait ses essais à volonté. Deux rachats
simultanés du même membre passent l'un après l'autre (verrou consultatif sur
son identifiant). Chaque rachat, réussi, raté ou limité, passe au journal
d'audit (`discord.linked`, `discord.redeem_failed`,
`discord.redeem_rate_limited`) avec la clé du bot qui a agi, jamais le code ni
l'identifiant Discord.

Le compte voit sa liaison (`GET /api/v1/auth/me/discord`) et peut la
supprimer (`DELETE`). La réponse dit aussi si le bot est branché
(`available` : au moins une clé active).

### Carte du compte

Dans le back-office, *Mon compte › Intégrations*, section « Votre compte »,
pour les comptes qui ont `discord.link` :

- **Bot non branché** : la carte le dit, sans proposer de code. Elle reste
  visible pour qu'un admin voie ce qui manque.
- **Non lié** : *Lier mon compte* tire un code, affiché avec un bouton de copie
  et son heure d'expiration. Tant qu'il est valable, la carte relit la liaison
  toutes les 4 secondes ; elle passe d'elle-même à « Lié » quand le bot l'a
  racheté.
- **Lié** : l'identifiant Discord et la date. *Changer* tire un code pour
  passer à un autre compte Discord, *Délier* demande confirmation.

Le portail client n'a pas encore d'écran de compte : un client a la
permission, mais pas encore la carte.

## Flux d'événements

`GET /api/v1/bot/events?after=<curseur>&limit=<1..100>` rend
`{items, next_after}`. Pagination par curseur et non par page : un flux n'a
pas de total, et une page numérotée glisserait à chaque écriture.

| Type | Données | Quand |
|---|---|---|
| `discord_link.upserted` | `discord_user_id`, `role` | Liaison créée ; rôle changé ou compte réactivé, s'il est lié |
| `discord_link.removed` | `discord_user_id` | Liaison supprimée ou remplacée ; compte désactivé ou supprimé |

Chaque événement est écrit dans la transaction du changement qu'il raconte :
un changement annulé n'émet rien. Les identifiants deviennent visibles dans
l'ordre : chaque émission incrémente un compteur dont la ligne reste
verrouillée jusqu'au commit. Avec une simple séquence, l'événement 11 pourrait
être validé avant le 10, et un lecteur déjà passé à 11 ne verrait jamais le 10.

Pour le bot :

- garder `next_after` après avoir appliqué une page, et le rendre tel quel.
  Il n'est jamais `null` : sur une page vide, c'est le curseur reçu ;
- ignorer les types inconnus. Les notifications du PM passeront par ce même
  flux ;
- appliquer chaque événement sans se soucier des doublons : ils sont
  idempotents.

Les événements sont conservés 30 jours. Un curseur plus ancien que la purge
reçoit `410 EVENTS_CURSOR_EXPIRED` : des événements manquent, il faut
réconcilier.

## Réconciliation

Au démarrage du bot, ou après un `410` :

1. `GET /api/v1/bot/discord-links`, première page : garder `events_cursor`.
2. Parcourir toutes les pages avec `after=<next_after>`, jusqu'à
   `next_after: null`. Appliquer les rôles de chaque liaison, retirer ceux des
   membres absents de la liste.
3. Reprendre le flux à partir de `events_cursor`.

Ce qui change pendant le parcours arrive par le flux. La liste est elle aussi
paginée par curseur, sur l'identifiant Discord : avec une page numérotée, une
liaison retirée pendant le parcours décalerait les suivantes, et l'une d'elles
serait sautée sans que le flux la rattrape. Seuls les comptes actifs y
figurent.

À l'arrivée d'un membre sur le serveur :
`GET /api/v1/bot/discord-links/{discord_user_id}` rend sa liaison, ou `404`.

## Données personnelles

L'identifiant Discord est une donnée personnelle.

- **Export** : `DiscordService.Export` rend la liaison d'un compte. À brancher
  sur l'export de compte de la 1.5.
- **Suppression** : `DiscordService.Forget` efface la liaison et le code dans
  la transaction de la suppression, et émet `discord_link.removed`. À brancher
  sur la suppression de compte de la 1.5. En attendant, la base efface la
  liaison avec le compte (`ON DELETE CASCADE`), mais sans prévenir le bot.
- **Purge**, toutes les heures : codes expirés, événements de plus de
  30 jours. Un événement garde donc l'identifiant Discord jusqu'à 30 jours
  après une déliaison : c'est ce qui permet au bot de retirer les rôles, et
  c'est la durée à inscrire au registre des traitements.
- **Journaux** : jamais de code, de clé ni d'identifiant Discord. Les erreurs
  journalisent le motif de la route (`/api/v1/bot/discord-links/:discord_user_id`),
  pas le chemin reçu ; la passerelle tronque les chemins qu'elle journalise.
  Redis compte les essais sous une empreinte HMAC de l'identifiant.
- **Journaux d'accès du proxy** : l'identifiant figure dans deux chemins
  (`/bot/discord-links/{id}`, `?after=`). Le bot appelle la passerelle sur le
  réseau interne, sans passer par Traefik : ces requêtes n'entrent pas dans
  ses journaux d'accès. Un bot qui passerait par l'adresse publique les y
  laisserait.
- **Ce que le bot apprend** : l'identifiant Discord, le code du rôle, la date
  de liaison. Ni nom, ni adresse. Le client du compte pourra s'y ajouter.
