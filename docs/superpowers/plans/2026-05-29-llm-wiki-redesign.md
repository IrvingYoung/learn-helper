# LLM Wiki 改造实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Learn Helper 从面试学习助手改造为 AI 维护的个人知识库（LLM Wiki）

**Architecture:** 彻底替换方案——删除旧功能代码（exercises/learning_records），新增 wiki_pages 表和 Wiki 模块，改造 AI Agent 为单一 wiki-maintainer 角色并支持工具调用和确认流程，前端从多页面路由改为三栏单页面。

**Tech Stack:** Go 1.25 + Chi + sqlc + SQLite (后端), React 19 + Vite + Tailwind + SWR (前端), Claude/DeepSeek API (AI)

---

## 文件结构

### 后端新增文件
- `backend/db/migrations/003_add_wiki_pages.sql` — wiki_pages 表迁移
- `backend/db/migrations/queries.sql` — 新增 Wiki 查询（追加）
- `backend/internal/handler/wiki.go` — Wiki CRUD handler
- `backend/cmd/migrate/main.go` — 数据迁移工具（topics → wiki_pages）

### 后端修改文件
- `backend/cmd/server/main.go` — 替换路由，删除旧模块，新增 Wiki 路由
- `backend/internal/handler/ai.go` — 改造为 wiki-maintainer 角色，新增工具调用
- `backend/internal/ai/provider.go` — 新增 wiki-maintainer System Prompt
- `backend/db/migrations/schema.sql` — 新增 wiki_pages 表定义

### 后端删除文件
- `backend/internal/handler/exercises.go`
- `backend/internal/handler/learning.go`

### 前端新增文件
- `frontend/src/app/wiki/page.tsx` — Wiki 主页面入口
- `frontend/src/components/WikiPage.tsx` — 三栏容器组件
- `frontend/src/components/KnowledgeTree.tsx` — 知识树组件
- `frontend/src/components/ChatPanel.tsx` — 改造后的聊天面板
- `frontend/src/components/PageViewer.tsx` — 页面浏览器组件
- `frontend/src/components/WikiPagePreview.tsx` — 变更预览组件（第二期）

### 前端修改文件
- `frontend/src/App.tsx` — 替换路由
- `frontend/src/components/Layout.tsx` — 简化导航
- `frontend/src/lib/api.ts` — 新增 Wiki API
- `frontend/src/types/index.ts` — 新增 Wiki 类型，删除旧类型

### 前端删除文件
- `frontend/src/app/learn/page.tsx`
- `frontend/src/app/learn/[slug]/page.tsx`
- `frontend/src/app/practice/page.tsx`
- `frontend/src/app/dashboard/page.tsx`
- `frontend/src/components/TopicCard.tsx`
- `frontend/src/components/DifficultyBadge.tsx`
- `frontend/src/components/StatusIcon.tsx`
- `frontend/src/components/ProgressBar.tsx`
- `frontend/src/components/FilterChips.tsx`
- `frontend/src/components/EmptyState.tsx`
- `frontend/src/components/Breadcrumb.tsx`
- `frontend/src/components/AIChatPanel.tsx`

---

## Task 1: 数据库迁移 — wiki_pages 表

**Files:**
- Create: `backend/db/migrations/003_add_wiki_pages.sql`
- Modify: `backend/db/migrations/schema.sql`
- Modify: `backend/cmd/server/main.go` (inline schema)

- [ ] **Step 1: 创建迁移文件**

```sql
-- backend/db/migrations/003_add_wiki_pages.sql
CREATE TABLE IF NOT EXISTS wiki_pages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    title           TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    page_type       TEXT NOT NULL DEFAULT 'entity',
    content         TEXT NOT NULL DEFAULT '',
    tags            TEXT DEFAULT '[]',
    parent_id       INTEGER REFERENCES wiki_pages(id),
    content_status  TEXT NOT NULL DEFAULT 'empty',
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_parent ON wiki_pages(parent_id);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_slug ON wiki_pages(slug);

INSERT OR IGNORE INTO wiki_pages (title, slug, page_type, content_status, sort_order)
VALUES ('概览', 'overview', 'overview', 'published', 0);
```

- [ ] **Step 2: 更新 schema.sql — 追加 wiki_pages 表**

