# LLM Wiki AI 操作能力增强设计

## 产品定位

人机协作知识库 — AI 是知识的组织者、维护者、运用者。用户可以手动操作，也可以让 AI 自动操作，两者平等协作。

## 核心问题

当前 LLM Wiki 的 AI 操作能力有三个交织的不足：

1. **稳定性** — AI 调用工具不稳定（不调用、重复、循环）
2. **工具能力** — AI 只能单页面 CRUD，不能批量整理、跨页面操作、创建链接
3. **知识理解** — AI 缺乏对知识库整体结构的理解，无法做智能决策

根因：AI 对知识库的认知模型太弱，只看到"一堆页面"，不理解页面间的关系、知识库的整体状态、以及操作的影响范围。

## 解决方案：AI 操作规划层

在 AI 和知识库之间增加规划层。AI 不再直接调用写操作工具，而是生成操作计划（Plan），用户审核确认后，系统确定性执行。

### 和当前确认流程的对比

| 当前 | 新方案 |
|------|--------|
| AI 逐步调用工具，每次写操作都暂停确认 | AI 一次性生成完整计划，用户一次审核确认 |
| 确认后 AI 继续循环，可能重复或偏离 | 确认后系统确定性执行，不需要 AI 参与 |
| 单次只能确认零散的操作 | 批量操作有依赖关系，执行顺序明确 |
| AI 需要理解工具调用格式 | AI 只需要输出结构化的计划 JSON |

## 数据模型

### Plan

```sql
CREATE TABLE plans (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  reasoning TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK(status IN ('pending', 'confirmed', 'executing', 'completed', 'rejected', 'completed_with_failures')),
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  executed_at TEXT
);
```

### Action

```sql
CREATE TABLE plan_actions (
  id TEXT PRIMARY KEY,
  plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
  type TEXT NOT NULL CHECK(type IN ('create_page', 'update_page', 'delete_page', 'link_pages', 'move_page')),
  params TEXT NOT NULL,        -- JSON object, compatible with current tool params
  depends_on TEXT NOT NULL DEFAULT '[]', -- JSON array of action IDs
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK(status IN ('pending', 'running', 'completed', 'failed', 'skipped')),
  result TEXT,                 -- JSON object with execution result or error
  sort_order INTEGER NOT NULL, -- execution order within the plan
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### 链接字段

```sql
ALTER TABLE wiki_pages ADD COLUMN links TEXT NOT NULL DEFAULT '[]';
-- JSON array of linked page IDs: [1, 5, 12]

ALTER TABLE wiki_pages ADD COLUMN backlinks TEXT NOT NULL DEFAULT '[]';
-- JSON array of backlink page IDs: [3, 7]
```

## AI 工具体系

### 只读工具（自动执行，不变）

- `lookup_page` — 按标题查找页面
- `read_page` — 读取页面内容
- `search_pages` — 关键词搜索
- `websearch` — 网络搜索（Tavily）
- `webfetch` — 抓取网页内容

### 写操作工具 → 替换为 `propose_plan`

移除 `create_page`、`update_page`、`delete_page`，替换为：

```
propose_plan {
  reasoning: string,       // 为什么建议这些操作
  actions: [{
    id: string,            // action 唯一标识，用于 depends_on 引用
    type: "create_page" | "update_page" | "delete_page" | "link_pages" | "move_page",
    params: object,        // 操作参数，与当前工具参数兼容
    depends_on: string[]   // 依赖的 action id 列表
  }]
}
```

### 新增操作类型

| 操作 | 参数 | 说明 |
|------|------|------|
| `link_pages` | `source_page_id`, `target_page_id`, `link_text` | 在 source 页面内容中插入 `[[link_text]]` 链接到 target 页面 |
| `move_page` | `page_id`, `new_parent_id` | 移动页面到新的父节点（含子树路径迁移） |

### 现有操作类型（参数不变）

| 操作 | 参数 |
|------|------|
| `create_page` | `title`, `slug`, `parent_id`, `content`, `page_type` |
| `update_page` | `page_id`, `content`, `title` |
| `delete_page` | `page_id` |

## 交互流程

### 多轮规划

Plan 不是一次性的，而是一个迭代过程：

1. **第一轮** — AI 生成初始 Plan，用户确认，系统执行
2. **执行后** — AI 查看执行结果，决定是否需要补充操作
3. **后续轮** — AI 生成新的 Plan（或说"完成了"），用户再确认

每一轮都是"AI 规划 → 用户确认 → 系统执行"，AI 每次只输出一个 Plan，不会进入无限循环。

### 具体流程

```
用户提问
    ↓
AI 调用只读工具探索知识库（自动执行，无需确认）
    ↓
AI 调用 propose_plan 生成操作计划
    ↓
前端展示 Plan：
  - 中栏（聊天）：AI reasoning + "已生成操作计划，请在右侧查看"
  - 右栏（PageViewer）：Plan 预览（操作列表、内容预览、diff）
    ↓
用户确认 / 修改 / 拒绝
    ↓
确认后：系统按依赖顺序执行 actions
    ↓
执行完成：生成执行报告
    ↓
AI 收到执行结果，决定是否继续规划
```

### 场景示例

**用户：** "我想了解一下 Docker"

**AI（只读探索）：**
- `search_pages("Docker")` → 没找到
- `websearch("Docker 入门 核心概念")` → 获取基础知识
- `lookup_page("DevOps")` → 找到已有页面

**AI 生成 Plan：**
```
Plan: "创建 Docker 知识体系"
Reasoning: "知识库中没有 Docker 相关内容。Docker 属于 DevOps 领域，建议在 DevOps 下创建知识体系。"

