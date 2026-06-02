import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      // NOTE: /share/* is NOT proxied in dev. In dev, the SPA's React Router
      // handles /share/:slug directly (renders the public read-only view).
      // The production reverse-proxy rule (or a built + served-by-Go setup)
      // is what enables SSR og: meta injection — see README.
    },
  },
})