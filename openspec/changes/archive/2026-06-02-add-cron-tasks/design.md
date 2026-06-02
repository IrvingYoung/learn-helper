## Context

**当前架构**（相关部分）：

- `AIHandler.AIChat`（`internal/handler/ai.go`, 1265 行）实现 ReAct 循环：调 `provider.StreamChat` → 分类 read/write/ask → 读工具自动执行 / 写工具走 permission channel（`permissions.Register`） / ask_user 走 askUser channel → 全部通过 `sseWrite*` 把事件推给 HTTP 客户端
- 系统 prompt 是 100+ 行的 `buildWikiMaintainerPrompt`，包含"先问再写"等人类协作条款
- 后端只有一个 worker（`internal/worker/summary.go`）— goroutine + channel 模式，做事件触发的 AI 摘要
- 没有任何调度器 / ticker / cron 库
- AI 工具集 `WikiTools()` 包含 6 读 + 6 写 + ask_user，全部走相同 channel
- plans / plan_actions 表在 migration 012 被 DROP — 当前写流程是"AI 调 create_page 等 → 用户在 PlanPreview 点确认 → engine 执行"

**关键约束**：
- 单用户本地应用，server 重启不频繁但会重启（DB-backed 状态是必须的）
- `auto_approve=true` 是默认（用户决策）— 写工具在 cron 模式下要直接执行
- 项目核心规则"所有 AI 写要确认"在 cron 场景被有意放宽，用户在创建任务时显式 opt-in
- 后端有 `worker/summary.go` 模式可参考（goroutine + channel + DB 表 + startup backfill）

## Goals / Non-Goals

**Goals:**
- 用户在 web UI 上 CRUD cron 任务
- 到点自动触发 AI 自主执行（无人工干预）
- 复用 100% 现有 AI 工具链（webfetch / websearch / create_page / update_page / 等等）
- 完整执行历史可查（status / output / error / conversation_id）
- scheduler 重启可恢复（DB 持久化 `next_run_at`）
- 失败不连坐 — 一次失败不影响下次调度
- 任务并发安全 — 任务在跑时不再触发下一次

**Non-Goals:**
- auto_approve=false 的 UI 流程（先把"全 auto"做对，需要时再扩展）
- 分布式锁 / 多实例协调（单进程）
- 时区配置（用 server 本地时区）
- 任务依赖 / 任务编排
- 通知推送（失败只在 cron_runs 表里记录 + UI 显示）
- 任务模板市场 / 导入导出
- 动态创建任务（任务只能从 UI 创建，AI 不能在执行过程中自创建 cron 任务）

## Decisions

### 1. ReAct 循环重构：抽 `ReActEventSink` 接口

**决策**：新增 `ReActEventSink` 接口，`AIHandler` 新增私有方法 `runReAct(ctx, req, sink, opts) -> (*ReActResult, error)`。`AIChat` 改为薄包装：装 SSE sink、调 `runReAct`、保存消息。cron runner 也调 `runReAct`，传 no-op sink。

**替代方案考虑**：
- ❌ 在 `AIChat` 里加 cron 模式分支 — 让一个 1265 行的函数更长更乱
- ❌ 复制 ReAct 循环给 cron — 漂移风险巨大
- ❌ 走 HTTP 自调用（cron 任务发 POST 到 /api/ai/chat）— 复杂、阻塞、需要 mock SSE

**接口设计**：
```go
type ReActEventSink interface {
    WriteContent(text string)
    WriteToolCallStart(id, name, input string)
    WriteToolResult(id, name, output, errStr string)
    WritePermissionRequired(PermissionRequest)  // no-op for cron
    WriteAskUserRequest(AskUserRequest)          // no-op for cron
    WriteDone()
    WriteError(msg string)
}

type ReActOptions struct {
    AutoApproveWrites bool         // cron only: skip channel, execute directly
    MaxSteps          int          // default 10 for cron, 20 for HTTP
    Timeout           time.Duration // 0 = no timeout (HTTP), 5min for cron
    Sink              ReActEventSink
    RunID             int64         // for logging/correlation
}

type ReActResult struct {
    FinalContent  string
    ToolCallCount int
    WriteCount    int  // number of write tools actually executed (cron, after auto-approve)
}
```

