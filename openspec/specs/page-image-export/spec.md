# page-image-export Specification

## Purpose
TBD - created by archiving change share-page-as-image. Update Purpose after archive.
## Requirements
### Requirement: Share icon button is available on every wiki page
The system SHALL display a compact icon-only "分享" (Share) button in the title block of `PageViewer` whenever a wiki page is loaded, and SHALL hide the button when no page is selected. The button MUST be a 32×32 icon button with no text label, and MUST use the share-with-nodes icon (three dots connected by lines).

#### Scenario: Page loaded with content
- **WHEN** user opens any wiki page with non-empty content
- **THEN** the page title block shows a 32×32 share icon button to the right of the title

#### Scenario: No page selected
- **WHEN** the PageViewer is in the empty state (no page)
- **THEN** no share button is visible

### Requirement: Share button opens dropdown menu
The system SHALL open a dropdown menu anchored below the share icon button when the button is clicked. The menu MUST contain at least one item: "用图片分享" (Share as Image), with an image icon. The menu MUST close when the user clicks outside it or presses Escape.

#### Scenario: Menu opens on click
- **WHEN** user clicks the share icon button
- **THEN** a dropdown menu appears below the button with a "用图片分享" menu item

#### Scenario: Menu closes on outside click
- **WHEN** the menu is open and the user clicks anywhere outside the menu
- **THEN** the menu closes

#### Scenario: Menu closes on Escape
- **WHEN** the menu is open and the user presses Escape
- **THEN** the menu closes

### Requirement: Menu item click opens preview modal with rendered PNG
The system SHALL, on "用图片分享" menu-item click, close the menu and asynchronously generate a PNG of the page content, then display it in a modal with a preview area, file size / dimensions info, and action buttons (copy, download, close).

#### Scenario: Successful generation
- **WHEN** user clicks "用图片分享" and generation completes within 10 seconds
- **THEN** the dropdown menu closes, a modal opens showing the PNG preview, the file's byte size and pixel dimensions, and three action buttons: "复制到剪贴板", "下载 PNG", and "关闭"

#### Scenario: Generation in progress
- **WHEN** PNG generation is in progress
- **THEN** the share menu item shows a loading state and the modal shows a spinner with all action buttons disabled

#### Scenario: Generation fails
- **WHEN** PNG generation throws or exceeds 10s timeout
- **THEN** the modal shows an error message with a "重试" (Retry) button, and the page state remains usable

### Requirement: PNG content matches page content with forced light theme
The system SHALL render the PNG to exactly match the page's markdown body and title block, with a white background, dark text, and a fixed outer width of 800px. The rendered PNG MUST ignore the user's current theme setting (dark or light).

#### Scenario: Page rendered in dark theme
- **WHEN** user has dark theme enabled and clicks share
- **THEN** the resulting PNG has a white background and dark text (not the user's dark theme colors)

#### Scenario: Page contains mermaid diagrams
- **WHEN** the page contains one or more mermaid blocks
- **THEN** the PNG includes the rendered mermaid SVG (not the loading placeholder) for every mermaid block

#### Scenario: Page contains internal links
- **WHEN** the page contains wiki internal links
- **THEN** the PNG includes the link text as plain text (links are rendered as their text content, not as clickable)

### Requirement: Share UI is excluded from exported image
The system SHALL NOT include any share-related UI (the share icon button, the dropdown menu, the menu items, and the page status ribbon) in the exported PNG. These UI elements are tagged with `data-share-ui` and MUST be removed from the clone before the screenshot is taken.

#### Scenario: Menu is open when share is triggered
- **WHEN** the user has the share menu open and clicks "用图片分享"
- **THEN** the exported PNG does not contain the text "用图片分享", "分享", or any other share-menu UI text — only the article content (title, body, backlinks) appears in the image

#### Scenario: Page status ribbon excluded
- **WHEN** a PNG is generated from any page
- **THEN** the bear-style status ribbon ("已发布" / "草稿" / "待填充") does not appear in the exported image

### Requirement: Exported image has share-ready visual polish
The system SHALL add the following visual elements to the exported PNG to make it look like a designed share image rather than a raw DOM screenshot:
- A 4px-tall accent bar (`#c45c26`) at the very top of the image, spanning the full width
- A footer at the bottom containing: brand text "learn-helper" in the accent color, the local date `YYYY-MM-DD`, and the article word count `N 字`, separated by middle dots, with a 1px separator line above
- A 48px padding on all four sides of the image content
- The overall image width SHALL be 800px

#### Scenario: Image includes brand bar
- **WHEN** a PNG is generated
- **THEN** the top 4px of the image is the orange accent color

#### Scenario: Image includes footer
- **WHEN** a PNG is generated
- **THEN** the bottom of the image shows "learn-helper · YYYY-MM-DD · N 字" with a separator line above

#### Scenario: Word count reflects content
- **WHEN** a page with mostly Chinese text is exported
- **THEN** the footer's character count reflects the number of CJK characters in the article

#### Scenario: Word count for English content
- **WHEN** a page with mostly English text is exported
- **THEN** the footer's character count reflects the number of English words (whitespace-separated tokens)

### Requirement: Download action saves a PNG file
The system SHALL provide a "下载 PNG" action in the modal that, on click, triggers a browser download of the PNG with filename `wiki-{slug}-{YYYY-MM-DD}.png`, where `{slug}` is the page's slug and `{YYYY-MM-DD}` is the local date.

#### Scenario: Download triggered
- **WHEN** user clicks "下载 PNG"
- **THEN** browser saves a file named `wiki-{slug}-2026-06-02.png` (date matches current local date)

### Requirement: Copy to clipboard action writes PNG to system clipboard
The system SHALL provide a "复制到剪贴板" action that writes the PNG to the system clipboard using `navigator.clipboard.write`. The action MUST be invoked within a user gesture (click handler) to satisfy browser security requirements.

#### Scenario: Clipboard API available
- **WHEN** the browser supports `navigator.clipboard.write` with image MIME type AND the page is served over HTTPS or localhost
- **THEN** the "复制到剪贴板" button is enabled, and clicking it copies the PNG; on success the button briefly shows a ✓ confirmation

#### Scenario: Clipboard API unavailable
- **WHEN** the browser does not support clipboard image writing (e.g., insecure context, older browser)
- **THEN** the "复制到剪贴板" button is disabled and shows a tooltip "当前环境不支持直接复制,请下载后手动粘贴"

### Requirement: Modal is dismissible
The system SHALL allow the user to close the modal by clicking the "关闭" button, clicking the backdrop, or pressing the Escape key. Closing the modal SHALL NOT discard the generated PNG (it remains in state for re-opening if the user re-clicks share).

#### Scenario: User dismisses modal
- **WHEN** user clicks "关闭", clicks the modal backdrop, or presses Escape
- **THEN** the modal is hidden and the share button returns to its idle state

### Requirement: No backend or schema changes
The system SHALL implement this capability entirely in the frontend. No new API endpoints, database tables, or migrations are introduced.

#### Scenario: Backend unchanged
- **WHEN** the feature is implemented
- **THEN** the backend binary, database schema, and API surface remain identical to the pre-change state

