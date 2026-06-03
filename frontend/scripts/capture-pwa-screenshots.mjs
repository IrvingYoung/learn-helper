import { chromium } from 'playwright'
import { mkdirSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const outDir = join(__dirname, '..', 'public', 'screenshots')
mkdirSync(outDir, { recursive: true })

const URL = 'http://localhost:4173/wiki'

const targets = [
  { name: 'desktop.png', width: 1280, height: 720 },
  { name: 'mobile.png', width: 750, height: 1334 },
]

const browser = await chromium.launch()
for (const { name, width, height } of targets) {
  const context = await browser.newContext({
    viewport: { width, height },
    deviceScaleFactor: 1,
  })
  const page = await context.newPage()
  await page.goto(URL, { waitUntil: 'networkidle', timeout: 15000 })
  // Wait a bit for the app to render
  await page.waitForTimeout(1500)
  await page.screenshot({
    path: join(outDir, name),
    fullPage: false,
  })
  console.log(`Captured ${name} (${width}x${height})`)
  await context.close()
}
await browser.close()