**关键约束**：现有 `sseWrite*` 系列函数保留 — SSE sink 内部直接调用它们，行为零变化。

### 2. cron 模式工具过滤

**决策**：在 `internal/ai/provider.go` 新增 `WikiToolsForCron() []Tool` — 复用 `WikiTools()` 但过滤掉 `ask_user`。`ask_user` 不在工具列表里，模型根本不会尝试调用它（最干净的禁用方式）。

`ask_user` 是当前唯一会无限阻塞的工具 — 它依赖 HTTP/SSE 推送 + 用户响应。permission 闸门在 `AutoApproveWrites=true` 时直接短路，不再 register channel。

### 3. 系统提示词 cron 前缀

**决策**：在 `buildWikiMaintainerPrompt` 顶部插入一个 cron 模式段，**不修改** 原有内容：

```go
func buildCronPrompt(basePrompt, taskPrompt, lastRunSummary string, now time.Time) string {
    prefix := fmt.Sprintf(`## 当前模式: 定时任务 (autonomous)
- 用户不在场,无法回答问题
- ask_user 工具不可用,禁止调用
- 写操作已开启 auto_approve,直接执行不需要确认
- 完成后用 1-2 句中文总结你做了什么（这是 cron_runs.output_summary 的来源）

## 当前时间
%s

## 本次任务
%s
`, now.Format("2006-01-02 15:04:05"), taskPrompt)

    if lastRunSummary != "" {
        prefix += fmt.Sprintf("\n## 上次运行摘要\n%s\n", lastRunSummary)
    }
    return prefix + "\n" + basePrompt
}
```

这样 cron 路径的 system prompt = prefix + 原有 wiki_maintainer prompt。结构规则、命名规范、工具说明全部保留，只在前面声明"这次是自主模式"。

### 4. 数据模型

**`cron_tasks`**：
```sql
CREATE TABLE cron_tasks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  cron_expr TEXT NOT NULL,              -- "0 9 * * *"
  prompt TEXT NOT NULL,                 -- the task description
  enabled INTEGER NOT NULL DEFAULT 1,
  auto_approve INTEGER NOT NULL DEFAULT 1,  -- always 1 for v1, reserved for future
  max_steps INTEGER NOT NULL DEFAULT 10,
  timeout_sec INTEGER NOT NULL DEFAULT 300,
  next_run_at DATETIME,                 -- computed from cron_expr
  last_run_at DATETIME,
  last_status TEXT,                     -- 'success' | 'failed' | 'running' | 'pending_review'
  last_error TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_cron_tasks_enabled_next_run ON cron_tasks(enabled, next_run_at);
```

**`cron_runs`**：
```sql
CREATE TABLE cron_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id INTEGER NOT NULL REFERENCES cron_tasks(id) ON DELETE CASCADE,
  status TEXT NOT NULL,                 -- 'running' | 'success' | 'failed' | 'timeout'
  started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  finished_at DATETIME,
  duration_ms INTEGER,
  output_summary TEXT,                  -- AI's final 1-2 sentence summary
  error TEXT,
  write_count INTEGER NOT NULL DEFAULT 0,  -- how many wiki pages were modified
  steps_used INTEGER NOT NULL DEFAULT 0,
  conversation_id INTEGER REFERENCES conversations(id) ON DELETE SET NULL
);
CREATE INDEX idx_cron_runs_task_id ON cron_runs(task_id, started_at DESC);
CREATE INDEX idx_cron_runs_status ON cron_runs(status);
```

每次 run 创建一个 `conversations` 行（`context_type='cron_task'`，`role='wiki_maintainer'`），完整对话持久化在 `messages` 表 — 用户点开 run 详情能看 AI 调了哪些 webfetch、读了哪些 wiki 页、提了哪些 create_page。

### 5. Scheduler 设计

**单 goroutine + 60s tick**，参考 `worker/summary.go` 的 `BackfillOnce` 模式：

