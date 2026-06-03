# X (Twitter) 每日 AI 热点摘要

| 日期 | 版本 | 变更 |
|------|------|------|
| 2026-06-03 | v1.0 | 初始设计 |

## 概述

为 LLM Wiki 增加「X 推文定时抓取 + AI 聚合分析」功能。用户通过 UI 维护一份"被追踪账号"清单；cron 任务定时通过 RSSHub 拉取这些账号的近期推文，存入本地 `tweets` 表；然后调用 AI 让其读取推文、生成结构化的"AI 日报"，作为 wiki 页面写入知识库。

每天生成一份日报，路径固定在 `AI 日报/YYYY-MM-DD`，由 AI 自己用 `propose_plan` 落到 `wiki_pages`。

## 需求

1. 用户能在 `/settings` 页面增删/启停被追踪的 X 账号
2. 用户能配置 RSSHub Base URL（默认 `https://rsshub.app`，自部署时可改）
3. cron 任务可由用户自由调度（如 `0 9 * * *`），并支持"立即运行一次"
4. 抓推文走 RSSHub（开源、免 X 账号），不直连 X
5. 推文原始数据落 `tweets` 表，按 `tweet_id` 幂等
6. AI 从 `tweets` 表读取本次 run 的推文，生成 趋势洞察 / 主题分类摘要 / 关键引述 三个板块
7. AI 调 `propose_plan` 把日报写到 `AI 日报/{YYYY-MM-DD}` 路径
8. cron 任务的 `auto_approve=true`，跳过 plan 二次确认
9. 当日页面已存在时走 update 而非 create（避免重复页）
10. 抓推文 / AI 分析 失败要在 run history 里能看到原因

## 技术方案

### 1. 数据模型

迁移脚本（追加到 `db/schema.sql` 后由 `cmd/migrate` 应用）：

```sql
CREATE TABLE tracked_twitter_accounts (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  handle        TEXT NOT NULL UNIQUE,           -- 'karpathy'，不带 @
  display_name  TEXT,                            -- 第一次成功抓取时回填，后续可手改
  enabled       INTEGER NOT NULL DEFAULT 1,
  added_at      TEXT NOT NULL DEFAULT (datetime('now')),
  notes         TEXT
);

CREATE TABLE tweets (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  tweet_id      TEXT NOT NULL UNIQUE,             -- X 推文 ID，幂等键
  handle        TEXT NOT NULL,                    -- 冗余便于按账号过滤
  author_name   TEXT,
  text          TEXT NOT NULL,
  created_at    TEXT NOT NULL,                    -- ISO8601 from RSS
  url           TEXT NOT NULL,
  metrics_json  TEXT,                             -- {likes, retweets, replies} 尽力解析
  raw_json      TEXT NOT NULL,                    -- RSSHub 原始 entry，调试用
  fetched_at    TEXT NOT NULL DEFAULT (datetime('now')),
  digest_run_id TEXT                              -- 哪次 cron run 抓到的
);
CREATE INDEX idx_tweets_handle_created ON tweets(handle, created_at);
CREATE INDEX idx_tweets_digest_run ON tweets(digest_run_id);

CREATE TABLE twitter_digest_runs (
  id              TEXT PRIMARY KEY,                -- uuid
  started_at      TEXT NOT NULL,
  finished_at     TEXT,
  status          TEXT NOT NULL,                   -- 'running' | 'fetched' | 'analyzed' | 'failed'
  tweets_fetched  INTEGER DEFAULT 0,
  plan_id         TEXT,                            -- 关联 wiki 日报创建
  error           TEXT
);
```

### 2. Twitter Client 抽象（`internal/twitter/`）

```go
// Client 抽象所有 X 数据源
type Client interface {
    FetchUserTweets(ctx context.Context, handle string, since time.Time, limit int) ([]Tweet, error)
}

type Tweet struct {
    TweetID    string
    Handle     string
    AuthorName string
    Text       string
    CreatedAt  time.Time
    URL        string
    Metrics    map[string]int    // likes, retweets, replies
    Raw        json.RawMessage   // RSSHub 原始 entry
}

// RSSHubClient: 调 GET {BaseURL}/twitter/user/{handle}
//  返回 RSS 2.0，解析每个 <item>
type RSSHubClient struct {
    BaseURL    string            // 来自配置
    HTTPClient *http.Client      // 15s timeout
}
```

`BaseURL` 读取顺序：`ai_configs.config.rsshub_base_url` → 默认 `https://rsshub.app`。配置存进现有 `ai_configs` 表的 JSON 列（同 Tavily key 模式）。

