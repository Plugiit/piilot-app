# plugiit-app-reactjs

Front Plugiit — SPA React servie en statique, consommant `plugiit-api-go`.

Une seule application pour **tout le front** : back-office de l'agence
(`/admin/*`), portail client (`/client/*`) et connexion (`/login`).

## Stack

| Role | Choix |
|---|---|
| Rendu | React 19 + Vite 8 |
| Routage | TanStack Router (fichiers, code splitting automatique, prefetch au survol) |
| Etat serveur | TanStack Query |
| Tables | TanStack Table + TanStack Virtual |
| Style | Tailwind CSS v4 (tokens dans `src/index.css`) |
| Formulaires | React Hook Form + Zod |
| Client HTTP | openapi-fetch, types generes depuis l'OpenAPI de l'API |
| Tests | Vitest + Testing Library |

## Demarrer

```bash
cp .env.example .env
npm install
npm run dev            # http://localhost:5173
```

L'API doit tourner sur `http://localhost:8080` : le proxy Vite renvoie `/api`
vers elle, ce qui garde le cookie de session same-origin en local et evite
d'avoir a configurer CORS pour developper.

## Scripts

| Commande | Effet |
|---|---|
| `npm run dev` | Serveur de developpement |
| `npm run build` | Genere les routes, verifie les types, construit `dist/` |
| `npm run test` | Tests unitaires |
| `npm run check` | Routes + lint + types + tests (a lancer avant de pousser) |
| `npm run routes:gen` | Regenere `src/routeTree.gen.ts` |
| `npm run api:types` | Regenere `src/types/api.ts` depuis l'OpenAPI de l'API |

## Architecture

```
src/
├── routes/           # une route = un fichier, code-splitte automatiquement
│   ├── __root.tsx
│   ├── index.tsx     # aiguillage selon le role, n'affiche rien
│   ├── login.tsx     # connexion, commune aux deux espaces
│   ├── admin.tsx     # garde du back-office (roles admin, team)
│   ├── admin/        # ecrans de l'agence
│   ├── client.tsx    # garde du portail (role client)
│   └── client/       # ecrans du portail
├── features/         # un dossier par domaine metier (queryOptions + composants)
├── components/
│   ├── ui/           # primitives sans logique metier
│   └── layout/
├── lib/              # client API, cache, session, utilitaires
└── types/api.ts      # GENERE — ne pas editer a la main
```

### Les deux espaces

`src/lib/auth.ts` porte la regle d'aiguillage (`homeFor`, `isInternal`) : la
racine, la connexion et les deux gardes s'y referent, pour qu'elle n'existe
qu'a un seul endroit.

Les gardes de `admin.tsx` et `client.tsx` s'executent dans `beforeLoad`, donc
avant le rendu et avant les loaders enfants. Un compte du mauvais espace est
redirige vers le sien, pas vers la connexion.

**Ces gardes ne protegent rien.** Elles evitent d'afficher des ecrans hors
sujet. C'est l'API qui refuse les donnees, sur chaque endpoint — et le portail
etant expose a l'exterieur, tout endpoint qu'il consomme filtre sur le client
de l'appelant.

### Regles qui tiennent la performance

1. **Un endpoint par vue**, jamais par entite : chaque route appelle une URL
   qui renvoie exactement ce que l'ecran affiche.
2. **Pagination, tri et filtre cote serveur**, toujours. Les parametres vivent
   dans l'URL (`validateSearch`), donc la vue est partageable et la cle de
   cache en decoule.
3. **`preload="intent"` sur les liens** : le loader de la route cible part au
   survol, la navigation est perceptuellement instantanee.
4. **`staleTime` de 30 s** par defaut : revenir sur un ecran deja vu n'emet
   aucune requete.
5. **Virtualisation au-dela de 100 lignes** (`@tanstack/react-virtual`).
6. **Aucun appel externe dans le chemin de rendu** : c'est le role de l'API.

Cibles : bundle initial < 200 Ko gzip, navigation a chaud 0 ms, navigation a
froid < 100 ms.

## Deploiement (Coolify)

Application de type **Dockerfile**, port interne **8080**.

Build args a definir :

| Variable | Exemple |
|---|---|
| `VITE_API_URL` | `https://api.plugiit.com` |

`VITE_API_URL` est lue **au build**, pas au runtime : la modifier impose de
reconstruire l'image.

Le build echoue si les types ou les tests echouent — aucune image n'est
produite sur du rouge. La sonde `/healthz` sert au healthcheck.
