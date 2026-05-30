## Context

The frontend uses a three-column layout in `WikiPage.tsx`: left sidebar (KnowledgeTree, 280px), center (ChatPanel, flex-1), right (PageViewer, 400px). Panel widths are hardcoded via `useState`. Users can collapse/expand the left and right panels, but cannot resize them by dragging. This is a common pain point when content doesn't fit the fixed widths.

## Goals / Non-Goals

**Goals:**
- Allow users to drag the dividers between panels to resize them
- Persist panel widths to localStorage so preferences survive page reloads
- Maintain existing collapse/expand behavior
- Provide visual feedback during resize (cursor, divider highlight)
- Set reasonable min/max width constraints per panel

**Non-Goals:**
- Vertical resizing (panels only resize horizontally)
- Per-page or per-route panel width preferences (one global layout)
- Double-click to reset to default sizes
- Mobile/touch resize support (desktop only for now)

## Decisions

### Decision 1: Use `react-resizable-panels` library

**Choice**: Use the `react-resizable-panels` npm package instead of custom drag implementation.

**Alternatives considered**:
- **Custom drag implementation**: Would require handling mousedown/mousemove/mouseup, cursor styles, min/max constraints, and localStorage persistence from scratch. More code to maintain and more edge cases (text selection during drag, iframe interference, etc.).
- **`allotment`**: Another good option, but `react-resizable-panels` is lighter weight, has better TypeScript support, and handles persistence natively via `autoSaveId`.

**Rationale**: `react-resizable-panels` provides:
- Built-in keyboard accessibility (arrow keys to resize)
- `autoSaveId` prop for automatic localStorage persistence
- Collapsible panel support (replaces our current collapse logic)
- Imperative API for programmatic collapse/expand
- No layout thrashing (uses CSS resize under the hood)
- Small bundle size (~5KB gzipped)

### Decision 2: Replace current flex layout with PanelGroup/Panel/PanelResizeHandle

**Choice**: Replace the current `flex` layout with `react-resizable-panels`'s `PanelGroup`, `Panel`, and `PanelResizeHandle` components.

**Rationale**: The library's component API maps directly to our three-column layout. Each column becomes a `<Panel>`, and the dividers become `<PanelResizeHandle>` components. This is cleaner than trying to integrate custom resize logic alongside the existing flex layout.

### Decision 3: Store sizes as percentages, not pixels

**Choice**: Use percentage-based sizing (the library's default) rather than pixel-based.

**Rationale**: Percentage sizes are responsive by nature and work correctly when the browser window is resized. The library handles conversion between percentage and pixel internally.

## Risks / Trade-offs

- **[New dependency]** Adding `react-resizable-panels` increases bundle size by ~5KB gzipped → Acceptable for the UX improvement gained.

- **[Layout migration]** Replacing the flex layout could introduce visual regressions → Test all panel states (expanded, collapsed, resized) across screen sizes.

- **[Persistence migration]** Existing users won't have saved panel sizes → Library falls back to default sizes when no localStorage data exists, which matches current behavior.
