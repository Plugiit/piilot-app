# RGPD : export et effacement

Piilot permet à un administrateur (permission `rgpd.manage`) d'exporter les données d'une personne ou d'un client, et d'en demander l'effacement. Ce document dit ce qui est effacé, ce qui est conservé et pourquoi, les durées, et ce qu'il faut faire après une restauration de sauvegarde.

## Deux sujets, deux effacements

| Sujet | Ce qui se passe | Pourquoi |
|---|---|---|
| **Un compte** (équipe ou portail) | **Anonymisation** : la ligne reste, tout ce qui identifie la personne part | Le temps saisi (`time_entries`) est une donnée de gestion de projet et de facturation ; le supprimer fausserait les rapports et les budgets des projets. La suppression de la ligne le supprimerait par cascade |
| **Un client** | **Suppression complète** : tout ce qui se rattache au client est supprimé, puis le client | Un client qui demande l'effacement de ses données les retire toutes. Proposez-lui d'abord l'export (archive zip) |

## Déroulé

1. **Demande** (`POST /api/v1/admin/rgpd/erasures`). L'effacement est planifié à l'échéance `RGPD_ERASURE_GRACE` (720 h, soit 30 jours, par défaut). Tout de suite : le compte, ou chaque compte du portail du client, est **désactivé** et ses sessions et connexions Claude sont **révoquées**. L'état précédent (désactivé ou non, et depuis quand) est gardé pour l'annulation.
2. **Annulation** (`POST /api/v1/admin/rgpd/erasures/:id/cancel`), possible tant que l'effacement est « planifié » : rien n'est effacé, les comptes retrouvent leur état d'avant. Un compte déjà désactivé avant la demande le reste.
3. **Exécution** : une tâche de fond (toutes les minutes) exécute les effacements échus. Chacun tient dans **une seule transaction** : si une étape échoue, rien n'est modifié. L'effacement est alors marqué « échoué » (le détail est dans les logs) et ne se rejoue pas tout seul ; un administrateur en planifie un nouveau après avoir corrigé la cause. Deux instances de l'application ne peuvent pas exécuter le même effacement (verrou `SKIP LOCKED`).
4. **Fichiers** : leurs clés sont relevées dans la transaction, et les fichiers du disque sont supprimés **après** le COMMIT. Un échec de suppression est écrit dans les logs sans rien défaire : un fichier orphelin vaut mieux qu'un effacement annulé à moitié.

## Ce qui est effacé

### Compte (anonymisation)
- Adresse e-mail remplacée par `supprime-<id>@invalid`, nom, prénom, civilité, téléphone, adresse, ville, pays, lien de prise de rendez-vous, avatar, dernière connexion. Mot de passe rendu inutilisable, double authentification retirée, compte marqué supprimé.
- Liaison et codes Discord (le bot reçoit l'événement de retrait des rôles).
- Sessions, connexions Claude (autorisations, codes et jetons OAuth), notifications, chrono en cours, favoris, liens de réinitialisation de mot de passe, invitations.
- Appartenances aux projets, affectations de tâches, désabonnements des rapports.
- E-mails sortants en attente ou envoyés vers son adresse, adresse retirée des destinataires de rapports, e-mails reçus de cette adresse.
- Tickets : adresse et nom du demandeur vidés ; messages : nom et adresse de l'expéditeur vidés.
- Archives d'export RGPD dont le compte est le sujet (fichier supprimé, libellé effacé de l'historique).

### Client (suppression)
- Le client, ses contacts, ses projets avec leurs tâches, tickets et messages, jalons, livrables et versions, rapports, **temps saisi sur ces projets**, échanges, sites et sources, et tous leurs fichiers (pièces jointes de projets, de tâches et de tickets, logos de projets).
- E-mails reçus rattachés au client ou envoyés par ses contacts et comptes du portail, e-mails sortants à leurs adresses ou portant sur leurs rapports.
- Ses comptes du portail : anonymisés comme ci-dessus, puis détachés du client.
- Traces d'import (clés de correspondance et lignes de rapport) qui désignent ces éléments.
- Archives d'export RGPD dont le client est le sujet.

## Ce qui est conservé, et pourquoi

