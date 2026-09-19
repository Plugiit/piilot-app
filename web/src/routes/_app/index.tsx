import { createFileRoute, redirect } from '@tanstack/react-router'

import { homeFor } from '@/lib/auth'

/**
 * Racine du back-office.
 *
 * Elle n'affiche rien : il n'y a pas d'ecran d'accueil au-dessus des modules,
 * le travail commence dans l'un d'eux. La route existe uniquement pour que la
 * racine du domaine mene quelque part plutot que sur un 404 — en production
 * c'est admin.plgt.fr tout court, l'adresse qu'on tape et qu'on met en favori.
 *
 * La redirection vit dans `beforeLoad` : elle part avant le rendu, donc aucun
 * chassis ne s'affiche le temps d'un aller-retour. `replace` garde l'historique
 * propre — sans lui, revenir en arriere depuis /pm rejouerait la redirection.
 *
 * La destination sort de `homeFor`, seule source de la regle d'atterrissage.
 * La garde de `_app` a deja renvoye les comptes clients vers leur portail : ici
 * l'utilisateur est forcement interne, mais lire la regle plutot que d'ecrire
 * /pm en dur evite d'avoir deux endroits a corriger le jour ou elle change.
 */
export const Route = createFileRoute('/_app/')({
  beforeLoad: ({ context }) => {
    throw redirect({ to: homeFor(context.user), replace: true })
  },
})
