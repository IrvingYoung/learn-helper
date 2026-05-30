## Why

The chat panel (center) and page viewer panel (right) have fixed widths, making it difficult for users to customize the layout for their workflow. Users may want a wider chat area for longer conversations or a wider view panel for reading detailed wiki content. This is a common UX pattern in IDEs and split-pane applications.

## What Changes

- Add drag-to-resize functionality between the chat panel and page viewer panel
- Introduce resizable dividers between the three columns (left sidebar, chat, view)
- Persist panel widths to localStorage so user preferences are remembered
- Add minimum width constraints to prevent panels from collapsing too small
- Maintain existing collapse/expand functionality alongside new resize behavior

## Capabilities

### New Capabilities

- `resizable-panels`: Drag-to-resize functionality for the three-column layout with persistent width preferences

### Modified Capabilities

None - this is a new UI capability that doesn't change existing spec-level behavior.

## Impact

- **Frontend Components**:
  - `WikiPage.tsx` - Replace fixed width state with resizable logic, add drag handlers
  - May need a new `ResizablePanels` or `SplitPane` component for reusability

- **CSS/Styling**:
  - Add styles for resize handles/dividers with hover/active states
  - Ensure cursor changes during drag operations

- **Browser APIs**:
  - `localStorage` for persisting panel widths
  - Mouse event handling for drag operations

- **Dependencies**:
  - Option to use a library like `react-resizable-panels` or implement custom drag logic
