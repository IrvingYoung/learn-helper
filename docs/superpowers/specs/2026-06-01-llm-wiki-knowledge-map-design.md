---
name: llm-wiki-knowledge-map-design
description: Knowledge map for the AI - per-page summaries, virtual index, write timeline
---

# LLM Wiki 知识地图设计

## 背景

### 当前 AI 看到的知识库

`buildWikiContext` (`internal/handler/ai.go:1032`) 把整棵树以裸骨架形式塞进 system prompt：

```
[FOLDER] [ID=1] 概览 (overview, 有内容, path="1/")
  [PAGE] [ID=2] 数据结构 (entity, 有内容, path="1/2/")
    [EMPTY] [ID=3] 数组 (entity, 空, path="1/2/3/")
    [PAGE] [ID=4] 链表 (entity, 有内容, path="1/2/4/")
```

每页只有标题 + 类型 + 状态 + 路径。AI 想知道"链表这页是关于什么的"，只能调 `read_page` 把全文读进来——一次回答要花 5-10 个工具调用和几千 token。

### 核心痛点

1. **AI 看不见知识库的"肉"**。它知道有哪些页、什么状态，但不知道每页是讲什么的。
2. **搜索是 LIKE 模糊匹配**。问"我了解机器学习吗"，AI 得先猜关键词（"ML"、"神经网络"、"深度学习"都可能命中也可能漏）。
3. **没有可读的时间线**。`messages` 表存了对话但人眼无法浏览，AI 也读不到"最近一周我做了什么"。
4. **健康检查只查结构不查内容**。`analyzeTreeHealth` 抓得到孤儿页和重复标题，抓不到"这页说 A 那页说 B 但其实矛盾"。
5. **好的回答不归档**。AI 给出的对比表、分析、总结散在聊天里，下一次再问同样的问题得从头来。

### 借鉴：Karpathy LLM Wiki 模式

Karpathy 在 [gist 442a6bf5](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) 描述的模式里有两个核心文件：

- **`index.md`**：按分类组织的目录，每页一行摘要。LLM 每次回答先读它，再钻进具体页。"Works surprisingly well at moderate scale (~100 sources, ~hundreds of pages) and avoids the need for embedding-based RAG infrastructure."
- **`log.md`**：可读的时间线，每条操作一行。前缀统一后可 grep。

我们已经有 SQLite，有 AGENTS.md，有 wiki_pages 表。**已经 80% 在按这个 pattern 走了，缺的就是这两个文件。**

### 设计目标

让 AI 在每次对话开始时看到的是"知识地图"而不是"裸树骨架"——按分类聚合、每页带摘要、附带最近活动——这样它能直接回答"我学过什么"、"这个主题覆盖到什么程度"、"该补什么"，不需要先做 5 次 `read_page`。

## 设计

### 三块增量

| 模块 | 类型 | 作用 |
|---|---|---|
| **每页摘要** | 存储 | `wiki_pages.summary` 列，AI 异步生成，content_hash 校验 |
| **知识地图** | 视图 | 不存储。每次 AI 请求时从 `wiki_pages` + 摘要渲染成 system prompt 的一段 |
| **写入日志** | 存储 | `wiki_log` 表。同步写入，按时间线渲染 |

外加未来两件事（这次**只设计、不实现**，单开 spec）：

- **Query → 回写**（"好的回答归档为新页面"）
- **语义 Lint**（矛盾/过时/孤页/缺主题）

### 一致性原则：index 是 view，不是 store

Karpathy 模式里 index.md 是个文件，所以 LLM 写完容易和实际页脱节。**我们用 SQLite，不需要重复制造同步问题**——index 不存，是从源数据派生的纯函数：

```
knowledge_map = render(wiki_pages, wiki_page_summaries, wiki_log)
```

每次 AI 请求都重新渲染。summary 是唯一昂贵的存储（AI 调用的产物），单独处理。

## Schema 变更

### Migration 008：摘要列

```sql
ALTER TABLE wiki_pages ADD COLUMN summary TEXT NOT NULL DEFAULT '';
ALTER TABLE wiki_pages ADD COLUMN summary_status TEXT NOT NULL DEFAULT 'empty';
-- status: 'empty' | 'pending' | 'ready' | 'failed'
ALTER TABLE wiki_pages ADD COLUMN summary_generated_at DATETIME;
ALTER TABLE wiki_pages ADD COLUMN summary_content_hash TEXT;
-- MD5 of content+title, 用于检测"页面改了但摘要没跟上"

-- 反范式：链接/反链计数（避免每次 context 渲染都做 JOIN）
ALTER TABLE wiki_pages ADD COLUMN link_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE wiki_pages ADD COLUMN backlink_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE wiki_pages ADD COLUMN tags_normalized TEXT NOT NULL DEFAULT '';
-- 标签 JSON 数组去重 + 小写化后的字符串

CREATE INDEX IF NOT EXISTS idx_wiki_pages_summary_status ON wiki_pages(summary_status);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_tags_normalized ON wiki_pages(tags_normalized);
```

