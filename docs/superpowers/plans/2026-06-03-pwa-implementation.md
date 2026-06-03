# PWA Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add PWA support to LLM Wiki so it can be installed to home screen and work offline with cached app shell + previously-viewed pages.

**Architecture:** Use `vite-plugin-pwa` (Workbox under the hood) to generate a service worker and web manifest at build time. Configure runtime caching for wiki page API responses with stale-while-revalidate. Generate PWA icons from the existing brand SVG via a one-off sharp script.

**Tech Stack:** vite-plugin-pwa, Workbox, sharp, Vite 6, React 19

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `frontend/package.json` | Add vite-plugin-pwa + sharp dependencies |
| Create | `frontend/public/favicon-source.svg` | Source SVG for icon generation |
| Create | `frontend/scripts/generate-icons.mjs` | One-off script to generate PWA icons |
| Create | `frontend/public/icons/icon-192.png` | 192x192 PWA icon (generated, committed) |
| Create | `frontend/public/icons/icon-512.png` | 512x512 PWA icon (generated, committed) |
| Create | `frontend/public/icons/icon-maskable-512.png` | 512x512 maskable icon (generated, committed) |
| Modify | `frontend/vite.config.ts` | Register VitePWA plugin with manifest + workbox config |
| Modify | `frontend/index.html` | Add theme-color, apple-touch-icon, manifest link |
| Modify | `frontend/src/main.tsx` | Register service worker on app start |

---

### Task 1: Install Dependencies

**Files:**
- Modify: `frontend/package.json`

- [ ] **Step 1: Install vite-plugin-pwa and sharp**

Run:
```bash
cd /Users/irving/repo/learn-helper/frontend && npm install --save-dev vite-plugin-pwa sharp
```

Expected: Both packages added to `devDependencies` in `package.json`, lock file updated.

- [ ] **Step 2: Verify install**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no errors (no app code changed yet)

- [ ] **Step 3: Commit**

```bash
cd /Users/irving/repo/learn-helper && git add frontend/package.json frontend/package-lock.json
git commit -m "chore: add vite-plugin-pwa and sharp deps"
```

---

### Task 2: Extract Favicon Source SVG

**Files:**
- Create: `frontend/public/favicon-source.svg`

The existing favicon is embedded as a data URI in `index.html`. Extract it to a file so the icon generator can use it.

- [ ] **Step 1: Create the source SVG file**

Write `frontend/public/favicon-source.svg` with this content:

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect width="32" height="32" rx="6" fill="#c45c26"/>
  <path d="M16 8v16M8 16h16" stroke="white" stroke-width="3" stroke-linecap="round" fill="none"/>
