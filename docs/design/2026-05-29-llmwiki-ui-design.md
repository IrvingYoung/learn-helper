# LLM Wiki UI Design Specification

| 设计负责人 | [TODO] |
| --- | --- |
| 版本 | v1.0 |
| 日期 | 2026-05-29 |
| 状态 | Draft |

---

## 1. Design Tokens

### 1.1 Color Palette

| Token | Color | Hex | Usage |
| --- | --- | --- | --- |
| --color-bg | White | #FFFFFF | Main background |
| --color-bg-secondary | Gray 50 | #F9FAFB | Sidebar/card bg |
| --color-bg-tertiary | Gray 100 | #F3F4F6 | Hover/input bg |
| --color-border | Gray 200 | #E5E7EB | Dividers/borders |
| --color-border-hover | Gray 300 | #D1D5DB | Hover border |
| --color-text-primary | Gray 900 | #111827 | Primary text |
| --color-text-secondary | Gray 500 | #6B7280 | Secondary text |
| --color-text-tertiary | Gray 400 | #9CA3AF | Placeholder/disabled |
| --color-accent | Blue 600 | #2563EB | Primary/selected |
| --color-accent-hover | Blue 700 | #1D4ED8 | Hover state |
| --color-accent-bg | Blue 50 | #EFF6FF | Selected bg |
| --color-node-filled | Green 500 | #22C55E | Node: content filled |
| --color-node-partial | Amber 500 | #F59E0B | Node: partial fill |
| --color-node-empty | Gray 300 | #D1D5DB | Node: empty page |
| --color-node-selected | Blue 600 | #2563EB | Node: selected |
| --color-success | Green 500 | #22C55E | Confirm/success |
| --color-warning | Amber 500 | #F59E0B | Warning |
| --color-danger | Red 500 | #EF4444 | Danger |
| --color-chat-user | Blue 600 | #2563EB | User bubble |
| --color-chat-ai | Gray 100 | #F3F4F6 | AI bubble bg |
| --color-chat-preview | Amber 50 | #FFFBEB | Preview change bg |
| --color-overlay | Black 40% | rgba(0,0,0,0.4) | Overlay |

### 1.2 Typography

| Token | Font | Size | Weight | Line Height |
| --- | --- | --- | --- | --- |
| --font-ui-xs | Inter | 11px | 400 | 16px |
| --font-ui-sm | Inter | 12px | 400 | 16px |
| --font-ui-base | Inter | 13px | 400 | 20px |
| --font-ui-md | Inter | 14px | 500 | 20px |
| --font-ui-lg | Inter | 16px | 600 | 24px |
| --font-heading-sm | Inter | 16px | 600 | 24px |
| --font-heading-md | Inter | 20px | 600 | 28px |
| --font-heading-lg | Inter | 24px | 700 | 32px |

### 1.3 Spacing Scale

| Token | Value |
| --- | --- |
| --space-1 | 4px |
| --space-2 | 8px |
| --space-3 | 12px |
| --space-4 | 16px |
| --space-5 | 20px |
| --space-6 | 24px |
| --space-8 | 32px |
| --space-10 | 40px |
| --space-12 | 48px |

### 1.4 Shadows

| Token | Value | Usage |
| --- | --- | --- |
| --shadow-sm | 0 1px 2px rgba(0,0,0,0.05) | Card |
| --shadow-md | 0 4px 6px -1px rgba(0,0,0,0.10) | Dropdown |
| --shadow-lg | 0 10px 15px -3px rgba(0,0,0,0.10) | Modal |

### 1.5 Border Radius

| Token | Value | Usage |
| --- | --- | --- |
| --radius-sm | 4px | Small elements |
| --radius-md | 6px | Input/button |
| --radius-lg | 8px | Card/panel |
| --radius-xl | 12px | Chat bubble |

---

## 2. Layout

### 2.1 Overall Layout

```
+--------------------------------------------------------------------+
|  Top Navigation Bar (48px)                                        |
+--------+-----------------------------+---------------------------+
| Left   |     Middle (Chat)           |   Right (Browser)        |
| Panel  |     (Flex 1, min 400px)     |   (Flex 2, default 480px)|
| (300px)|                             |                           |
|        |   +-----------------------+ |   +--------------------+  |
| Go (5) |   | [User] Message        | |   | # Page Title       |  |
| +-goro |   | --------------------- | |   | Content...         |  |
|   +-GMP|   | [AI] Response         | |   | ## Section         |  |
|   +-Cha|   |                       | |   | More content...    |  |
| Redis  |   | [Preview Card]        | |   |                    |  |
|        |   |   [Confirm] [Adjust]  | |   | Status: Published  |  |
|        |   | [Input area...]       | |   |                    |  |
+--------+-----------------------------+---------------------------+
```

### 2.2 Column Configuration