### 3. cron 任务类型

在 `internal/cron/runner.go` 注册新任务类型 `twitter_digest`，任务配置 JSON：

```json
{
  "type": "twitter_digest",
  "schedule": "0 9 * * *",
  "auto_approve": true,
  "config": {
    "since_hours": 24,
    "max_tweets_per_account": 50,
    "max_total_tweets": 200
  }
}
```

执行流程（`runTwitterDigest(ctx, taskID)`）：

```
1. INSERT INTO twitter_digest_runs (status='running')
2. 读 tracked_twitter_accounts WHERE enabled=1
3. 串行抓取（避免 RSSHub 限流）:
     for each account:
       try  { tweets = client.FetchUserTweets(handle, since=now-since_hours) }
       catch { log.Warn; continue }   // 单账号失败不阻断
       INSERT INTO tweets ON CONFLICT(tweet_id) DO NOTHING
       若该账号 display_name 为空且返回 tweets 非空：UPDATE display_name 来自 tweets[0].author_name
4. UPDATE runs SET status='fetched', tweets_fetched=...
5. 若 tweets_fetched=0:
     UPDATE runs SET status='failed', error='no_new_tweets'
     return
6. 调 AI chat（带 auto_approve=true），system prompt 注入「digest mode」指令
   - 复用现有 ReAct 循环，但**禁用前端 SSE 推送**（cron 无人监听）；走一个内部 Go channel 把 `meta` 事件中的 `plan_id` 拿出来
   - 调 ListRe-Act 结束的标志是收到 `propose_plan` tool_use
7. 从 channel 拿到 plan_id，直接调 `engine.ExecutePlan(plan_id)`（auto_approve=true 跳过二次确认）
8. UPDATE runs SET status='analyzed', plan_id=..., finished_at=...
```

**手动触发**：cron 任务详情页 `POST /api/cron/tasks/{id}/run-now` 直接调 `runTwitterDigest`，不依赖 cron 调度器。

### 4. AI 集成

**新 AI 工具 `list_recent_tweets`**（加入 `WikiTools()`）：

```json
{
  "name": "list_recent_tweets",
  "description": "读取已抓取的推文。since: ISO 时间; handle: 可选账号过滤; limit: 默认 50",
  "input_schema": {
    "type": "object",
    "properties": {
      "since":  {"type": "string"},
      "handle": {"type": "string"},
      "limit":  {"type": "integer", "default": 50}
    }
  }
}
```

注册到 `autoTools` map，走自动执行路径（无需用户确认）。返回 tweets JSON 列表。

**digest mode system prompt 注入**（任务触发时由 runner 拼到 system prompt 末尾）：

```
=== 特殊任务：AI 日报生成 ===
1. 调 list_recent_tweets 读取本次 run 抓到的推文（since=本次 run 起始时间）
2. 按以下结构组织日报:
   ## 今日趋势   (3-5 条要点)
   ## 主题讨论   (按主题分组, 每组 2-4 条)
   ## 关键引述   (3-5 条原推 + 背景解读)
3. 通过 propose_plan 创建 wiki 页面:
   - path: "AI 日报/{YYYY-MM-DD}"
   - actions: [{type: "create_page" OR "update_page", title: "AI 日报 · YYYY-MM-DD", content: <上面的 markdown>}]
```

**当日页面已存在**：engine 只有 `create_page` / `update_page` 两种 action，不能合并。AI 在 prompt 里被明确告知流程：

```
1. 调 lookup_page(path="AI 日报/YYYY-MM-DD")
2. 若存在 → propose_plan 的 action 用 type=update_page, page_id=<该页 ID>
3. 若不存在 → action 用 type=create_page, path="AI 日报/YYYY-MM-DD"
```

### 5. UI 改造

**A. `/settings` 页面新增 "推文账号" 面板**

```
推文账号                                       [+ 添加账号]
┌─────────────────────────────────────────────┐
│  ✓ karpathy    添加于 2026-05-12    [⋮]      │
│  ✓ sama        添加于 2026-05-12    [⋮]      │
│  ⠀ ylecun      添加于 2026-05-12    [⋮]      │  ← 已禁用，灰显
└─────────────────────────────────────────────┘
RSSHub Base URL:  [https://rsshub.app          ]  [保存]
```

行菜单：编辑备注 / 启用切换 / 删除（带确认）。

**B. `/cron` 页面**