```go
type Scheduler struct {
    db        *sql.DB
    queries   *model.Queries
    runner    *Runner
    cron      *cron.Parser  // robfig/cron/v3
    interval  time.Duration // default 60s
}

func (s *Scheduler) Run(ctx context.Context) {
    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:
            s.tick(ctx)
        }
    }
}

func (s *Scheduler) tick(ctx context.Context) {
    now := time.Now()
    // Single transaction: claim tasks atomically
    tx, _ := s.db.BeginTx(ctx, nil)
    rows, _ := tx.Query(`SELECT id, ... FROM cron_tasks
                         WHERE enabled=1 AND next_run_at <= ?
                         AND (last_status != 'running' OR last_run_at < ?)`,
                         now, now.Add(-2*time.Hour))  // stale 'running' = crashed, allow retry
    for each task:
        // Compute next_run_at, mark last_status='running', commit
    // After commit, dispatch to runner
    for each claimed task:
        go s.runner.Run(ctx, task)
}
```

**关键点**：
- **不堆积**：用 `last_status='running'` 守卫，正在跑的任务不重复触发
- **崩溃恢复**：超过 2 小时还 `running` 视为僵尸，下次 tick 可以重新触发
- **next_run_at 提前算好**：tick 时一次性 UPDATE，避免同 tick 多 worker 抢同一任务
- **dispatch 用 goroutine**：scheduler 主体不阻塞，runner 自带超时

### 6. Runner 生命周期

```go
func (r *Runner) Run(parentCtx context.Context, task Task) {
    // 1. Apply per-task timeout
    ctx, cancel := context.WithTimeout(parentCtx, time.Duration(task.TimeoutSec)*time.Second)
    defer cancel()

    // 2. Create conversation record
    conv, _ := r.queries.CreateConversation(ctx, model.CreateConversationParams{
        ContextType: "cron_task", Role: "wiki_maintainer",
        Title: fmt.Sprintf("[cron] %s @ %s", task.Name, time.Now().Format(...)),
    })

    // 3. Load last run summary
    lastSummary := r.loadLastRunSummary(ctx, task.ID)

    // 4. Build system prompt + user message
    sysPrompt := ai.BuildCronPrompt(ai.BuildSystemPrompt(...), task.Prompt, lastSummary, time.Now())
    userMsg := fmt.Sprintf("请执行本次定时任务。当前时间 %s。", time.Now().Format(...))

    // 5. Build ChatRequest
    chatReq := ai.ChatRequest{
        Messages:     []ai.Message{{Role: "user", Content: userMsg}},
        SystemPrompt: sysPrompt,
        Tools:        ai.WikiToolsForCron(),  // excludes ask_user
        MaxTokens:    8192,
    }

    // 6. Build sink
    sink := &CronSink{
        taskID: task.ID,
        runID:  runID,
        log:    r.log,
    }

    // 7. Save user message
    r.saveUserMessage(ctx, conv.ID, userMsg)

    // 8. Run
    result, err := r.aiHandler.RunReAct(ctx, chatReq, ReActOptions{
        AutoApproveWrites: true,
        MaxSteps:          task.MaxSteps,
        Timeout:           time.Duration(task.TimeoutSec) * time.Second,
        Sink:              sink,
        RunID:             runID,
    })

    // 9. Save assistant message + update run
    r.saveAssistantMessage(ctx, conv.ID, result.FinalContent, result.ToolCallCount)
    r.finalizeRun(ctx, runID, err, result)
}
```

### 7. 错误处理

| 情况 | 处理 |
|---|---|
| `StreamChat` 失败 | run 标 `failed`，error = 错误信息，next_run_at 正常计算 |
| 单个 tool 执行失败 | 现有逻辑：tool_result 注入错误，AI 继续推理，run 整体标 `success`（除非 AI 也认为失败） |
| `MaxSteps` 达到 | run 标 `failed` `error="max_steps_reached"`，next_run_at 正常 |
| `Timeout` 达到 | `ctx` cancel，AI 中断，run 标 `failed` `error="timeout"` |
| AI 提出写但 `executeWriteTool` 失败 | tool_result 注入错误给 AI，AI 继续，run 整体根据 AI 决定 |
| 任务崩溃（server 退出） | last_status='running' 留在 DB，下次启动 tick 检测到超时则可重试（v1 只警告不自动重启） |
| scheduler 重启 | startup backfill：把 `next_run_at <= now` 的 enabled 任务全部触发（与 worker/summary 模式一致） |