| Property | Left Panel | Middle Panel | Right Panel |
| --- | --- | --- | --- |
| Default Width | 300px | Flex (min 400px) | 480px (Flex) |
| Collapsible | Yes | No | Yes |
| Resizable | Yes | Yes | Yes |
| Min Width | 220px | 360px | 320px |
| Max Width | 400px | None | 640px |

### 2.3 Resize Handle

| Property | Value |
| --- | --- |
| Width | 4px |
| Hover Color | --color-accent |
| Default Color | transparent |
| Cursor | col-resize |
---

## 3. Top Navigation Bar

### 3.1 Structure

[ LLM Wiki Logo ] -- [ 🧠 知识库 | 📖 浏览 | 📊 仪表盘 ] -- [ ⚙ Settings ]

### 3.2 Properties

| Property | Value |
| --- | --- |
| Height | 48px |
| Background | --color-bg |
| Border Bottom | 1px solid --color-border |
| Padding | 0 var(--space-4) |
| Logo Font | --font-heading-sm |
| Nav Tab Font | --font-ui-md |
| Active Tab | 2px bottom border, --color-accent |

### 3.3 Navigation Tabs

| Tab | Route | Description |
| --- | --- | --- |
| 知识库 (Knowledge) | /library | Default view, 3-column layout |
| 浏览 (Browse) | /browse | Full-width page browser |
| 仪表盘 (Dashboard) | /dashboard | Knowledge stats overview |

---

## 4. Left Panel - Knowledge Tree

### 4.1 Node States

| State | Dot | Text | BG | Condition |
| --- | --- | --- | --- | --- |
| Filled | Green #22C55E | --color-text-primary | transparent | content_status=published |
| Partial | Amber #F59E0B | --color-text-primary | transparent | content_status=draft |
| Empty | Gray #D1D5DB | --color-text-tertiary | transparent | content_status=empty |
| Selected | Blue #2563EB | --color-accent | --color-accent-bg | Active click |
| Hover | Same as state | Same | --color-bg-tertiary | Mouse hover |

### 4.2 Tree Header

| Element | Type | Action |
| --- | --- | --- |
| Title | Text + icon | Panel title |
| Collapse | Button | Toggle panel visibility |
| Search | Input | Filter tree nodes by title |
| Add | Button (+) | Create new top-level topic |

### 4.3 Node Specifications

| Property | Value |
| --- | --- |
| Height | 32px |
| Horizontal Padding | var(--space-2) |
| Indent per Level | 16px |
| Status Dot Size | 8px |
| Expand Arrow Size | 12px |
| Child Count Badge | Text in parentheses |
| Page Count Display | Title (N) -- total descendants |

### 4.4 Context Menu (Right Click)

| Menu Item | Behavior |
| --- | --- |
| 新建子页面 (New Child) | Create empty page, inline title input |
| 重命名 (Rename) | Inline text edit |
| 删除 (Delete) | Confirm toast, then remove |
| 移动到 (Move to) | Modal to select target parent |

---

## 5. Middle Panel - AI Chat

### 5.1 Message Types

