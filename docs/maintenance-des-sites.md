# Maintenance des sites

Piilot ne touche jamais un site. Il **commande** et il **surveille** : la CI du
dépôt fait le travail, le site signale ses sauvegardes, et le silence d'une
source déclenche une alerte comme le ferait un échec.

Tout se règle dans *Projet › Paramètres › Maintenance*.

Un échec de mise à jour, des sauvegardes muettes ou un domaine qui expire arrivent
aux admins dans la cloche **et par e-mail**, si SMTP est réglé : une alerte que
personne ne vient lire ne protège rien. Si SMTP est en panne, ces e-mails ne partent
pas non plus : la cloche reste alors le seul signal.

## Mises à jour

Prérequis, une seule fois : un jeton GitHub à portée fine, limité aux dépôts des
sites, avec les permissions *Actions : lecture et écriture* et *Pull requests :
lecture*, enregistré dans *Paramètres › Dépôts Git*. La seconde sert au rattrapage
des pull requests fusionnées (voir *Rapport mensuel*) ; sans elle, les lancements
fonctionnent, mais le rattrapage signale une erreur d'accès. Un jeton déjà
enregistré se complète côté GitHub, sans le retirer d'ici.

Puis, pour chaque projet : renseigner son dépôt GitHub, choisir **l'environnement**
(aucun n'est choisi d'office), le workflow, la branche et, si vous le souhaitez,
une fréquence. Vide, la mise à jour ne part qu'à la demande.

Déroulé d'un lancement :

1. L'écran ou la planification enregistre la demande ; une tâche de fond appelle
   GitHub dans la minute. Un seul lancement est actif par projet.
2. GitHub démarre le workflow avec trois entrées : `environment`, `callback_url`
   et `callback_token`.
3. Le workflow rend compte, réussi ou non.

Sans compte rendu sous deux heures, le lancement compte comme un échec. Après un
échec, les admins sont prévenus et **la planification s'arrête** jusqu'à une
relance à la main. Un compte rendu tardif reste accepté pendant sept jours.

Seuls un `429` ou un `503` de GitHub sont retentés. Une coupure réseau ou un autre
`5xx` laisse un doute : le workflow tourne peut-être déjà, il ne doit pas partir
deux fois, donc Piilot s'arrête et vous demande de vérifier dans le dépôt.

Enregistrer une fréquence ne lance rien : la première échéance part de
l'enregistrement. Désactiver la cible ou changer d'environnement annule un
lancement encore en attente. Si le dépôt du projet change, rien ne part avant que
vous ayez enregistré à nouveau la cible. Choisir l'environnement, lancer et régler
les sauvegardes exigent la permission `reports.write`.

### Modèle de workflow

