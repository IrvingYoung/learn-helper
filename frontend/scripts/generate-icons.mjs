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