- [ ] **Step 3: 更新 main.go 内联 schema — 在 ai_configs 之后追加 wiki_pages**

- [ ] **Step 4: 验证 — 删除旧 db，启动后端，检查 wiki_pages 表创建**

```bash
cd backend && rm -f learn-helper.db && go run ./cmd/server &
sleep 2 && sqlite3 learn-helper.db ".schema wiki_pages"
```

- [ ] **Step 5: Commit**

```bash
git add backend/db/migrations/003_add_wiki_pages.sql backend/db/migrations/schema.sql backend/cmd/server/main.go
git commit -m "feat(db): add wiki_pages table for LLM Wiki"
```

---

## Task 2: sqlc 查询 — Wiki CRUD

**Files:**
- Modify: `backend/db/migrations/queries.sql`
- Regenerate: `backend/internal/model/`

- [ ] **Step 1: 追加 Wiki 查询到 queries.sql**

新增查询：GetAllWikiPages, GetWikiPageTree, GetWikiPageBySlug, GetWikiPageByID, GetOverviewPage, CreateWikiPage, UpdateWikiPage, UpdateWikiPageContent, DeleteWikiPage, GetWikiPageChildren, CountWikiPages, CountWikiPagesByStatus, GetEmptyWikiPages

- [ ] **Step 2: 运行 sqlc generate**

```bash
cd backend && sqlc generate
```

- [ ] **Step 3: 验证生成的代码包含 WikiPage**

```bash
grep -l "WikiPage" backend/internal/model/*.go
```

- [ ] **Step 4: Commit**

```bash
git add backend/db/migrations/queries.sql backend/internal/model/
git commit -m "feat(db): add sqlc queries for wiki_pages CRUD"
```

---

## Task 3: 后端 Wiki Handler

**Files:**
- Create: `backend/internal/handler/wiki.go`

- [ ] **Step 1: 创建 Wiki handler**

包含：WikiHandler struct, WikiTreeNode, WikiPageResponse, GetWikiTree, GetWikiPageBySlug, GetOverviewPage, CreateWikiPage, UpdateWikiPage, DeleteWikiPage

- [ ] **Step 2: 验证编译**

```bash
cd backend && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handler/wiki.go
git commit -m "feat(handler): add Wiki CRUD handlers"
```

---

## Task 4: 数据迁移工具 — topics → wiki_pages

**Files:**
- Create: `backend/cmd/migrate/main.go`

- [ ] **Step 1: 创建迁移工具**

功能：读取 topics 表，映射字段到 wiki_pages，处理 parent_id 关系，删除旧表

- [ ] **Step 2: 验证编译**

```bash
cd backend && go build ./cmd/migrate
```

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/migrate/main.go
git commit -m "feat(migrate): add topics to wiki_pages migration tool"
```

---

## Task 5: AI Provider 改造 — wiki-maintainer 角色

**Files:**
- Modify: `backend/internal/ai/provider.go`

- [ ] **Step 1: 新增 RoleWikiMaintainer 常量和 System Prompt**

- [ ] **Step 2: 验证编译**

```bash
cd backend && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/ai/provider.go
git commit -m "feat(ai): add wiki-maintainer role and system prompt"
```

---

## Task 6: AI Handler 改造 — 工具调用支持

**Files:**
- Modify: `backend/internal/handler/ai.go`

- [ ] **Step 1: 定义 ToolCall、PendingAction、ChatResponseMeta 类型**

- [ ] **Step 2: 修改 AIChat handler — wiki_maintainer 时注入 wiki 上下文，处理工具调用和确认流程**

- [ ] **Step 3: 新增辅助方法 — buildWikiContext, executeConfirmedActions, updateOverviewPage**

- [ ] **Step 4: 验证编译**

```bash
cd backend && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/ai.go
git commit -m "feat(ai): add tool calling support for wiki-maintainer"
```

---

## Task 7: 后端路由改造

**Files:**
- Modify: `backend/cmd/server/main.go`
- Delete: `backend/internal/handler/exercises.go`
- Delete: `backend/internal/handler/learning.go`

- [ ] **Step 1: 替换路由 — 删除旧路由，新增 Wiki 路由，修复 AI 路由 bug**

- [ ] **Step 2: 删除旧 handler 文件**

```bash
rm backend/internal/handler/exercises.go backend/internal/handler/learning.go
```

- [ ] **Step 3: 验证编译和启动**

```bash
cd backend && go build ./cmd/server && ./server &
curl http://localhost:8080/api/wiki
```

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/server/main.go
git rm backend/internal/handler/exercises.go backend/internal/handler/learning.go
git commit -m "feat(routes): replace old routes with wiki routes, remove exercises/learning handlers"
```

