# Sécurité

## Versions maintenues

Seule la **dernière version publiée** reçoit les correctifs de sécurité. Avant
de signaler une faille, vérifier qu'elle se reproduit sur cette version
(fichier `VERSION`, ou `/health/live` sur une instance).

| Version | Correctifs de sécurité |
|---|---|
| Dernière release | ✅ |
| Versions antérieures | ❌ Mettre à jour |

## Signaler une faille

**Ne pas ouvrir d'issue publique**, ni de pull request, ni de discussion : la
faille serait visible de tous avant d'être corrigée.

Utiliser le signalement privé de GitHub : onglet
[*Security*](https://github.com/Plugiit/piilot-app/security) du dépôt, puis
*Report a vulnerability*. Le signalement n'est visible que des mainteneurs.

Indiquer :

- la version concernée ;
- la faille et son impact : ce qu'un attaquant peut lire, modifier ou
  obtenir, et avec quel niveau d'accès au départ (anonyme, compte `client`,
  compte `team`) ;
- les étapes pour la reproduire, sur une instance de test ;
- une correction, si vous en avez une.

## Ce qui suit

- Accusé de réception sous **7 jours**.
- Correctif publié dans une release dédiée, avec une note dans le
  `CHANGELOG` et un avis de sécurité GitHub.
- La faille est rendue publique **après** la publication du correctif. Vous
  êtes crédité dans l'avis, sauf si vous préférez rester anonyme.

## Dans le périmètre

Toute faille du code de ce dépôt : contournement de l'authentification ou des
permissions, accès d'un compte `client` aux données d'un autre client,
injection, XSS, fuite de données dans les réponses ou les logs, faille de
l'image Docker telle qu'elle est construite ici.

## Hors périmètre

- Une instance mal configurée par son exploitant (HTTPS absent, secret faible,
  port de base de données exposé).
- Les failles des dépendances déjà publiées en amont : Dependabot les suit.
- Les tests de charge et de déni de service contre une instance que vous
  n'exploitez pas.

**Ne jamais tester contre une instance en production qui ne vous appartient
pas.** Une instance locale se monte en quelques minutes (voir le
[README](README.md#développement)).