</svg>
```

- [ ] **Step 2: Verify file is created**

Run: `cat /Users/irving/repo/learn-helper/frontend/public/favicon-source.svg`
Expected: SVG content displayed

- [ ] **Step 3: Commit**

```bash
cd /Users/irving/repo/learn-helper && git add frontend/public/favicon-source.svg
git commit -m "feat: extract favicon source SVG"
```

---

### Task 3: Icon Generation Script

**Files:**
- Create: `frontend/scripts/generate-icons.mjs`
- Modify: `frontend/package.json`

- [ ] **Step 1: Create the generate-icons script**

Write `frontend/scripts/generate-icons.mjs`:

```js
import sharp from 'sharp'
import { readFileSync, mkdirSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const src = join(__dirname, '..', 'public', 'favicon-source.svg')
const outDir = join(__dirname, '..', 'public', 'icons')
mkdirSync(outDir, { recursive: true })

const svg = readFileSync(src)

const sizes = [
  { size: 192, name: 'icon-192.png' },
  { size: 512, name: 'icon-512.png' },
]

for (const { size, name } of sizes) {
  await sharp(svg).resize(size, size).png().toFile(join(outDir, name))
  console.log(`Generated ${name}`)
}

const maskableSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect width="32" height="32" fill="#c45c26"/>
  <g transform="translate(16 16) scale(0.4) translate(-16 -16)">
    <path d="M16 8v16M8 16h16" stroke="white" stroke-width="6" stroke-linecap="round" fill="none"/>
  </g>
</svg>`
await sharp(Buffer.from(maskableSvg)).resize(512, 512).png()
  .toFile(join(outDir, 'icon-maskable-512.png'))
console.log('Generated icon-maskable-512.png')
```

- [ ] **Step 2: Add icons script to package.json**

Edit `frontend/package.json` to add to the `scripts` section:

```json
"scripts": {
  "dev": "vite",
  "build": "tsc -b && vite build",
  "preview": "vite preview",
  "icons": "node scripts/generate-icons.mjs"
}
```

- [ ] **Step 3: Run the script**

Run: `cd /Users/irving/repo/learn-helper/frontend && npm run icons`
Expected output:
```
Generated icon-192.png
Generated icon-512.png
Generated icon-maskable-512.png
```

- [ ] **Step 4: Verify icons exist**

Run: `ls -la /Users/irving/repo/learn-helper/frontend/public/icons/`
Expected: 3 PNG files (icon-192.png, icon-512.png, icon-maskable-512.png)

- [ ] **Step 5: Commit**

```bash
cd /Users/irving/repo/learn-helper && git add frontend/scripts/generate-icons.mjs frontend/package.json frontend/public/icons/
git commit -m "feat: generate PWA icons from source SVG"
```

---

### Task 4: Configure vite-plugin-pwa

**Files:**
- Modify: `frontend/vite.config.ts`

- [ ] **Step 1: Update vite.config.ts with PWA plugin**

Replace the contents of `frontend/vite.config.ts`:

```ts
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
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
cd /Users/irving/repo/learn-helper && git add frontend/vite.config.ts
git commit -m "feat: add vite-plugin-pwa configuration"
```

---

### Task 5: Update index.html with PWA Meta Tags

**Files:**
- Modify: `frontend/index.html`

- [ ] **Step 1: Update the head section**

Replace the `<head>` section of `frontend/index.html`:

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="theme-color" content="#c45c26" />
    <meta name="apple-mobile-web-app-capable" content="yes" />
    <meta name="apple-mobile-web-app-title" content="LLM Wiki" />
    <meta name="description" content="AI 维护的个人知识库 — 通过对话管理知识树" />
    <meta property="og:title" content="LLM Wiki" />
    <meta property="og:description" content="AI 维护的个人知识库" />
    <meta property="og:type" content="website" />
    <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='6' fill='%23c45c26'/%3E%3Cpath d='M16 8v16M8 16h16' stroke='white' stroke-width='3' stroke-linecap='round'/%3E%3C/svg%3E" />
    <link rel="icon" type="image/png" sizes="192x192" href="/icons/icon-192.png" />
    <link rel="icon" type="image/png" sizes="512x512" href="/icons/icon-512.png" />
    <link rel="apple-touch-icon" href="/icons/icon-192.png" />
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 2: Verify it builds**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx vite build 2>&1 | tail -10`
Expected: Build succeeds. The `dist/` should now contain `manifest.webmanifest` and `sw.js` (or `registerSW.js`).

- [ ] **Step 3: Verify manifest was generated**

Run: `ls /Users/irving/repo/learn-helper/frontend/dist/ | grep -E "manifest|sw"`
Expected: Files matching `manifest.webmanifest` and `registerSW.js` (or `sw.js`)

- [ ] **Step 4: Commit**

```bash
cd /Users/irving/repo/learn-helper && git add frontend/index.html
git commit -m "feat: add PWA meta tags to index.html"
```

---

### Task 6: Register Service Worker in main.tsx

**Files:**
- Modify: `frontend/src/main.tsx`

- [ ] **Step 1: Read the current main.tsx**

Run: `cat /Users/irving/repo/learn-helper/frontend/src/main.tsx`

- [ ] **Step 2: Add SW registration**

Edit `frontend/src/main.tsx`. After all existing imports, add:

```ts
import { registerSW } from 'virtual:pwa-register'
registerSW({ immediate: true })
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/irving/repo/learn-helper/frontend && npx tsc --noEmit`
Expected: no errors (the `virtual:pwa-register` types are provided by the plugin)

- [ ] **Step 4: Commit**

```bash
cd /Users/irving/repo/learn-helper && git add frontend/src/main.tsx
git commit -m "feat: register service worker on app start"
```

---

### Task 7: Build and Verify

- [ ] **Step 1: Clean build**

Run: `cd /Users/irving/repo/learn-helper/frontend && rm -rf dist && npm run build`
Expected: Build succeeds with no errors. Output shows `dist/` with `index.html`, `manifest.webmanifest`, `registerSW.js`, `icons/`, `assets/`.

- [ ] **Step 2: Verify all PWA artifacts exist**

Run: `ls /Users/irving/repo/learn-helper/frontend/dist/`
Expected: `index.html`, `manifest.webmanifest`, `registerSW.js` (or `sw.js`), `icons/`, `assets/`

- [ ] **Step 3: Verify manifest content**

Run: `cat /Users/irving/repo/learn-helper/frontend/dist/manifest.webmanifest`
Expected: JSON with name, short_name, theme_color, icons, etc.

- [ ] **Step 4: Verify SW was generated**

Run: `head -50 /Users/irving/repo/learn-helper/frontend/dist/registerSW.js` (or `sw.js`)
Expected: SW code with precache list and runtime caching rules

---

### Task 8: Manual Testing in Browser

- [ ] **Step 1: Start preview server**

Run: `cd /Users/irving/repo/learn-helper/frontend && npm run preview &`
Expected: Preview server starts (typically on port 4173)

- [ ] **Step 2: Verify Service Worker activates**

1. Open preview URL in Chrome
2. Open DevTools (F12 or Cmd+Option+I)
3. Go to Application tab → Service Workers
4. Expected: SW shows "activated and is running"

- [ ] **Step 3: Verify Manifest**

1. In DevTools → Application → Manifest
2. Expected: All fields show correctly (name, short_name, icons, theme_color, etc.)
3. Expected: "Installability" section shows no errors

- [ ] **Step 4: Test offline mode**

1. In DevTools → Network tab → check "Offline"
2. Reload the page → app shell should still load
3. Navigate to a previously-visited wiki page → content shows from cache
4. Try to navigate to a new page or send a chat message → fails with existing SWR error UI

- [ ] **Step 5: Test install prompt (optional)**

1. In Chrome address bar, look for install icon
2. Or check console for `beforeinstallprompt` event
3. Expected: Browser shows native install option

- [ ] **Step 6: Run Lighthouse PWA audit**

1. In DevTools → Lighthouse tab
2. Select "Progressive Web App" category
3. Click "Analyze page load"
4. Expected: PWA score ≥ 90 (ideally 100)

- [ ] **Step 7: Final cleanup**

Stop the preview server: `kill %1` (or use the process ID)
