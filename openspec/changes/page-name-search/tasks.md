## 1. Search State & UI

- [x] 1.1 Add `searchQuery` state to KnowledgeTree component
- [x] 1.2 Add search icon SVG and input element below the "知识库" header
- [x] 1.3 Add clear button (×) when searchQuery is not empty
- [x] 1.4 Add placeholder text "搜索页面名称..." and basic styling (bg, border, rounded)

## 2. Tree Filtering Logic

- [x] 2.1 Implement `filterTreeByTitle(nodes, query)` function: keeps nodes whose title matches (case-insensitive) AND their ancestors; hides non-matching branches
- [x] 2.2 Apply filtered tree to tree rendering when searchQuery is non-empty
- [x] 2.3 Store original `expandedIds` before search, auto-expand all ancestor nodes during search
- [x] 2.4 Restore original `expandedIds` when search is cleared

## 3. Search State Integration

- [x] 3.1 Show empty state text (e.g. "未找到匹配的页面") when search has no results
- [x] 3.2 Disable draggable on TreeNode during search (skip search-state nodes)
- [x] 3.3 Hide context-menu when search is active (skip rendering TreeNodeMenu)