- 任务类型下拉新增 `AI 日报 (twitter_digest)`
- 选中后展示专属字段：`since_hours`、`max_tweets_per_account`、`max_total_tweets`
- 任务详情页加 "立即运行" 按钮

**C. 知识树**

`AI 日报/` 节点由日报自然形成，无需特殊 UI。

### 6. 新增 API

| Method | Path | 用途 |
|---|---|---|
| GET    | `/api/twitter/accounts`          | 列出被追踪账号 |
| POST   | `/api/twitter/accounts`          | 新增账号（body: `{handle, notes?}`）|
| PUT    | `/api/twitter/accounts/{id}`    | 改 enabled / notes / handle |
| DELETE | `/api/twitter/accounts/{id}`    | 删除账号 |
| GET    | `/api/twitter/config`           | 读 RSSHub URL |
| PUT    | `/api/twitter/config`           | 改 RSSHub URL |
| POST   | `/api/cron/tasks/{id}/run-now`  | 手动触发 cron 任务 |
| GET    | `/api/cron/tasks/{id}/runs`     | 列出该任务历史 runs（已存在则复用）|

## 错误处理

| 失败场景 | 策略 |
|---|---|
| RSSHub 实例不可达 | 整体 run failed，错误写入 `runs.error`；不影响其他 cron |
| 单账号抓取失败 | log warn，跳过该账号继续；`runs.tweets_fetched` 反映成功数 |
| 抓回 0 条推文 | 跳过 AI 步骤，status='failed'，error='no_new_tweets' |
| AI 调失败 / 拒绝生成 | 推文已落库，下次 cron 再读同一批；run 标 'failed' |
| AI 没调 propose_plan | log warning，run 标 'analyzed'（从 run history 排查） |
| RSSHub 的 twitter 路由被 X 屏蔽 | 抛清晰错误，引导用户自部署或换 RSSHub 实例；属于**已知限制** |

## 测试

- `internal/twitter/rsshub_client_test.go` — httptest mock RSSHub 返回 RSS fixture
- `internal/cron/digest_runner_test.go` — 注入 mock Client，验证 (1) 抓推文写库 (2) AI 调用 (3) 单账号失败跳过 (4) 0 推文分支
- `db/migrate` — 在空库上能成功应用新增表
- 前端 — 账号管理页 CRUD / 启停；cron 任务类型下拉

## 文件改动清单

| 文件 | 改动 |
|------|------|
| `db/schema.sql` | 新增三张表 |
| `backend/internal/twitter/rsshub_client.go` | 新建 |
| `backend/internal/twitter/rsshub_client_test.go` | 新建 |
| `backend/internal/twitter/types.go` | `Tweet`、`Client` interface |
| `backend/internal/handler/twitter_account.go` | 新建，账号 CRUD HTTP handler |
| `backend/internal/handler/twitter_account_test.go` | 新建 |
| `backend/internal/handler/ai_tools.go` | 注册 `list_recent_tweets` 工具 |
| `backend/internal/cron/runner.go` | 注册 `twitter_digest` 任务类型 + `runTwitterDigest` 实现 |
| `backend/internal/cron/digest_runner_test.go` | 新建 |
| `backend/internal/handler/cron.go` | 新增 `run-now` 端点（若尚未存在）|
| `backend/cmd/server/main.go` | 装配 `RSSHubClient`（注入 BaseURL）|
| `frontend/src/app/settings/page.tsx` | 新增"推文账号"面板 + RSSHub URL 配置 |
| `frontend/src/components/CronTaskForm.tsx` | 任务类型下拉 + twitter_digest 专属字段 |
| `frontend/src/app/cron/page.tsx` | 任务详情页加"立即运行"按钮 |
| `frontend/src/lib/api.ts` | 新增 twitter 账号 / cron run-now 调用 |

## 已知限制

- **X 屏蔽 RSSHub 的 twitter 路由**：X 多次屏蔽 RSSHub 的 twitter 路由（`/twitter/user/:id` 间歇性 502/403）。公网实例 `rsshub.app` 经常不可用。**强烈建议自部署 RSSHub**，否则功能可能跑不通。
- **抓取量受限**：RSSHub 的 RSS 输出默认 10-20 条，单账号过去 24h 推文全量抓不到；按 `since_hours=24` + `max_tweets_per_account=50` 在多数情况下够用
- **指标字段（likes/retweets）不保证有**：RSSHub 部分路由不返回 metrics，会显示为空
- **不做评论抓取 / 趋势图**：明确 YAGNI
- **不存用户身份信息**：无 X 账号、不需登录；公开数据范围
