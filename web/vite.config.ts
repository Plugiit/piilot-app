import { readFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

/**
 * Version du front, lue dans le fichier VERSION a la racine du depot — celui
 * que le script de release tient a jour et que le binaire Go embarque aussi.
 * Le front la compare a celle du serveur pour proposer un rechargement apres
 * un deploiement.
 */
function appVersion() {
  try {
    return readFileSync(new URL('../VERSION', import.meta.url), 'utf8').trim()
  } catch {
    return 'dev'
  }
}

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(appVersion()),
  },

  plugins: [
    // Genere src/routeTree.gen.ts depuis src/routes/. Doit precéder le plugin
    // React pour que l'arbre soit a jour avant la transformation JSX.
    tanstackRouter({ target: 'react', autoCodeSplitting: true }),
    react(),
    tailwindcss(),
  ],

  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },

  server: {
    port: 5173,
    // L'API tourne sur 8080 : on passe par un proxy en dev pour que le cookie
    // de session soit same-origin et que CORS ne s'applique pas localement.
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      // La sonde porte la version du serveur, que le front surveille.
      '/health': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },

  build: {
    // Le decoupage par route est fait par autoCodeSplitting. On isole en plus
    // les dependances lourdes partagees pour qu'un deploiement qui ne touche
    // que le code applicatif n'invalide pas leur cache navigateur.
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (id.includes('/react-dom/') || id.includes('/react/')) return 'react'
          if (id.includes('/@tanstack/')) return 'tanstack'
        },
      },
    },
    // Un chunk au-dela de 300 Ko sur un admin signale presque toujours un
    // import statique qui aurait du etre dynamique.
    chunkSizeWarningLimit: 300,
  },

  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
