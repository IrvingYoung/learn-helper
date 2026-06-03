import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg', 'icons/*.png'],
      manifest: {
        name: 'LLM Wiki — 个人知识库',
        short_name: 'LLM Wiki',
        description: 'AI 维护的个人知识库',
        theme_color: '#c45c26',
        background_color: '#fbf8f3',
        display: 'standalone',
        start_url: '/wiki',
        scope: '/',
        icons: [
          { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: '/icons/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        runtimeCaching: [
          {
            urlPattern: /^\/api\/wiki\/[^/]+$/,
            handler: 'StaleWhileRevalidate',
            options: {
              cacheName: 'wiki-pages',
              expiration: { maxEntries: 50, maxAgeSeconds: 60 * 60 * 24 * 7 },
            },
          },
          {
            urlPattern: /^\/api\/wiki\/tree/,
            handler: 'StaleWhileRevalidate',
            options: { cacheName: 'wiki-tree' },
          },
        ],
        navigateFallback: '/index.html',
      },
    }),
  ],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
