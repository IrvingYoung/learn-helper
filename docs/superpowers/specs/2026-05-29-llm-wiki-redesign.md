# LLM Wiki 改造设计

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-05-29 | v1.0 | 初始设计 |

## 概述

将 Learn Helper（面试学习助手）改造为 LLM Wiki（AI 维护的个人知识库）。核心变化：知识载体从固定 DSA 知识图谱变为用户自建知识树，交互从"浏览+练习"变为"纯对话驱动+确认写入"，UI 从多页面路由变为三栏单页面。

### 设计决策

- **迁移策略：** 彻底替换，删除旧功能代码
- **数据迁移：** 自动迁移 topics → wiki_pages，删除 exercises/learning_records
- **AI 角色：** 单一 wiki-maintainer 角色，替代现有三个角色
- **文件上传：** 第一期仅支持粘贴文本
- **确认流程：** 纯对话确认，无按钮
- **操作模式：** 第一期全部通过聊天操作，知识树直接操作放到第二期

## 数据层

### wiki_pages 表

```sql
CREATE TABLE wiki_pages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    title           TEXT NOT NULL,
    slug            TEXT UNIQUE NOT NULL,
    page_type       TEXT NOT NULL DEFAULT 'entity',  -- entity/concept/overview
    content         TEXT NOT NULL DEFAULT '',
    tags            TEXT DEFAULT '[]',
    parent_id       INTEGER REFERENCES wiki_pages(id),
    content_status  TEXT NOT NULL DEFAULT 'empty',   -- empty/draft/published
    sort_order      INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_wiki_pages_parent ON wiki_pages(parent_id);
CREATE INDEX idx_wiki_pages_slug ON wiki_pages(slug);
```

增加 `sort_order` 用于知识树子节点排序。`tags` 用 TEXT 存 JSON 而非 JSON 类型，避免 SQLite 编译选项依赖。

### 概览页面

一条 `page_type = 'overview'` 的 wiki_page 记录，AI 每次写入后自动更新，不经过用户确认。

### 迁移策略

1. 创建 `wiki_pages` 表
2. 迁移脚本：topics → wiki_pages（name→title, description→content, parent_id→parent_id, slug→slug）
3. 删除 topics、exercises、learning_records 表
4. 保留 conversations、messages、ai_configs 表

## 后端架构

### 模块变更

**新增：**
- `handler/wiki.go` — Wiki CRUD（GET 公开，POST/PUT/DELETE 仅 AI Agent 内部调用）
- `model/wiki_models.go` — WikiPage struct（sqlc 生成）
- `repository/wiki_repo.go` — Wiki 查询（sqlc 生成）

**删除：**
- `handler/exercises.go`
- `handler/learning.go`
- model/repository 中 Exercise、LearningRecord 相关代码

**改造：**
- `handler/ai.go` — 新增 wiki-maintainer 角色 + 工具调用能力

### API

| Method | Endpoint | 说明 |
|--------|----------|------|
| GET | `/api/wiki` | 获取完整知识树 |
| GET | `/api/wiki/:slug` | 获取页面内容 |
| POST | `/api/wiki` | 创建页面（AI Agent 内部调用） |
| PUT | `/api/wiki/:id` | 更新页面（AI Agent 内部调用） |
| DELETE | `/api/wiki/:id` | 删除页面（AI Agent 内部调用） |
| POST | `/api/ai/chat` | AI 聊天（支持工具调用和确认流程） |

POST/PUT/DELETE 不对外暴露，只有 AI Agent 在用户确认后调用。

### AI Agent 改造

1. **wiki-maintainer 角色：** System Prompt 定义为"你是用户知识库的管理员，负责理解用户意图、读/写 Wiki、维护概览页"
2. **工具调用能力：** AI 可调用 `get_wiki_tree`、`get_wiki_page`、`create_wiki_page`、`update_wiki_page`、`delete_wiki_page`
3. **确认流程：** AI 生成变更预览后，等待用户输入"确认"才执行写入

## 前端架构

### 页面结构

删除多页面路由，替换为单一 Wiki 页面：

- `/wiki` → WikiPage（三栏布局）
- `/settings` → 保留，用于 AI 配置

### 三栏布局