| Donnée | Raison |
|---|---|
| **Temps saisi par un compte effacé** | Donnée de gestion et de facturation des projets ; il n'est plus rattaché à une personne identifiable |
| **Ligne de l'utilisateur anonymisé** | Nécessaire à l'intégrité des références (tâches, tickets, livrables y pointent) ; elle ne contient plus aucune donnée personnelle |
| **Journal d'audit** | Intérêt légitime de sécurité. `actor_email` y est figé à l'écriture et **n'est pas effacé** ; il disparaît à l'expiration de `AUDIT_RETENTION` (365 jours par défaut). L'exécution d'un effacement y laisse une ligne `rgpd.erasure_executed` qui ne porte que des identifiants et des compteurs, aucun nom |
| **Historique des effacements et des exports** | Preuve que la demande a été traitée : sujet (type et identifiant), dates, compteurs. Le libellé devient « Supprimé » |
| **Texte libre saisi par l'équipe** | Commentaires de tâches, descriptions, notes de ticket : ils restent attachés à l'élément. Ils peuvent citer une personne par son nom ; pensez à les relire si la demande l'exige |

## Purge des comptes inactifs

`RGPD_INACTIVE_AFTER` vaut `0` par défaut : **la purge est désactivée**. Pour l'activer, indiquez une durée d'au moins `720h` ; `26280h` (3 ans) est un bon point de départ.

- Une passe quotidienne planifie l'effacement (motif « inactivité ») des comptes **non administrateurs** dont la dernière activité (connexion ou prolongation de session ; à défaut, la création) est plus ancienne que ce seuil. Les administrateurs ne sont jamais concernés. Les comptes désactivés manuellement le sont, eux.
- Ces comptes **ne sont pas désactivés** pendant le délai : se connecter, ou simplement rester connecté (la prolongation de session compte comme une activité), annule l'effacement (ligne d'audit `rgpd.erasure_cancelled`, avec `via: login` ou `via: refresh`).
- Si l'envoi d'e-mails est configuré (SMTP), un e-mail de préavis part au titulaire avec la date et le lien de connexion. Sans SMTP, aucun préavis n'est envoyé : activez la purge en connaissance de cause.

## Durées

| Réglage | Défaut | Rôle |
|---|---|---|
| `RGPD_ERASURE_GRACE` | 720 h | Délai entre la demande et l'exécution |
| `RGPD_INACTIVE_AFTER` | 0 (désactivée) | Inactivité au bout de laquelle un compte est planifié |
| `RGPD_EXPORT_TTL` | 168 h | Durée de vie d'une archive d'export prête |
| `AUDIT_RETENTION` | 365 jours | Durée du journal d'audit |
| `BACKUP_KEEP_*` | 7 / 4 / 3 | Sauvegardes quotidiennes / hebdomadaires / mensuelles |

## Sauvegardes : rejouer les effacements après une restauration

Les sauvegardes sont des copies de la base **avant** l'effacement : une personne effacée y figure encore tant que la sauvegarde existe, soit **jusqu'à 3 mois** avec la rétention par défaut (`BACKUP_KEEP_MONTHLY`). Ces copies ne servent qu'à restaurer et ne sont pas relues autrement.

**Après toute restauration, il faut rejouer les effacements** : la table `rgpd_erasures` restaurée est celle de la date de la sauvegarde et ignore les effacements plus récents. Procédure :

1. Avant de restaurer, conservez la liste des effacements exécutés depuis la date de la sauvegarde : `SELECT id, subject_type, subject_id, executed_at FROM rgpd_erasures WHERE status = 'done' AND executed_at > '<date de la sauvegarde>'` (sur la base d'avant restauration, ou à partir des lignes d'audit `rgpd.erasure_executed`, dont le `target_id` est l'identifiant de l'effacement et les `details` le sujet).
2. Restaurez.
3. Pour chaque `subject_type` / `subject_id` de cette liste, planifiez un effacement (`POST /api/v1/admin/rgpd/erasures`) puis, si le délai de rétractation n'a plus lieu d'être, avancez `scheduled_for` en base pour l'exécuter à la prochaine minute. Seuls les identifiants sont utilisés : aucune donnée personnelle n'est nécessaire.
4. Vérifiez que les effacements sont à l'état « done ».

Les fichiers du disque supprimés ne reviennent pas avec une sauvegarde de la base ; si le volume de fichiers est lui aussi restauré, les mêmes effacements les suppriment de nouveau.