`content_hash` 用 MD5，存 `hash = MD5(content + title)`——只改标题也要重生成摘要。

`link_count` / `backlink_count` 在 `link_pages` / `delete_page` / `update_page` 时同步维护（已有 `links`/`backlinks` JSON 列）。

`tags_normalized` 在 `update_page` 时由 handler 计算（split by `,` → trim → lowercase → sort → join `,`），写入此列。system prompt 渲染时按这个列做索引。

### Migration 009：写入日志

```sql
CREATE TABLE wiki_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  action TEXT NOT NULL,                 -- 'create' | 'update' | 'delete' | 'move' | 'rename'
  page_id INTEGER,                      -- 操作目标页（删除后为 NULL 但 title 仍可读）
  page_title TEXT NOT NULL,             -- 标题快照
  page_path TEXT,                       -- 路径快照
  source TEXT NOT NULL DEFAULT 'plan',  -- 'plan' | 'manual' | 'lint' | 'query_filing'
  summary TEXT,                         -- 一句话描述这次操作
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_wiki_log_created_at ON wiki_log(created_at DESC);
CREATE INDEX idx_wiki_log_page_id ON wiki_log(page_id);
```

**同步写入，0 延迟，永远一致。** 每次 `INSERT`/`UPDATE`/`DELETE` 页面，handler 在同一事务里写一条 log。

### 兼容性

- 旧数据：现有页面 `summary_status = 'empty'`，第一次写入后自动重生成
- 启动时一次性回填：批量把所有 `summary_status='empty'` 的页面标记为 `pending`，由后台 worker 慢慢生成

## 架构

### 写流程

```
plan confirm / manual edit
  |
  v
DB 事务：
  1. INSERT/UPDATE/DELETE wiki_pages
  2. INSERT INTO wiki_log (...)
  3. 如果是 content 变化：UPDATE wiki_pages SET summary_status='pending', summary_content_hash=NULL
  4. 如果是 link 变化：UPDATE wiki_pages SET link_count=..., backlink_count=...
  |
  v
COMMIT
  |
  v
go summaryWorker.Enqueue(pageID)  // 非阻塞，channel buffer 100
```

`summaryWorker` 是 in-process goroutine：

```go
type SummaryWorker struct {
    db       *sql.DB
    queries  *model.Queries
    provider ai.AIProvider
    queue    chan int64
}

func (w *SummaryWorker) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case pageID := <-w.queue:
            w.generateOne(ctx, pageID)
        }
    }
}

func (w *SummaryWorker) generateOne(ctx context.Context, pageID int64) {
    page, err := w.queries.GetWikiPageByID(ctx, pageID)
    if err != nil { return }

    if page.Content == "" {
        w.queries.MarkSummaryEmpty(ctx, pageID)
        return
    }

    newHash := md5Hash(page.Content + page.Title)
    if page.SummaryContentHash.Valid && page.SummaryContentHash.String == newHash {
        return
    }

    prompt := fmt.Sprintf(
        "为以下 Wiki 页面生成 1-2 句中文摘要（50-150 字），说明这页讲什么、" +
        "适合谁看。不超过 200 字。\n\n标题：%s\n内容：%s",
        page.Title, truncate(page.Content, 3000),
    )
    summary, err := w.provider.Chat(ctx, ai.ChatRequest{Messages: ..., MaxTokens: 256})
    if err != nil {
        w.queries.MarkSummaryFailed(ctx, pageID, err.Error())
        return
    }

    w.queries.UpdateSummary(ctx, model.UpdateSummaryParams{
        PageID: pageID,
        Summary: summary,
        Status: "ready",
        ContentHash: newHash,
    })
}
```

**重试策略**：单页最多连续 3 次失败，标 `failed` 不再重试。**任何后续写入**会重新触发。失败日志写到 `wiki_log`（source='summary_gen'）。

**任务丢失**：进程崩溃后 channel 里的 pending 任务丢失。无所谓——下次任何写入都会重新触发。但**会启动时回填**：扫一遍 `summary_status='pending'`，重新入队。

### 读流程：渲染知识地图

`buildWikiContext` 重写为 `buildKnowledgeMap`：

```go
func (h *AIHandler) buildKnowledgeMap(ctx context.Context, focusPageID *int64) string {
    var b strings.Builder

    // === 1. 知识库概览 ===
    b.WriteString(renderOverview(ctx, h.queries))

    // === 2. 知识地图（核心：分类聚合的树）===
    b.WriteString(renderKnowledgeMap(ctx, h.queries, focusPageID))

    // === 3. 最近活动时间线 ===
    b.WriteString(renderRecentLog(ctx, h.queries, 7*24*time.Hour, 20))

    // === 4. 结构健康检查（保留 + 增强）===
    b.WriteString(renderHealthCheck(ctx, h.queries))

    // === 5. 知识缺口（按一级分类分组）===
    b.WriteString(renderKnowledgeGaps(ctx, h.queries))

    return b.String()
}
```

