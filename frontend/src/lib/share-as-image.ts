import { domToBlob } from "modern-screenshot";

const OFFSCREEN_LEFT = -99999;
const MERMAID_WAIT_TIMEOUT_MS = 5000;
const MERMAID_POLL_INTERVAL_MS = 50;
const MERMAID_SETTLE_MS = 200;
const ACCENT_COLOR = "#c45c26";
const BRAND_TEXT = "learn-helper";

const LIGHT_THEME_VARS = `
  --bg-primary: #faf9f7;
  --bg-secondary: #ffffff;
  --bg-tertiary: #f5f3ef;
  --border: #eae7e0;
  --border-hover: #d9d5cc;
  --text-primary: #1a1815;
  --text-secondary: #4a4640;
  --text-muted: #8a857a;
  --accent: #c45c26;
  --accent-light: #e87d3a;
  --accent-bg: #fdf6f0;
  --success: #2d7a3e;
  --warning: #c78a2a;
  --error: #dc2626;
  --node-filled: #2d7a3e;
  --node-partial: #c78a2a;
  --node-empty: #b5b0a6;
  --scrollbar-thumb: #d9d5cc;
  --scrollbar-thumb-hover: #b5b0a6;
  --shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
  --input-bg: #ffffff;
  --input-border: #d9d5cc;
  --user-bubble-bg: #c45c26;
  --user-bubble-text: #ffffff;
`;

const FORCED_STYLES = `
  *, *::before, *::after {
    color-scheme: light;
  }
  body, .prose-custom, [class*="th-bg-"], [class*="th-text-"] {
    color: #1a1815;
  }
  pre, code, [class*="th-bg-tertiary"] {
    background: #f5f3ef !important;
    color: #1a1815 !important;
  }
  [class*="th-separator"], [class*="th-border"] {
    border-color: #eae7e0 !important;
  }
`;

export interface ExportPageAsPngOptions {
  width?: number;
  padding?: number;
  scale?: number;
}

export async function exportPageAsPng(
  sourceEl: HTMLElement,
  options: ExportPageAsPngOptions = {},
): Promise<Blob> {
  const { width = 800, padding = 48, scale = window.devicePixelRatio || 1 } = options;

  const host = document.createElement("div");
  host.setAttribute("data-theme", "warm");
  host.style.cssText = [
    "position: fixed",
    `left: ${OFFSCREEN_LEFT}px`,
    "top: 0",
    `width: ${width}px`,
    "background: #ffffff",
    `padding: ${padding}px`,
    "box-sizing: border-box",
    "color: #1a1815",
    "font-family: 'Source Sans 3', system-ui, -apple-system, sans-serif",
    "pointer-events: none",
    "z-index: -1",
  ].join("; ");

  const topBar = document.createElement("div");
  topBar.style.cssText = `position: absolute; top: 0; left: 0; right: 0; height: 4px; background: ${ACCENT_COLOR};`;
  host.appendChild(topBar);

  const styleEl = document.createElement("style");
  styleEl.textContent = `:host, [data-theme="warm"] { ${LIGHT_THEME_VARS} } ${FORCED_STYLES}`;
  host.appendChild(styleEl);

  const clone = sourceEl.cloneNode(true) as HTMLElement;
  clone.querySelectorAll("[data-share-ui]").forEach((el) => el.remove());
  host.appendChild(clone);

  const footer = buildFooter(clone);
  host.appendChild(footer);

  document.body.appendChild(host);

  try {
    await waitForMermaidRender(host);
    const blob = await domToBlob(host, {
      scale,
      backgroundColor: "#ffffff",
      style: {
        background: "#ffffff",
        color: "#1a1815",
      },
    });
    return blob;
  } finally {
    host.remove();
  }
}

function buildFooter(articleClone: HTMLElement): HTMLElement {
  const wordCount = countWords(articleClone.textContent || "");
  const date = formatDateShort(new Date());
  const footer = document.createElement("footer");
  footer.style.cssText = [
    "margin-top: 48px",
    "padding-top: 16px",
    "border-top: 1px solid #eae7e0",
    "font-size: 12px",
    "color: #8a857a",
    "display: flex",
    "align-items: center",
    "gap: 8px",
    "font-family: 'Source Sans 3', system-ui, sans-serif",
    "line-height: 1",
  ].join("; ");

  const brand = document.createElement("span");
  brand.textContent = BRAND_TEXT;
  brand.style.cssText = `color: ${ACCENT_COLOR}; font-weight: 600;`;
  footer.appendChild(brand);
  footer.appendChild(dot());
  footer.appendChild(plain(date));
  footer.appendChild(dot());
  footer.appendChild(plain(`${wordCount} 字`));
  return footer;
}

function dot(): HTMLElement {
  const el = document.createElement("span");
  el.textContent = "·";
  el.style.opacity = "0.5";
  return el;
}

function plain(text: string): HTMLElement {
  const el = document.createElement("span");
  el.textContent = text;
  return el;
}

function countWords(text: string): number {
  const cjkChars = (text.match(/[一-鿿]/g) ?? []).length;
  const nonCjk = text.replace(/[一-鿿]/g, " ");
  const englishWords = nonCjk.split(/\s+/).filter(Boolean).length;
  return cjkChars + englishWords;
}

function formatDateShort(date: Date): string {
  const yyyy = date.getFullYear();
  const mm = String(date.getMonth() + 1).padStart(2, "0");
  const dd = String(date.getDate()).padStart(2, "0");
  return `${yyyy}-${mm}-${dd}`;
}

async function waitForMermaidRender(root: HTMLElement): Promise<void> {
  const start = performance.now();
  while (performance.now() - start < MERMAID_WAIT_TIMEOUT_MS) {
    const loading = root.querySelectorAll(".mermaid-loading").length;
    if (loading === 0) {
      await new Promise((r) => setTimeout(r, MERMAID_SETTLE_MS));
      return;
    }
    await new Promise((r) => setTimeout(r, MERMAID_POLL_INTERVAL_MS));
  }
}

export function supportsClipboardImageWrite(): boolean {
  return (
    typeof navigator !== "undefined" &&
    !!navigator.clipboard?.write &&
    typeof window !== "undefined" &&
    typeof window.ClipboardItem !== "undefined"
  );
}

export async function copyPngToClipboard(blob: Blob): Promise<boolean> {
  if (!supportsClipboardImageWrite()) return false;
  try {
    await navigator.clipboard.write([new window.ClipboardItem({ "image/png": blob })]);
    return true;
  } catch (err) {
    console.error("Failed to copy PNG to clipboard:", err);
    return false;
  }
}

export function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

export function formatFilename(slug: string, date: Date = new Date()): string {
  const yyyy = date.getFullYear();
  const mm = String(date.getMonth() + 1).padStart(2, "0");
  const dd = String(date.getDate()).padStart(2, "0");
  return `wiki-${slug}-${yyyy}-${mm}-${dd}.png`;
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
}

export async function getPngDimensions(
  blob: Blob,
): Promise<{ width: number; height: number } | null> {
  try {
    const buf = await blob.slice(0, 24).arrayBuffer();
    const view = new DataView(buf);
    const width = view.getUint32(16);
    const height = view.getUint32(20);
    if (width > 0 && height > 0) return { width, height };
    return null;
  } catch {
    return null;
  }
}
