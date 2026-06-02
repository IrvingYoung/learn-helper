## 1. Setup

- [x] 1.1 Add `modern-screenshot` to `frontend/package.json` dependencies and run `pnpm install`

## 2. Core library (`frontend/src/lib/share-as-image.ts`)

- [x] 2.1 Implement `exportPageAsPng(sourceEl: HTMLElement, options?: { width?: number; padding?: number }): Promise<Blob>` — creates off-screen white-bg container at `left: -99999px`, `cloneNode(true)` the source into it, wait for `.mermaid-loading` count to be 0 (max 5s), call `domToPng` with `scale: window.devicePixelRatio` and `backgroundColor: '#fff'`, return the Blob, and remove the off-screen node in a `finally` block
- [x] 2.2 Implement `copyPngToClipboard(blob: Blob): Promise<boolean>` — feature-detect `navigator.clipboard?.write && window.ClipboardItem`, return `false` if unsupported; otherwise `await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])`
- [x] 2.3 Implement `downloadBlob(blob: Blob, filename: string): void` — create object URL, anchor with `download` attribute, click, revoke URL on next tick
- [x] 2.4 Implement `formatFilename(slug: string, date = new Date()): string` — returns `wiki-${slug}-${YYYY-MM-DD}.png` using local timezone

## 3. UI components

- [x] 3.1 Create `frontend/src/components/ShareAsImageModal.tsx` — props: `{ open: boolean, blob: Blob | null, error: string | null, onClose: () => void, onRetry: () => void }`. Renders modal with backdrop, preview area showing the blob as `<img>`, byte size + dimensions line, three buttons (Copy / Download / Close). Copy button disabled when `!supportsClipboard` (detect on mount). Loading state via `blob === null && error === null`. Error state shows message + Retry.
- [x] 3.2 Add "分享为图片" button to `PageViewer.tsx` title block — refactor title block to a flex row: title on left, button on right; button text toggles between "分享为图片" and "生成中…"; disabled while generating
- [x] 3.3 Wire state in `PageViewer.tsx` — add `useRef<HTMLDivElement>` for the markdown body container, `useState` for modal open + blob + error. On button click: (a) close any open selection tooltip, (b) set generating, (c) call `exportPageAsPng(bodyRef.current)`, (d) on success open modal with blob, (e) on error open modal with error string, (f) on finally set generating false

## 4. Verification

- [ ] 4.1 Manual test on a normal wiki page (no mermaid): share icon button appears in title block (no text label). Click → dropdown menu opens with "用图片分享". Click menu item → modal opens with PNG preview, download saves `wiki-{slug}-2026-06-02.png`, image is white-bg
- [ ] 4.2 Manual test on a page with a mermaid block: confirm PNG includes the rendered diagram (not the loading state)
- [ ] 4.3 Manual test with dark theme enabled in the app: confirm exported PNG is still light-themed
- [ ] 4.4 Manual test of clipboard copy on localhost: click "复制到剪贴板", switch to WeChat / Notes app, paste, confirm image appears
- [ ] 4.5 Manual test of clipboard disabled state (simulate by overriding `navigator.clipboard` in devtools): confirm Copy button is disabled with tooltip
- [ ] 4.6 Manual test of dropdown menu close: click outside the open menu, confirm it closes; press Escape, confirm it closes
- [ ] 4.7 Visual polish: confirm exported PNG has 4px orange bar at top, footer at bottom with "learn-helper · DATE · N 字", overall image ~800px wide with comfortable padding
- [ ] 4.8 UI chrome excluded: open the share menu, then click "用图片分享"; confirm the generated PNG does NOT contain "用图片分享" text, the share icon, or the page status ribbon