Actions:
  1. create_page("Docker 概述", parent: DevOps, content: "Docker 是...")
     → id: a1
  2. create_page("Docker 安装", parent: a1, content: "...")
     depends_on: [a1]
  3. create_page("Docker 基本命令", parent: a1, content: "...")
     depends_on: [a1]
  4. link_pages(DevOps → Docker 概述)
     depends_on: [a1]
```

**用户确认后：** 系统执行 a1 → a2, a3, a4 并行 → 知识树实时更新

**AI 回复：** "Docker 知识体系已创建，包含 3 个页面。你可以让我补充更多内容，或者直接编辑页面。"

## 前端布局

### 三栏分工

| 左栏：知识树 | 中栏：聊天 | 右栏：内容/预览 |
|---|---|---|
| 展示知识库层级 | AI 对话 + reasoning | 正常时：显示选中页面内容 |
| Plan 执行后实时更新 | 确认/拒绝按钮 | 有 Plan 时：切换为 Plan 预览 |

### Plan 预览（右侧 PageViewer 区域）

- 顶部：Plan 标题 + reasoning 摘要
- 主体：操作列表，每个操作显示：
  - `create_page`: 标题 + 内容摘要
  - `update_page`: diff（旧内容 → 新内容）
  - `delete_page`: 页面标题 + 确认提示
  - `link_pages`: 链接关系（source → target）
  - `move_page`: 从哪移到哪
- 底部：确认 / 拒绝按钮

### 聊天区（中栏）

- AI 的 reasoning 文本
- "已生成操作计划，请在右侧查看" 提示
- 确认/拒绝按钮（与右侧同步）

## 知识链接系统

### 链接语法

页面内容中使用 `[[页面标题]]` 语法创建链接：

```markdown
Docker 是一个容器化平台，常用于 [[DevOps]] 和 [[微服务]] 部署。
```

### 链接的创建方式

1. **AI 创建** — 通过 `link_pages` action
2. **用户手动创建** — 编辑页面时输入 `[[页面标题]]`，系统自动解析并建立链接

### 链接的展示

- **页面内容中** — `[[页面标题]]` 渲染为可点击链接，点击跳转到目标页面
- **页面底部** — 显示"反向链接"列表：哪些页面链接到了当前页面
- **知识树** — 可选显示链接关系（虚线连接）

### 链接的维护

- 页面删除时，自动清理相关链接和反向链接
- 页面重命名时，自动更新所有引用该页面的链接文本
- AI 在规划时可以建议链接

## 执行引擎

### 执行流程

```
Plan confirmed
    ↓
按依赖关系拓扑排序 actions
    ↓
依次执行每个 action:
    ├── 成功 → 标记 completed，继续下一个
    └── 失败 → 标记 failed，跳过所有依赖此 action 的后续 actions
    ↓
所有 actions 执行完毕
    ↓
生成执行报告
    ↓
通知 AI 执行结果（作为 tool_result 返回）
    ↓
AI 决定是否需要继续规划
```

### 原子性

- 单个 action 是原子的（数据库事务）
- 整个 Plan 不是原子的 — 如果中间失败，已执行的操作不会回滚
- 失败的 action 在执行报告中标注，AI 可以在下一轮 Plan 中修复

### 执行报告格式

```json
{
  "plan_id": "plan-123",
  "status": "completed_with_failures",
  "actions": [
    {"id": "a1", "type": "create_page", "status": "completed", "result": {"page_id": 42}},
    {"id": "a2", "type": "create_page", "status": "completed", "result": {"page_id": 43}},
    {"id": "a3", "type": "link_pages", "status": "failed", "error": "target page not found"},
    {"id": "a4", "type": "update_page", "status": "skipped", "reason": "depends on failed a3"}
  ]
}
```

### ID 替换

Action 的 `depends_on` 允许引用同 Plan 中其他 action 的结果。例如 `create_page` action a1 生成 page_id=42，后续 action a2 的 `parent_id` 可以引用 a1 的结果。执行引擎在运行每个 action 前，将参数中形如 `{{action:a1.page_id}}` 的占位符替换为 a1 的实际执行结果。

### 和现有代码的关系

- 复用 `WikiHandler` CRUD 方法（`CreateWikiPage`, `UpdateWikiPage` 等）
- 新增 `PlanHandler` 管理计划生命周期
- 新增 `ExecutionEngine` 负责按依赖顺序执行 actions
- AI handler 的 ReAct 循环简化：只读工具自动执行 + `propose_plan` 触发确认流程

## AI 知识理解增强

### 增强的上下文注入

AI 的 system prompt 注入以下结构化信息：

1. **知识库概览** — 页面总数、覆盖率、最近更新
2. **当前焦点区域的上下文** — 用户正在查看的页面及其关联页面
3. **链接关系图** — 当前页面链接到了哪些页面、被哪些页面链接
4. **知识空洞提示** — 哪些页面内容为空或过短

### 注入示例

```
当前知识库状态:
- 总页面数: 23
- 有内容的页面: 15 (65%)
- 空页面: 8 (35%)
- 最近更新: Docker 概述 (2分钟前)

当前焦点: DevOps/Docker 概述
- 链接到: 容器技术, Linux 基础
- 被链接: DevOps, CI/CD
- 子页面: Docker 安装, Docker 基本命令 (内容为空)

知识空洞:
- Docker 安装: 内容为空
- Docker 基本命令: 内容为空
- 3 个页面没有链接到任何其他页面
```

## 不改动的部分

- 树状层级结构（保留为主组织方式）
- 只读工具（保留，仍自动执行）
- SSE 流式传输（保留）
- 多 provider 支持（保留）
- 用户确认机制（保留，从逐个确认改为 Plan 级确认）