#### 2. 知识地图渲染

按一级分类（path 不含 `/` 的 overview 节点）聚合，二级及以下展开：

```
【知识地图】

[FOLDER] 数据结构与算法 (overview, 12/15 已建, ID=2)
  摘要：系统的数据组织与处理方法，涵盖线性结构、树、图及经典算法

  [PAGE] 数组 (有内容, 8 反链, 标签: 数据结构/线性表, ID=12)
    摘要：线性表基础，O(1) 随机访问，连续内存存储
  [PAGE] 链表 (有内容, 3 反链, 标签: 数据结构/线性表, ID=13)
    摘要：节点+指针+遍历，重点是反转、合并、环检测
  [EMPTY] 二叉树 (摘要待更新, ID=14)

[FOLDER] Go 语言 (overview, 5/8 已建, ID=20)
  摘要：Go 语法、并发、Web 开发相关内容
  ...

【全局标签索引】
#数据结构 (3 页) · #算法 (5 页) · #Go (4 页) · #系统设计 (2 页)
```

**每页展示字段**：
- 标题、状态、ID、page_type
- 摘要（按 `summary_status` 降级显示）
- 反链数（`backlink_count`）
- 标签
- 路径（focus 模式或用户问"这个页在哪"时显示）

**降级策略**（一致性保证）：

| summary_status | hash 匹配 | 渲染 |
|---|---|---|
| `ready` | 是 | `summary` 完整文本 |
| `ready` | 否（页被改了） | `content` 前 80 字 + "(摘要待更新)" |
| `pending` | - | `content` 前 80 字 + "(摘要待更新)" |
| `failed` | - | `content` 前 80 字 + "(摘要生成失败)" |
| `empty` | - | `content` 前 80 字 + "(暂无摘要)" |

**关键不变量**：AI 永远不会看到"比真实页面更新"的信息，只可能"更陈旧"——陈旧会被 hash 校验抓到并降级。

#### 3. 最近活动时间线

```
【最近活动】(过去 7 天，共 23 条操作)

- 2026-06-01 14:30 [create] 数据结构与算法 (ID=12)
- 2026-06-01 14:25 [update] 链表 (ID=13) · 补充了环检测部分
- 2026-06-01 10:15 [create] Go 语言 (ID=20)
- 2026-05-31 22:00 [update] 概览 (ID=1) · 自动更新
- ...
```

来源：`wiki_log` 表 `WHERE created_at > now - 7 days` ORDER BY created_at DESC LIMIT 20。

#### 4. 结构健康检查（增强）

保留现有 `analyzeTreeHealth` 的所有检查。新增：
- **死页警告**：有内容 + 0 反链 + 0 出链 的页标记 `severity=warning`
- **标签缺失**：有内容但 `tags_normalized=''` 的页（按比例抽样报，不全列）
- **覆盖率**：按一级分类显示"X/Y 已建"，Y = 该分类下所有子页数

#### 5. 知识缺口（按分类分组）

```
【知识缺口】

[FOLDER] 数据结构与算法: 3 个空页（数组 / 链表 / 二叉树）
[FOLDER] Go 语言: 1 个空页（并发编程）
... 其他 8 个分类有空页
```

来源：扫所有 `content_status='empty'` 的页，按 `parent_id` 一级分类分组。

## System Prompt 调整

`buildWikiMaintainerPrompt` 中关于"目录"的段落重写：

**旧**：
> 你的职责是管理知识树，包括创建、更新、删除页面和建立页面间的链接。

**新**：
> 你的 system prompt 里包含"知识地图"——按一级分类组织的目录，每页带 1-2 句摘要。
> 
> **使用规则**：
> 1. 回答前先看地图，定位相关分类，再钻到具体页
> 2. 用户问"我了解 X 吗"时，先在地图里找 X 相关的分类和页，再读具体页
> 3. 摘要可能标"待更新"或"生成失败"——这种时候用 `read_page` 工具读全文
> 4. 全局标签索引帮你做跨分类检索
> 5. "最近活动"告诉你用户最近在学什么、改了什么

## 实现顺序

### Phase 1：Schema + 摘要 Worker

- Migration 008、009
- `SummaryWorker` 实现 + 启动逻辑
- 启动时回填：扫 `summary_status='empty'`，批量标 `pending` 入队
- 不动 `buildWikiContext`

### Phase 2：Knowledge Map 渲染

