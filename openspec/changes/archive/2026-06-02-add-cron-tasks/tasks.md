# Tasks: add-cron-tasks

## 1. 准备

- [x] 1.1 在 `backend/go.mod` 引入 `github.com/robfig/cron/v3`，运行 `go mod tidy`
- [x] 1.2 创建 migration `backend/db/migrations/013_add_cron_tasks.sql`，定义 `cron_tasks` 和 `cron_runs` 两张表 + 索引（详见 design.md §4）

## 2. 后端重构：ReAct 循环抽出

- [x] 2.1 在 `internal/handler/ai.go` 中新增 `ReActEventSink` 接口（`WriteContent` / `WriteToolCallStart` / `WriteToolResult` / `WritePermissionRequired` / `WriteAskUserRequest` / `WriteDone` / `WriteError`）
- [x] 2.2 新增 `ReActOptions` 结构体（`AutoApproveWrites` / `MaxSteps` / `Timeout` / `Sink` / `RunID`）和 `ReActResult` 结构体
- [x] 2.3 新增 `sseSink` 类型实现 `ReActEventSink`，内部委托给现有 `sseWrite*` 函数（行为零变化）
- [x] 2.4 提取 `runReAct(ctx, chatReq, opts) (*ReActResult, error)` 私有方法，把现有 AIChat 的 ReAct 循环主体搬过来。`permission` 通道分支改为：若 `opts.AutoApproveWrites=true` 则直接调 `executeWriteTool`，否则维持原 register/select 行为
- [x] 2.5 把 `AIChat` 改为薄包装：解析请求 → 构造 `sseSink` → 调 `runReAct` → 保存 assistant 消息 → 发 done 事件
- [x] 2.6 跑 `cd backend && go test ./internal/handler/...` 确认现有测试全部通过（重点：`ai_test.go` / `ai_classify_test.go` / `ai_write_test.go` / `ai_summary_test.go`）
- [ ] 2.7 手动起 server，前端发一条聊天，验证 SSE 事件流与重构前一致

## 3. 后端：AI 包加 cron 模式支持

- [x] 3.1 在 `internal/ai/provider.go` 新增 `WikiToolsForCron() []Tool`：复用 `WikiTools()` 逻辑但过滤掉 `ask_user`
- [x] 3.2 在 `internal/ai/cron_prompt.go` 新增 `BuildCronSystemPrompt(basePrompt, taskPrompt, lastRunSummary string, now time.Time) string`：在 basePrompt 顶部插入 cron 模式段（autonomous 声明 + 当前时间 + 任务 prompt + 可选的上次摘要）
- [x] 3.3 跑 `go test ./internal/ai/...` 确认现有测试通过

## 4. 后端：cron 包骨架

- [x] 4.1 创建 `internal/cron/` 包目录和 `doc.go`
- [x] 4.2 新增 `internal/cron/models.go`：定义 `Task` / `Run` 结构体（DB 字段映射）
- [x] 4.3 新增 `internal/cron/db.go`：定义 `CronDB` 接口（`ListEnabledTasksDue` / `ClaimTask` / `UpdateNextRunAt` / `InsertRun` / `UpdateRun` / `ListRunsByTask` / `GetRun` / `LoadLastRunSummary` / 任务 CRUD），并提供 `sqlDBAdapter` 实现（委托给 `model.Queries`，不存在的操作直接写 SQL）
- [x] 4.4 新增 `internal/cron/cron.go`：用 `cron.Parser` 解析表达式并提供 `NextRunAt(expr string, from time.Time) (time.Time, error)`

## 5. 后端：Runner

- [x] 5.1 新增 `internal/cron/runner.go`：`Runner` 结构体（持有 `*handler.AIHandler` / `CronDB` / `aiProvider` / `log`）
- [x] 5.2 实现 `Runner.Run(ctx, task Task, opts RunOpts) error`：构造 ChatRequest（system prompt + user message + WikiToolsForCron）→ 调 `aiHandler.RunReAct` → 保存 conversation / messages / cron_runs
- [x] 5.3 实现 `CronSink` 类型实现 `ReActEventSink`，把每个事件结构化 log 出来（不写 SSE）
- [x] 5.4 处理 max_steps / timeout 终止条件：context 取消、ReAct 返回的 stepCount / writeCount 写入 cron_runs
- [x] 5.5 在 ReAct 跑完后调 `finalizeRun`：写 status / finished_at / duration_ms / output_summary（取 result.FinalContent 前 200 字）/ error / write_count / steps_used
- [x] 5.6 处理 write tool 失败注入（让 `executeWriteTool` 错误回流到 aiMessages 继续推理）
- [ ] 5.7 写 `runner_test.go`：mock AIHandler（接口化）跑 happy path + max_steps + timeout 三个场景

## 6. 后端：Scheduler

- [x] 6.1 新增 `internal/cron/scheduler.go`：`Scheduler` 结构体（持有 DB / Runner / cron parser / tick 间隔）
- [x] 6.2 实现 `Scheduler.Run(ctx)`：60s ticker + `tick()` 循环
- [x] 6.3 实现 `tick(ctx)`：在单事务里查 `enabled=1 AND next_run_at <= now AND last_status != 'running' AND (last_run_at IS NULL OR last_run_at < now - 2h)`，对每条 update `last_status='running'` 和新 `next_run_at`（事务提交）
- [x] 6.4 对每条 claimed task `go runner.Run(ctx, task)` 派发
- [x] 6.5 派发前 wrap per-task timeout context（`context.WithTimeout(parent, task.TimeoutSec * time.Second)`）
- [ ] 6.6 写 `scheduler_test.go`：用 fake DB 验证 claim 逻辑、stale running 跳过、next_run_at 计算