| Type | Alignment | Background | Border Radius |
| --- | --- | --- | --- |
| user | Right | Blue 600 (#2563EB) | 16px 16px 4px 16px |
| ai | Left | Gray 100 (#F3F4F6) | 4px 16px 16px 16px |
| system | Center | None | None |
| preview | Full width | Amber 50 (#FFFBEB) border | 8px |

### 5.2 Change Preview Card

┌─── Pending Changes ─────────────────────┐
| 📝 Will execute:                       |
| ● CREATE: topic/subtopic               |
| ● UPDATE: topic (add section)           |
|                                          |
| Impact: domain (5 -> 8 pages)            |
| Coverage: 60% -> 75%                     |
|                                          |
| [Adjust]  [Reject]  [Confirm]             |
└──────────────────────────────────────────┘

### 5.3 Input Area

| Element | Description |
| --- | --- |
| Text Input | Auto-resize textarea, placeholder "Input message..." |
| Upload Button | File upload (.md/.txt/.pdf, <= 5MB) |
| Knowledge Toggle | Toggle for KB search mode |
| Send Button | Send message |
| Drag & Drop | Visual overlay for file drops |
---

## 6. Right Panel - Page Browser

### 6.1 Header Elements

| Element | Position | Action |
| --- | --- | --- |
| Back Button | Top-left | Return to tree navigation |
| Bookmark | Top-right | Toggle bookmark |
| More Menu | Top-right | Context actions dropdown |
| Breadcrumb | Below header | Parent > Current page path |
| Title | Bold heading | Page title |
| Status Badge | Below title | published/draft/empty |
| Tags | Below title | Topic tags display |

### 6.2 Page States

| State | Badge | Color |
| --- | --- | --- |
| Published | Published | Green (#22C55E) |
| Draft | Draft | Amber (#F59E0B) |
| Empty | Empty | Gray (#D1D5DB) |

### 6.3 Empty Page State

When a page has no content, display:
- Large centered empty state icon
- "This page has no content yet" message
- Hint: "Tell AI in chat to write content"

### 6.4 Overview Page Content Blocks

The overview page (auto-maintained by AI) includes:
- Total page count and content coverage percentage
- Domain distribution (area : filled/total)
- Recently updated pages list
- Empty pages needing content
- Auto-generated by AI, updated after every write

---

## 7. Dashboard View

### 7.1 Stats Cards

4-card row at top:
- Total Pages (count)
- Content Coverage (percentage)
- Pages Pending (count)
- Topics Active (count)

### 7.2 Domain Coverage

Horizontal progress bars for each knowledge domain:
- Bar width = coverage percentage
- Color = filled portion in accent, remainder in gray
- Label = domain name + "N/M pages"

### 7.3 Recent Activity

Timeline list of recent changes:
- CREATE: Page Title - time ago
- UPDATE: Page Title - time ago
- Each entry with relative timestamp

---

## 8. Browse View

### 8.1 Layout

Two-column layout:
- Left: Knowledge tree (320px, collapsible)
- Right: Full-page content view (flex)
- Same tree component as library view
- Full-width Markdown rendering on the right

---

## 9. Component Library

### 9.1 Buttons

| Variant | BG | Text | Border Radius | Height |
| --- | --- | --- | --- | --- |
| Primary | --color-accent | White | --radius-md | 32px |
| Secondary | transparent | --color-text-primary | --radius-md | 32px |
| Danger | --color-danger | White | --radius-md | 32px |
| Ghost | transparent | --color-text-secondary | --radius-md | 32px |

### 9.2 Search Input

| Property | Value |
| --- | --- |
| Height | 32px |
| Border | 1px solid --color-border |
| Radius | --radius-md |
| Focus Border | --color-accent |
| Icon | Search icon at left |

### 9.3 Scrollbar

| Property | Value |
| --- | --- |
| Width | 6px |
| Thumb Color | --color-border |
| Thumb Hover | --color-border-hover |
| Track | transparent |

---

## 10. Interaction Patterns

### 10.1 Tree Operations

| Action | Trigger | Behavior |
| --- | --- | --- |
| Select | Click | Highlight blue, load content right |
| Expand/Collapse | Click arrow | Toggle children visibility |
| Add Child | + button on hover | Inline input, Enter to confirm |
| Rename | Right-click menu | Inline edit, Enter to confirm |
| Delete | Right-click menu | Confirm toast, remove node |
| Drag Move | Drag and drop | Visual indicator, reposition |
| Collapse Panel | Header button | Panel slides away |

---

## 11. Responsive Breakpoints

| Breakpoint | Width | Layout |
| --- | --- | --- |
| Desktop | > 1200px | Full three columns |
| Tablet | 768-1200px | Left collapsed, two columns |
| Mobile | < 768px | Single column (chat), drawer panels |

---

## 12. Accessibility

- All interactive elements focusable via Tab
- Color-coded nodes accompanied by text labels
- Keyboard shortcuts: Ctrl+N (new topic), Ctrl+/ (search tree)
- ARIA labels on collapsible panels and tree nodes
- Visible focus ring on all interactive elements

---

## 13. Data Flow

### 13.1 Wiki Tree Loading

1. App mounts -> GET /api/wiki -> full tree
2. Tree flattens to list with depth info -> recursive render
3. Click node -> GET /api/wiki/:slug -> content in right panel
4. Chat confirm -> optimistic update -> API call -> reconcile

### 13.2 Confirmation Flow Sequence

1. User types request -> frontend sends to AI Agent via chat
2. AI generates preview -> returns structured diff
3. Frontend renders preview card in chat
4. User clicks confirm -> frontend calls API -> AI writes pages
5. Success -> tree refreshes -> overview updated -> panels sync
6. Error -> error message in chat, no changes persisted

## Visual Layout

### 2.4 Three-Column Layout Diagram

```mermaid
graph LR
    subgraph App[LLM Wiki - Full App Layout]
        subgraph Nav[Top Navigation Bar 48px]
            Logo[LLM Wiki Logo]
            Tabs[Knowledge | Browse | Dashboard]
        end

        subgraph Main[Main Content Area]
            subgraph Left[Left Panel - 300px]
                L1[Knowledge Tree]
                L2[Go (5 pages)]
                L3[  goroutine (3)]
                L4[    GMP Model]
                L5[    Channel]
                L6[  Data Types]
                L7[Redis (2 pages)]
            end

            subgraph Middle[Middle Panel - Flex 1]
                M1[Chat Messages]
                M2[User: I want to learn...]
                M3[AI: Here is an outline...]
                M4[Preview Card [Confirm]]
                M5[Input Area]
            end

            subgraph Right[Right Panel - Flex 2]
                R1[Page Content]
                R2[Markdown Rendering]
                R3[Status & Tags]
            end
        end
    end

    Nav --> Main
    Left --> Middle
    Middle --> Right
```
