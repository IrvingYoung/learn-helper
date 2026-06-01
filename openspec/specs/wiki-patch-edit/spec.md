# Wiki Patch Edit

Incremental editing of wiki page content via patch operations, without sending the full page content.

## Requirements

### Requirement: AI can incrementally edit wiki pages

The AI SHALL be able to edit wiki page content incrementally via the `patch_page` action, without sending the full page content.

#### Scenario: AI replaces a section by heading

- **WHEN** the AI sends a `patch_page` action on page 123 with operation `{"type": "replace", "target": "## 核心概念", "content": "## 核心概念\n\n新的内容..."}`
- **THEN** the system SHALL locate the `## 核心概念` section in the page, replace its content (from that heading to the next heading of equal or higher level) with the provided content, and return success with the updated page.

#### Scenario: AI appends content to a page

- **WHEN** the AI sends a `patch_page` action on page 456 with operation `{"type": "append", "content": "## 新章节\n\n追加的内容"}`
- **THEN** the system SHALL append the content to the end of the page content and return success.

#### Scenario: AI patches multiple sections in one action

- **WHEN** the AI sends a `patch_page` action with operations `[{"type": "replace", "target": "## 背景", "content": "..."}, {"type": "append", "content": "..."}]`
- **THEN** the system SHALL apply all operations sequentially to the page content and return success.

#### Scenario: Heading not found

- **WHEN** the AI sends a `patch_page` action with `type: "replace"` and a `target` heading that does not exist in the page
- **THEN** the system SHALL return an error listing the available headings in the page.

#### Scenario: Invalid operation type

- **WHEN** the AI sends a `patch_page` action with an operation type other than `replace` or `append`
- **THEN** the system SHALL return an error indicating the invalid operation type.

#### Scenario: Patch_page re-parses wiki links after edit

- **WHEN** the AI uses `patch_page` to modify page content that contains or modifies `[[wiki links]]`
- **THEN** the system SHALL re-parse all wiki links from the updated content (same as `update_page` does).