À adapter dans `.github/workflows/piilot-update.yml`. Piilot ne décide pas de ce
que fait la mise à jour : c'est au dépôt de le dire, tests compris. Utilisez les
[environnements GitHub](https://docs.github.com/actions/deployment/targeting-different-environments)
pour séparer les secrets de préproduction et de production, et imposer une
validation sur la production si vous le souhaitez. **Les deux heures incluent
l'attente de cette validation**, d'où le groupe `concurrency` du modèle : une
relance après un dépassement n'exécute pas la mise à jour deux fois.

Le jeton de compte rendu ne vaut que pour ce lancement et ne permet que de rendre
compte. Il reste lisible dans les entrées du workflow par qui lit le dépôt.

```yaml
name: Mise à jour par Piilot

on:
  workflow_dispatch:
    inputs:
      environment:
        type: choice
        options: [preprod, prod]
        required: true
      callback_url:
        type: string
        required: true
      callback_token:
        type: string
        required: true

concurrency:
  group: piilot-update-${{ inputs.environment }}
  cancel-in-progress: false

jobs:
  update:
    runs-on: ubuntu-latest
    environment: ${{ inputs.environment }}
    steps:
      - name: Masquer le jeton
        run: |
          token="$(jq -r '.inputs.callback_token' "$GITHUB_EVENT_PATH")"
          echo "::add-mask::$token"

      - uses: actions/checkout@v4

      # À remplacer : mettre à jour, tester, déployer sur l'environnement demandé.
      - name: Mettre à jour
        run: ./scripts/update.sh "${{ inputs.environment }}"

      # always() : un échec doit lui aussi être signalé.
      - name: Rendre compte à Piilot
        if: always()
        env:
          JOB_STATUS: ${{ job.status }}
          RUN_URL: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
        run: |
          status=failed
          [ "$JOB_STATUS" = "success" ] && status=success
          url="$(jq -r '.inputs.callback_url' "$GITHUB_EVENT_PATH")"
          token="$(jq -r '.inputs.callback_token' "$GITHUB_EVENT_PATH")"
          jq -n --arg status "$status" --arg run_url "$RUN_URL" '{status: $status, run_url: $run_url}' \
            | curl -fsS --retry 3 -X POST "$url" \
                -H "Authorization: Bearer $token" \
                -H "Content-Type: application/json" \
                --data-binary @-
```

## Sauvegardes

*Surveiller les sauvegardes* tire un jeton, montré une seule fois. Le script de
sauvegarde du site appelle Piilot à la fin de chaque exécution :

```sh
curl -fsS --retry 3 -X POST https://piilot.example/api/v1/hooks/site-backups \
  -H "Authorization: Bearer $PIILOT_BACKUP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"success","size_bytes":1048576}'
```

`status` vaut `success` ou `failed` ; `size_bytes`, `message` et `finished_at`
sont facultatifs. Sans sauvegarde réussie dans le délai attendu (26 heures par
défaut), ou si la dernière exécution a échoué, les admins sont prévenus une fois
par jour. Rangez le jeton dans les secrets du site, jamais dans son dépôt.

La date du **dernier test de restauration** se saisit à la main : une sauvegarde
jamais restaurée n'est pas une preuve.

## Domaines

L'échéance de chaque domaine est relevée par RDAP chaque semaine. Les admins sont
prévenus à 60, 30, 14, 7 et 1 jours, et si le relevé ne réussit plus depuis trois
jours. Certaines extensions nationales ne publient pas leur date : le relevé
reste alors en échec, et l'alerte le dit.

## Déploiements (Coolify)

*Paramètres → Modules → Coolify* (admins) : Piilot lit les applications de
l'instance [Coolify](https://coolify.io) de l'agence et leurs déploiements, en
lecture seule. Il ne déclenche jamais un déploiement.

### Prérequis

1. **Dans Coolify**, activer l'API dans les réglages de l'instance (*Settings →
   Advanced*, option d'accès à l'API ; l'intitulé varie selon la version). Sur une
   instance auto-hébergée elle est désactivée par défaut, et Coolify répond alors
   « accès refusé ».
2. Si une liste d'adresses IP autorisées est renseignée, y ajouter l'adresse IP
   **sortante** du serveur qui héberge Piilot. Une adresse non autorisée donne la
   même erreur d'accès refusé.
3. *Keys & Tokens → API tokens* (intitulé à vérifier selon la version) : créer un
   jeton avec la seule permission **read** (jamais *write* ni *deploy*), limité aux
   projets et ressources utiles quand l'écran le propose, avec une expiration.
   Copier le jeton **complet**, préfixe compris (`1|…`) : Coolify ne le montre
   qu'une fois.
4. **Dans Piilot**, renseigner l'adresse de l'instance (sans `/api/v1`) et le
   jeton, cocher *Activer le module*, choisir l'intervalle (15 minutes à 24
   heures, 1 heure par défaut), enregistrer. *Tester la connexion* demande une
   relève tout de suite ; le résultat s'affiche en haut de l'écran.

Le jeton est chiffré en base et ne ressort jamais de l'API. `COOLIFY_URL` et
`COOLIFY_API_TOKEN` peuvent le fixer par l'environnement : elles priment alors, et
l'écran les montre en lecture seule. Un jeton expiré ou révoqué se repère à
l'erreur de relève affichée sur l'écran du module.

Pour le « jeton » GitHub du rattrapage des pull requests, voir *Mises à jour* plus
haut : il lui faut la permission *Pull requests : lecture*.

### Fonctionnement

- **Liaison** : dans *Projet › Paramètres › Maintenance*, la carte *Déploiements*
  lie le projet à ses applications Coolify (production, préproduction, dix au plus),
  choisies dans la liste relevée.
- **Fréquence** : la liste des applications est relue à l'intervalle du module. Pour
  les applications liées, les déploiements **réussis** (statut `finished`) du mois
  courant et des deux précédents sont relus à chaque passage, et écrits une seule
  fois (un déploiement porte son identifiant Coolify : deux relèves ne doublent
  rien). Les échecs, déploiements en cours et statuts inconnus ne sont pas écrits ;
  un statut inconnu est signalé dans les journaux.
- **Priorité sur le webhook** : pour un projet et un mois, la section *Mises en
  ligne* du rapport n'affiche qu'**une** source. Si Coolify a relevé des
  déploiements ce mois-là, ce sont eux (« d'après Coolify ») ; sinon, ceux du
  webhook Git (« d'après le dépôt »). Les rapports d'avant la 1.6 ne disent pas leur
  source. Le journal du client n'est pas alimenté par les déploiements Coolify.
- **Le 1er du mois** : avant de créer le brouillon d'un projet lié, Piilot demande
  une relève et l'attend, pendant le délai d'attente des rapports. Une relève
  terminée, même en échec, ne fait plus attendre.
- **Retenue** : un brouillon d'un projet lié ne part pas tout seul tant que la
  dernière relève n'a pas **réussi** après la fin du mois (motif *Coolify n'a pas
  été relevé avec succès…*, avec un lien vers l'onglet Maintenance). Les déploiements
  ont pu changer depuis. Un module inactif ne retient rien.

### Limites connues

- Retirer une application d'un projet **ne supprime pas** les déploiements déjà
  relevés : ils restent dans le rapport (la table ne retient pas l'application
  d'origine).
- La relève ne remonte pas plus de **trois mois** (mois courant et deux
  précédents) : un déploiement plus ancien qui n'a pas été relevé à temps ne l'est
  plus.
- La lecture s'interrompt (et la relève passe en erreur) au-delà de 2 000
  déploiements pour une application, plutôt que de rendre une liste tronquée.
- L'heure retenue pour un déploiement est celle de sa fin (`updated_at`, à défaut
  `created_at`), dans le fuseau de l'agence pour les bornes du mois.

### À vérifier sur une instance Coolify réelle

Le schéma de l'API est mal documenté ; le client lit les réponses avec
tolérance, mais ces points n'ont été vérifiés que sur la documentation :

1. **Statut de succès** : `finished` est le seul statut retenu
   (`coolifySuccessStatuses`, dans `api/internal/usecase/coolify.go`). Si Coolify
   en emploie un autre pour un déploiement réussi, la section restera vide : c'est
   là qu'on l'ajoute.
2. **Champs `environment` et `project`** d'une application : Piilot lit
   `environment.name` et `environment.project.name`, ou à plat `environment_name`
   et `project_name`. Si la réponse ne donne que `environment_id`, ces deux champs
   resteront vides dans le menu de choix (sans conséquence sur les rapports).
3. **Ordre de la pagination** de `/deployments/applications/{uuid}` : détecté sur
   la première page, et le code ne s'y fie pas pour être exact ; seul le nombre
   d'appels en dépend. À confirmer, ainsi que le nom des champs `deployment_uuid`,
   `git_commit_sha`, `commit_message`, `created_at` et `updated_at`.
4. **Message d'une API désactivée ou d'une IP refusée** : il est repris tel quel
   dans l'erreur de relève ; vérifier qu'il est lisible.

## Rapport mensuel

Ajoutez les sections *Mises en ligne*, *Mises à jour*, *Sauvegardes* et *Domaine* à une formule. Les
mises à jour comptent les lancements réussis, par environnement, et les pull
requests fusionnées, reçues par le webhook Git. Un
lancement en préproduction s'affiche comme tel, et ne vaut jamais une mise à jour
de la production pour lever une retenue.

### Rattrapage des pull requests

Le webhook peut manquer un évènement (coupure de Piilot, webhook mal réglé). Pour
les projets qui ont un dépôt **GitHub** et une formule avec la section *Mises à
jour*, Piilot relit donc GitHub une fois par jour, et le 1er du mois avant de créer
le brouillon : il lit les pull requests fusionnées du mois courant et du précédent,
et n'inscrit que celles qui manquent. Une pull request déjà reçue par le webhook
n'est jamais écrasée, et la synchronisation ne touche ni aux tickets ni aux tâches.
Le brouillon d'un projet attend cette synchronisation pendant le délai d'attente
des rapports ; un essai terminé, même en échec, ne le fait plus attendre. En
revanche, tant que la dernière synchronisation n'a pas **réussi** après la fin du
mois, le brouillon ne part pas tout seul (motif *Le dépôt GitHub n'a pas été
synchronisé avec succès…*, avec un lien vers l'onglet Maintenance). Sans jeton
GitHub enregistré, rien n'est attendu et rien ne retient : le webhook fait foi.

*Projet › Paramètres › Maintenance*, carte *Pull requests fusionnées* : date de la
dernière synchronisation réussie, erreur du dernier essai, et *Synchroniser
maintenant* (permission `reports.write`). Les erreurs sont dites en clair : aucun
jeton, jeton refusé ou expiré, permission « Pull requests : lecture » manquante,
dépôt introuvable ou inaccessible, limite de débit GitHub. GitLab n'est pas encore
pris en charge pour le rattrapage : seul son webhook compte.

Une mise à jour de dépendance est reconnue à son auteur (`renovate[bot]`,
`dependabot[bot]`), à sa branche (`renovate/…`, `dependabot/…`) ou à l'étiquette
`dependencies`. Un correctif de sécurité l'est à une étiquette contenant
`security` ou `sécurité`, ou à un titre contenant `[SECURITY]`, `CVE-` ou `GHSA-`.

**Limites connues**
- Piilot ne lit pas les alertes Dependabot. Un correctif de sécurité de Dependabot
  sans étiquette de sécurité, au titre sans `CVE-` ni `GHSA-`, compte comme une
  mise à jour ordinaire : le nombre de correctifs de sécurité peut être
  sous-estimé. Le mieux est d'étiqueter ces pull requests `security` (option
  `labels` de `dependabot.yml`).
- Le rattrapage ne lit que le mois courant et le précédent : pas de rattrapage
  au-delà, et une pull request fusionnée plus tôt que le mois précédent, jamais
  reçue par le webhook, n'est pas retrouvée.
- **GitLab n'est pas encore pris en charge** par le rattrapage : seul son webhook
  compte, et ses merge requests ne sont pas rattrapées.
- Le rattrapage n'inscrit que des pull requests pour le rapport : il ne fait
  avancer ni ticket ni tâche (ce que seul le webhook fait).

### Envoi automatique

L'envoi automatique, dans *Paramètres › Formules de maintenance*, n'envoie que ce
qui ne pose pas de question. Un rapport attend une relecture, et les admins sont
prévenus, si :

| Motif | Condition |
|---|---|
| Destinataires | aucun |
| Disponibilité | aucun site de production surveillé, ou un jour sans mesure |
| Incident | au moins un dans le mois |
| Sauvegardes | non surveillées, ou en défaut à la fin du mois |
| Domaine | échéance inconnue, ou à moins de 30 jours |
| Mises à jour | ni dépendance fusionnée ni lancement réussi en production dans le mois, ou dernier lancement en production en échec |
| Synchronisation Git | dépôt GitHub et jeton présents, mais dernière synchronisation non réussie après la fin du mois |
| Déploiements | projet lié à Coolify, module actif, mais dernière relève non réussie après la fin du mois |
| Relecture | rapport remis en brouillon, ou projet en « Toujours relire » |

Valider un rapport à la main enregistre la version relue : si ses données ont
changé depuis l'ouverture de l'écran, Piilot refuse et demande de le relire.

L'audience ne retient jamais un rapport. Les données d'un brouillon suivent ce qui
se saisit encore sur le mois, jusqu'à son départ. Un e-mail que la file abandonne
après six essais est signalé sur le rapport, qui peut le relancer.