- `buildKnowledgeMap` 替换 `buildWikiContext`
- `renderKnowledgeMap` 实现
- `renderRecentLog` 实现
- `renderHealthCheck` 增强
- `renderKnowledgeGaps` 按分类分组
- `buildWikiMaintainerPrompt` 中"目录"段重写
- 前端不动（system prompt 内容变化对用户透明）

### Phase 3：Log 写入集成

- `engine/engine.go` 中所有 action 执行成功后写 `wiki_log`
- `handler/wiki.go` 中手动编辑（rename/move/update/create）也写 `wiki_log`
- 删除页面：page_id 设为 NULL，title 快照保留

### Phase 4：标签和链接计数维护

- `update_page` / `create_page` handler 计算 `tags_normalized` 写入
- `link_pages` action 成功后更新源和目标的 `link_count` / `backlink_count`
- `delete_page` 同步减少相关页的 `backlink_count`

## 文件变更预估

### 后端

| 文件 | 变更类型 | 说明 |
|---|---|---|
| `db/migrations/008_summary_columns.sql` | 新增 | 摘要相关列 |
| `db/migrations/009_wiki_log.sql` | 新增 | 写入日志表 |
| `db/migrations/queries.sql` | 中改 | 新增 UpdateSummary / MarkSummaryEmpty / MarkSummaryFailed / ListPendingSummaries / GetRecentLog / GetKnowledgeGaps 等 |
| `internal/worker/summary.go` | 新增 | SummaryWorker |
| `internal/worker/summary_test.go` | 新增 | 单元测试 |
| `cmd/server/main.go` | 小改 | 启动 SummaryWorker，传 context |
| `internal/handler/ai.go` | 中改 | `buildWikiContext` → `buildKnowledgeMap`；新增 4 个 render 函数 |
| `internal/handler/wiki.go` | 小改 | update/create 时计算 tags_normalized 和 content_hash；写 wiki_log |
| `internal/engine/engine.go` | 中改 | 所有 action 执行后写 wiki_log；link_pages 维护计数 |
| `internal/ai/provider.go` | 小改 | system prompt 中"目录"段重写 |
| `internal/model/queries.sql.go` | 重新生成 | sqlc 重新生成 |

### 前端

无需改动。system prompt 变化对用户透明，`wiki_log` 是后端能力，不直接展示给用户。

## 风险和缓解

1. **AI 调用成本**
   - 风险：每个新建/更新页都触发一次 AI 调用
   - 缓解：worker 串行处理、限速（每 200ms 一次）；空页不生成；同一页短时间内多次写入只触发一次（按 pageID 去重 channel）

2. **Worker 队列积压**
   - 风险：批量导入时队列堵
   - 缓解：channel buffer 100；超出的 pageID 在数据库里保持 `pending` 状态，下次启动或写入时重新入队

3. **AI 调用失败影响主流程**
   - 风险：网络抖动 / provider 限流时 summary 一直生成失败
   - 缓解：失败标 `failed`，**完全不影响主流程**；下一次写入时重新入队

4. **System Prompt 变长**
   - 风险：知识地图比裸树长，可能挤压其他内容
   - 缓解：渲染时按 token 数预估截断；一级分类不超过 30 个时全展开，超过则只展开用户最近访问过的 5 个分类

5. **summary 的"摘要质量"**
   - 风险：AI 生成的摘要可能不准确、过长、风格不一致
   - 缓解：prompt 明确要求 50-150 字、中文、客观；用同一个 chat 工具（不调外部大模型）；失败重试时换 prompt 措辞

6. **回填成本**
   - 风险：旧库一次性回填所有页面摘要，AI 调用次数 = 总页数
   - 缓解：启动后分批回填（每批 10 个，间隔 1 秒）；用户可禁用回填（环境变量 `SKIP_SUMMARY_BACKFILL=1`）

## 成功指标

- AI 第一次回答"我了解 X 吗"这类问题的工具调用次数从 ~5 降到 ~1-2
- 摘要生成成功率 > 95%（重试后）
- `wiki_log` 表写入成功率 100%（与页面写入同事务）
- 用户在聊天中问"我最近学了什么"的回答准确度（人工评估）：摘要注入后明显提升
- System prompt 中"知识地图"段平均长度 < 2000 token（一级分类 ≤ 20 时）

## 未来 spec（不在本次范围）

- **Query → 回写**：好的回答归档为新页面。UX 需要单独设计（什么时候问用户"要不要存"、归档到哪个父页面、如何避免重复）。
- **语义 Lint**：用 LLM 检查跨页矛盾、过期信息、缺主题。tool 实现 + 触发时机（手动/自动）单独设计。
- **qmd 集成**：wiki 增长到几百页后考虑 BM25 + 向量混合搜索。karpathy 推荐的 [qmd](https://github.com/tobi/qmd) 可作为 CLI 子进程。
