## Context

Currently, the AI can edit wiki pages only via `update_page`, which takes the full `content` string and replaces the entire page content in the database. This works well for small pages but has two problems as pages grow:

1. **Token waste**: Changing one sentence requires the LLM to output the entire page content
2. **Content drift**: LLMs tend to subtly rewrite content when asked to reconstruct a long page, leading to unwanted changes

The existing action dispatch system (`engine.go:executeAction`) uses a simple type switch. Adding a new action type follows the established pattern.

## Goals / Non-Goals

**Goals:**
- Add `patch_page` as a new action type for incremental edits
- Support two operations: heading-based replace and content append
- Keep implementation simple (~60 lines) — no diff engine, no AST parsing
- Update AI prompt to guide when to use `patch_page` vs `update_page`

**Non-Goals:**
- Not building a general-purpose diff/merge engine
- Not supporting line-based or position-based edits
- Not adding client-side (frontend) support — this is purely AI‑side
- Not replacing `update_page` — both coexist

## Decisions

### 1. Heading-based replace, not regex or line-based

**Chosen**: Match `operations` as a JSON array with `{type, target?, content}` objects.
- `replace`: `target` is a markdown heading string (e.g., `## 核心概念`). The engine finds the section from that heading to the next heading of equal or higher level, and replaces it.
- `append`: no `target` needed; content goes at the end.

**Alternatives considered:**
- **Line ranges**: Fragile — content changes shift line numbers.
- **Regex replace**: Powerful but the LLM would need to write valid regex, error-prone.
- **AI generates full page (status quo)**: Simple but wasteful.

**Why heading-based**: Wiki pages are naturally section-structured. Headings are stable anchors that don't shift when other sections change. Simple to implement and reliable.

### 2. Single action with operation array, not per-operation actions

**Chosen**: A single `patch_page` action with an `operations` array, allowing multiple patches in one action.

**Why**: Efficient — one action can fix typos in multiple sections, or replace a section and append a new one, without needing separate action IDs and dependency tracking.

### 3. Content passed as string, not diff

**Chosen**: Each operation carries the full new content for the target section (or the full text to append).

**Why**: The LLM still outputs the section content, but section-size content (~5-20 lines) is vastly smaller than full-page content (~50-200 lines). This balances token savings against implementation complexity.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Heading not found (typo) | Return actionable error including available headings in the page |
| Multi-level headings (### vs ##) | Replace from match to next same-or-higher-level heading — handles nested sections gracefully |
| AI uses patch_page when update_page would be simpler | Prompt guidance: use `patch_page` for targeted edits, `update_page` for major rewrites |
| Operations partially applied | Wrap in a transaction or apply sequentially with rollback on first failure |