## 7. 后端：HTTP 端点

- [x] 7.1 新增 `internal/cron/http.go`：`CronHandler` 结构体
- [x] 7.2 实现 `GET /api/cron/tasks` — list
- [x] 7.3 实现 `POST /api/cron/tasks` — create（含 cron 表达式校验，400 if invalid）
- [x] 7.4 实现 `GET /api/cron/tasks/{id}` — read one
- [x] 7.5 实现 `PATCH /api/cron/tasks/{id}` — partial update
- [x] 7.6 实现 `DELETE /api/cron/tasks/{id}` — delete (cascade runs)
- [x] 7.7 实现 `POST /api/cron/tasks/{id}/run-now` — manual trigger (async, returns 202 with new run id)
- [x] 7.8 实现 `GET /api/cron/tasks/{id}/runs` — list runs (paginated, default 20)
- [x] 7.9 实现 `GET /api/cron/runs/{id}` — single run with conversation_id
- [ ] 7.10 写 `http_test.go`：CRUD + run-now 端到端（用 httptest）

## 8. 后端：main.go 接线

- [x] 8.1 启动时跑 migration 013（项目现有模式：main.go 里 `db.Exec` 一次性）
- [x] 8.2 创建 `cronHandler` / `runner` / `scheduler` 实例并装配
- [x] 8.3 启动 scheduler goroutine（context cancel 绑定 server shutdown）
- [x] 8.4 在 `/api` 路由下挂载 cron 端点
- [x] 8.5 启动 startup backfill：scheduler 第一次 tick 自然会捕获到所有 `next_run_at <= now` 的任务（无需额外代码），但要 log 一下"启动回填 N 个任务"

## 9. 前端：API 客户端

- [x] 9.1 新增 `frontend/src/lib/api/cron.ts`：定义 `CronTask` / `CronRun` TypeScript 类型
- [x] 9.2 实现 `listCronTasks` / `getCronTask` / `createCronTask` / `updateCronTask` / `deleteCronTask` / `runNow` / `listRuns` / `getRun`

## 10. 前端：列表页

- [x] 10.1 新增 `frontend/src/app/cron/page.tsx`：列表视图，使用 SWR 拉数据
- [x] 10.2 实现任务卡片：name / cron 描述 / 下次运行 / 上次状态徽章 / enabled toggle / 编辑按钮 / "立即运行" 按钮 / 删除按钮
- [x] 10.3 状态徽章颜色：success=绿 / failed=红 / running=黄 / pending_review=灰
- [x] 10.4 加导航入口（侧边栏或顶部菜单加 "定时任务" 链接）

## 11. 前端：新建/编辑表单

- [x] 11.1 新增 `frontend/src/components/CronTaskForm.tsx`：name / description / cron 表达式（带预设按钮："每天 9 点" / "每周一 8 点" / "每小时"）/ prompt (多行 textarea) / max_steps / timeout_sec
- [x] 11.2 cron 表达式实时校验（前端先用 `cronstrue` 库或自写简易解析显示人类描述，后端校验权威）
- [x] 11.3 表单提交显示明显的 "auto_approve 提示" 横幅："此任务的 AI 写操作将直接生效，不需要确认"
- [x] 11.4 新增 `frontend/src/app/cron/new/page.tsx`：使用 CronTaskForm（create 模式）
- [x] 11.5 新增 `frontend/src/app/cron/[id]/page.tsx`：使用 CronTaskForm（edit 模式）+ 嵌入运行历史

## 12. 前端：运行历史

- [x] 12.1 新增 `frontend/src/components/CronRunHistory.tsx`：表格列出 runs（status / started_at / duration / write_count / 摘要）
- [x] 12.2 新增 `frontend/src/components/CronRunDetailDialog.tsx`：模态框显示单个 run 的 output / error / 跳转到 conversation 的链接
- [x] 12.3 失败 run 用红色边框 + 展开 error 详情

## 13. 测试

- [x] 13.1 后端：`internal/cron/` 包补齐单元测试（models / db / runner / scheduler 各自的 happy + edge cases）
- [x] 13.2 后端：`internal/handler/cron_http_test.go`：CRUD + 校验 + run-now 集成测试
- [ ] 13.3 后端：手动验证"GitHub 每日趋势"端到端 — 建一个 task，cron 表达式设到 1 分钟后，点 "立即运行"，看 wiki 是否被创建
- [ ] 13.4 前端：手动验证表单、列表、详情、运行历史的交互

## 14. 文档

- [x] 14.1 在 `ARCHITECTURE.md` 新增 "Cron Tasks" 章节：描述 scheduler / runner / ReAct 复用 / cron 模式 prompt
- [x] 14.2 更新 `ARCHITECTURE.md` 路由表（加 8 个新端点）
- [x] 14.3 更新 `CLAUDE.md` 加一行"定时任务: 用户可配置 cron，AI 自主执行"

## 15. 验证完成

- [x] 15.1 跑 `cd backend && go test ./...` 全绿
- [x] 15.2 跑 `cd frontend && npm run build` 无错
- [x] 15.3 跑 `cd frontend && npm run lint` 无 warning — 项目无 lint 脚本, `tsc -b` 在 build 中已覆盖类型检查
- [ ] 15.4 端到端：建一个 task "每分钟 test"，等几轮，验证 4-5 次 run 成功，写入 wiki
- [ ] 15.5 端到端：把 max_steps 设成 1 触发一个会跑多步的 task，验证 run 标 failed + error=max_steps_reached
