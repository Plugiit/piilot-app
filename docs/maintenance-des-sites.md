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
sites, avec la seule permission *Actions : lecture et écriture*, enregistré dans
*Paramètres › Dépôts Git*.

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

## Rapport mensuel

Ajoutez les sections *Mises à jour*, *Sauvegardes* et *Domaine* à une formule. Les
mises à jour comptent les lancements réussis, par environnement, et les pull
requests fusionnées de Renovate ou Dependabot, reçues par le webhook Git. Un
lancement en préproduction s'affiche comme tel, et ne vaut jamais une mise à jour
de la production pour lever une retenue.

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
| Relecture | rapport remis en brouillon, ou projet en « Toujours relire » |

Valider un rapport à la main enregistre la version relue : si ses données ont
changé depuis l'ouverture de l'écran, Piilot refuse et demande de le relire.

L'audience ne retient jamais un rapport. Les données d'un brouillon suivent ce qui
se saisit encore sur le mois, jusqu'à son départ. Un e-mail que la file abandonne
après six essais est signalé sur le rapport, qui peut le relancer.
