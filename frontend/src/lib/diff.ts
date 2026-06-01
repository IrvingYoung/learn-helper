// Tiny line-level inline diff. Returns hunks of {type, text} where type is
// "context" | "add" | "del". For display, render add in green, del in red.
//
// This is intentionally simple — not a Myers diff. Good enough for the
// inline diff view in the right panel.
export type DiffLine = { type: "context" | "add" | "del"; text: string };

export function inlineDiff(before: string, after: string): DiffLine[] {
  const a = before.split("\n");
  const b = after.split("\n");
  const out: DiffLine[] = [];

  // Simple LCS-based line diff. O(n*m) — fine for pages up to a few thousand lines.
  const m = a.length, n = b.length;
  const lcs: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
  for (let i = m - 1; i >= 0; i--) {
    for (let j = n - 1; j >= 0; j--) {
      lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
    }
  }

  let i = 0, j = 0;
  while (i < m && j < n) {
    if (a[i] === b[j]) {
      out.push({ type: "context", text: a[i] });
      i++; j++;
    } else if (lcs[i + 1][j] >= lcs[i][j + 1]) {
      out.push({ type: "del", text: a[i] });
      i++;
    } else {
      out.push({ type: "add", text: b[j] });
      j++;
    }
  }
  while (i < m) { out.push({ type: "del", text: a[i++] }); }
  while (j < n) { out.push({ type: "add", text: b[j++] }); }
  return out;
}

export function diffSummary(before: string, after: string): { added: number; removed: number } {
  const lines = inlineDiff(before, after);
  return {
    added: lines.filter(l => l.type === "add").length,
    removed: lines.filter(l => l.type === "del").length,
  };
}
