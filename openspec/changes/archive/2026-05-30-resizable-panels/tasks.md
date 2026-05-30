## 1. Setup

- [x] 1.1 Install `react-resizable-panels` package (`pnpm add react-resizable-panels` in frontend/)
- [x] 1.2 Add CSS styles for resize handles (hover/active states, col-resize cursor)

## 2. Refactor WikiPage Layout

- [x] 2.1 Replace the three-column flex layout in `WikiPage.tsx` with `PanelGroup` + `Panel` + `PanelResizeHandle` from `react-resizable-panels`
- [x] 2.2 Configure default panel sizes (left 20%, center 50%, right 30%) and min sizes (left 150px, center 300px, right 200px)
- [x] 2.3 Add `autoSaveId="wiki-layout"` to `PanelGroup` for localStorage persistence
- [x] 2.4 Add `PanelResizeHandle` components between panels with styled dividers

## 3. Preserve Collapse/Expand

- [x] 3.1 Use `react-resizable-panels` imperative API (`ref` + `collapse()`/`expand()`) to replace current `leftCollapsed`/`rightCollapsed` state logic
- [x] 3.2 Wire existing collapse/expand toggle buttons to the new imperative panel API
- [x] 3.3 Ensure collapsed panels hide their resize handles

## 4. Verification

- [x] 4.1 Test drag-to-resize between all three panels
- [x] 4.2 Test that panel sizes persist after page reload
- [x] 4.3 Test collapse/expand still works correctly
- [x] 4.4 Test minimum width constraints are enforced
- [x] 4.5 Test divider hover/drag visual feedback
