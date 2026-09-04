# syntax=docker/dockerfile:1

# =============================================================================
# Stage 1 : build des assets
# =============================================================================
FROM node:24-alpine AS build

WORKDIR /app

# Manifestes d'abord : la couche d'installation ne rejoue que si les
# dependances changent, pas a chaque commit.
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

COPY . .

# L'URL de l'API est figee dans le bundle au build (Vite remplace
# import.meta.env a la compilation). A definir dans les build args Coolify.
ARG VITE_API_URL=""
ENV VITE_API_URL=${VITE_API_URL}

# Le typage et les tests bloquent le build : une image n'est produite que si
# la verification passe, donc un deploiement ne peut pas partir sur du rouge.
RUN npm run routes:gen \
    && npx tsc -b \
    && npm run test \
    && npx vite build

# =============================================================================
# Stage 2 : service des fichiers statiques
# =============================================================================
FROM nginx:1.29-alpine AS production

# La configuration par defaut de l'image ecoute sur le port 80 en root ; elle
# est remplacee integralement par nginx.conf.
RUN find /usr/share/nginx/html -mindepth 1 -delete \
    && rm /etc/nginx/conf.d/default.conf

COPY nginx.conf /etc/nginx/nginx.conf
COPY --from=build /app/dist /usr/share/nginx/html

# nginx tourne en non-root : l'image officielle fournit deja l'utilisateur
# nginx (uid 101), il lui faut seulement pouvoir lire les fichiers servis.
# Ses fichiers temporaires et son pid sont rediriges vers /tmp par nginx.conf.
RUN chown -R nginx:nginx /usr/share/nginx/html

USER nginx

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 || exit 1

CMD ["nginx", "-g", "daemon off;"]