### 8. 前端

**页面结构**：
- `/cron` — 任务列表（卡片：name / cron 描述 / next_run / last_status / enabled toggle / 编辑按钮 / 立即运行 / 删除）
- `/cron/new` — 新建表单
- `/cron/{id}` — 编辑 + 运行历史（runs 列表 + 单个 run 详情对话框显示 output / error / conversation 链接）

**组件**：
- `CronTaskForm.tsx` — name、cron 表达式（带"每天 9 点 / 每周一 8 点 / 每小时"预设按钮）、prompt（多行 textarea）、max_steps、timeout
- `CronRunHistory.tsx` — 表格列出 runs，状态用颜色徽章
- `CronRunDetailDialog.tsx` — 显示 output / error / 链接到 conversation（如果存在）
- `useCronTasks` / `useCronRuns` SWR hooks

**cron 表达式 UX**：直接用 `<input>` 接受表达式，旁边显示人类可读描述（用 `cron.Parser` 反向解析）。预设按钮插入对应表达式到 input。

### 9. 依赖

新增：`github.com/robfig/cron/v3` — 标准选择，文档好，活跃维护。

## Risks / Trade-offs

- **[重构 ReAct 循环引入回归风险]** → 保留所有现有 `sseWrite*` 函数 + 现有 SSE 行为不变；`AIChat` 改完后跑现有 `ai_test.go` 套件（如果通过）；手动 SSE 端到端测试
- **[cron 模式 auto_approve 静默写 wiki]** → UI 创建表单必须显式提示"auto_approve=true 时 AI 写操作直接生效，无确认"；保留任务详情里的"上次写入了 N 个页面"统计
- **[AI 跑超长任务占资源]** → max_steps (默认 10) + timeout (默认 5min) 双重保险；超过的 run 标 failed
- **[cron 跑失败用户不知道]** → 列表页 last_status 标红 + failed 计数；详情页显示 error；v1 不做推送通知
- **[prompt 写得宽泛导致 AI 跑偏]** → prompt 是用户自己写的自负其责；UI 不做 prompt 模板校验；runs 历史可以审计
- **[cron 表达式错 → 永远不跑]** → 创建时用 `cron.Parser.Parse()` 校验，无效表达式 400 拒绝；UI 实时显示"下次运行"时间
- **[任务堆积（DB 锁 / 长任务重叠）]** → 任务级 running 守卫 + 单 runner goroutine（v1 不做并发执行，同一任务必须串行）
- **[TZ 问题]** → server local time，明确写在 UI 提示"使用服务器本地时区"；v1 不做用户 TZ 设置

## Migration Plan

- **新增 migration `013_add_cron_tasks.sql`**：建 `cron_tasks` + `cron_runs` 表 + 索引
- **零停机**：表是新增，server 启动时跑 migration（项目现有模式）
- **回滚**：drop 这两张表即可，应用代码 if not used 也不报错
- **部署顺序**：先合 backend 重构（不挂到路由），合后跑测试；再合 scheduler 启动；最后合前端

## Open Questions

- **prompt 长度上限**：要不要限 2000 字防爆？建议限 4000 字，超出 400
- **run 输出截断**：AI 总结的 1-2 句存 `output_summary`，完整 final_content 存 `messages.content` 表（已有）
- **同一任务可不可同时跑多次**（v1 策略：不行，running 守卫；多并发是未来特性）
- **删除任务时 `cron_runs` 怎么办**：当前设计 `ON DELETE CASCADE` 自动清理（保留近 30 天的策略不实现）
- **任务能复制吗**：v1 不做（用户手动再填）