---

## Task 8: 前端类型定义

**Files:**
- Modify: `frontend/src/types/index.ts`

- [ ] **Step 1: 替换类型定义 — WikiPage, WikiTreeNode, PendingAction, 调整 AIRole/AIContextType**

- [ ] **Step 2: Commit**

```bash
git add frontend/src/types/index.ts
git commit -m "feat(types): replace topic/exercise types with wiki types"
```

---

## Task 9: 前端 API 客户端

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: 替换 API 函数 — fetchWikiTree, fetchWikiPage, fetchOverviewPage, streamChat**

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(api): replace topic/exercise API with wiki API"
```

---

## Task 10: 前端组件 — KnowledgeTree

**Files:**
- Create: `frontend/src/components/KnowledgeTree.tsx`

- [ ] **Step 1: 创建知识树组件 — 递归树，状态颜色，展开/折叠**

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/KnowledgeTree.tsx
git commit -m "feat(components): add KnowledgeTree component"
```

---

## Task 11: 前端组件 — PageViewer

**Files:**
- Create: `frontend/src/components/PageViewer.tsx`

- [ ] **Step 1: 创建页面浏览器 — Markdown 渲染，状态标签，空状态提示**

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/PageViewer.tsx
git commit -m "feat(components): add PageViewer component"
```

---

## Task 12: 前端组件 — ChatPanel 改造

**Files:**
- Create: `frontend/src/components/ChatPanel.tsx`

- [ ] **Step 1: 创建改造后的 ChatPanel — 简化角色选择，支持 streamChat generator，确认按钮**

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/ChatPanel.tsx
git commit -m "feat(components): add ChatPanel with confirmation support"
```

---

## Task 13: 前端组件 — WikiPage 三栏布局

**Files:**
- Create: `frontend/src/components/WikiPage.tsx`
- Create: `frontend/src/app/wiki/page.tsx`

- [ ] **Step 1: 创建三栏布局 — KnowledgeTree + ChatPanel + PageViewer，可折叠**

- [ ] **Step 2: 创建页面入口**

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/WikiPage.tsx frontend/src/app/wiki/page.tsx
git commit -m "feat(pages): add Wiki page with three-column layout"
```

---

## Task 14: 前端路由改造

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/Layout.tsx`
- Delete: 多个旧页面和组件

- [ ] **Step 1: 替换 App.tsx — /wiki 和 /settings 两个路由**

- [ ] **Step 2: 简化 Layout.tsx**

- [ ] **Step 3: 删除旧文件**

- [ ] **Step 4: 验证编译**

```bash
cd frontend && npx tsc --noEmit
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/Layout.tsx
git rm frontend/src/app/learn/page.tsx frontend/src/app/learn/[slug]/page.tsx frontend/src/app/practice/page.tsx frontend/src/app/dashboard/page.tsx frontend/src/components/TopicCard.tsx frontend/src/components/DifficultyBadge.tsx frontend/src/components/StatusIcon.tsx frontend/src/components/ProgressBar.tsx frontend/src/components/FilterChips.tsx frontend/src/components/EmptyState.tsx frontend/src/components/Breadcrumb.tsx frontend/src/components/AIChatPanel.tsx
git commit -m "feat(routes): replace multi-page routes with wiki single page, remove old components"
```

---

## Task 15: 集成测试与验证

- [ ] **Step 1: 启动后端，运行数据迁移**

- [ ] **Step 2: 验证 API — curl wiki 和 overview 端点**

- [ ] **Step 3: 启动前端，验证三栏布局和基本功能**

- [ ] **Step 4: 端到端测试 — AI 聊天 → 确认 → 知识树刷新**

- [ ] **Step 5: Commit**

```bash
git commit -m "test: integration test passed for LLM Wiki MVP"
```

---

## Task 16: 清理与文档更新

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: 更新 CLAUDE.md — 项目概述、技术栈、项目结构、MVP 边界**

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for LLM Wiki"
```