```
WikiPage
├── KnowledgeTree（左栏）— 可折叠、可拖拽宽
│   ├── 树节点（状态颜色：绿=已填充，黄=部分，灰=空，蓝=选中）
│   └── 概览入口
├── ChatPanel（中栏）— 固定显示
│   ├── 消息列表（含 AI 变更预览的 Markdown 渲染）
│   └── 输入框
└── PageViewer（右栏）— 可折叠、可拖拽宽
    └── Markdown 渲染页面内容
```

### 删除的页面/组件

- `app/learn/`、`app/practice/`、`app/dashboard/` 页面
- `TopicCard`、`DifficultyBadge`、`StatusIcon`、`ProgressBar`、`FilterChips`、`EmptyState`、`Breadcrumb` 组件
- types 中 Exercise、LearningRecord、Topic 相关类型

### 新增组件

- `WikiPage` — 三栏容器，管理折叠/宽度状态
- `KnowledgeTree` — 递归树组件，节点状态颜色
- `ChatPanel` — 基于 AIChatPanel 改造，支持确认流程
- `PageViewer` — Markdown 渲染 + 页面元信息
- `WikiPagePreview` — AI 变更预览（diff 高亮）

### 状态管理

SWR 管理知识树和页面数据，React state 管理三栏布局状态。不引入新状态管理库。

### 顶部导航

```
[Learn Helper]   知识库 | 设置
```

## AI 确认流程

### 流程

```
用户输入意图 → AI 生成变更预览 → 聊天展示预览 → 用户确认/调整/取消 → 执行写入 → 更新概览 → 前端刷新
```

### 后端实现

AI Agent 使用工具调用（tool_use）模式：

1. AI 判断需要写入时，返回 `tool_use` 响应
2. 后端不立即执行，将工具调用参数作为预览返回前端
3. 前端展示预览，等待用户确认
4. 用户确认后，前端发送确认请求，后端执行工具调用
5. 执行完成后，AI 自动更新概览页

### 消息格式扩展

```typescript
interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  pending_actions?: WikiAction[];
  confirmed?: boolean;
}

interface WikiAction {
  type: 'create' | 'update' | 'delete';
  page_id?: number;
  title?: string;
  slug?: string;
  content?: string;
  preview?: string;
}
```

## 概览页面自动维护

### 触发时机

每次 Wiki 写入操作完成后，AI Agent 自动更新概览页面。

### 内容结构

- 统计：总页面数、已填充/空页面、覆盖领域数
- 知识树摘要
- 最近更新列表
- 待填充页面列表

### 实现方式

Wiki 写入完成后，调用 AI 生成概览内容，直接更新 `page_type = 'overview'` 的记录。不经过确认。前端打开知识库时默认请求 `/api/wiki/overview` 在右栏展示。

## 测试策略

- **Handler 层：** Wiki CRUD 每个端点的 HTTP 测试，AI 聊天确认流程集成测试
- **AI Provider：** wiki-maintainer 角色的 mock 测试，工具调用返回格式测试
- **前端组件：** KnowledgeTree 渲染测试，ChatPanel 确认流程交互测试，PageViewer 渲染测试
- **迁移：** topics → wiki_pages 迁移脚本端到端测试

## 错误处理

| 场景 | 处理方式 |
|------|---------|
| AI API 不可用 | 聊天显示错误提示，浏览 Wiki 不受影响 |
| AI 内容质量差 | 用户拒绝确认，要求重新生成 |
| slug 冲突 | 后端返回 409，AI 生成新 slug 重试 |
| 空知识树 | 右栏引导信息，提示开始对话 |

## 性能考量

- 知识树 API 一次返回完整树
- 页面内容按需加载
- AI streaming 继续使用 SSE
- 概览页面更新异步执行，不阻塞写入响应

## 第一期范围

**包含：**
- 三栏布局 UI（可折叠、可拖拽宽）
- AI 聊天创建/更新知识树
- 粘贴文本吸收知识
- 确认流程（预览→确认→写入）
- 概览页面自动维护
- topics 数据自动迁移到 wiki_pages

**不包含（第二期）：**
- 文件上传（.md/.txt/.pdf）
- 知识树直接操作（新建空页面、拖拽移动、右键菜单）
- 全文搜索
- 数据导出
