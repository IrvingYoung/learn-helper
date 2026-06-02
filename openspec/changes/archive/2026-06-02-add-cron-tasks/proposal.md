## Why

AI 驱动的 wiki 现在完全靠用户在聊天里触发 — 没有"让 AI 主动定期干活"的机制。用户希望能把重复性的工作自动化（第一个用例：每天抓 GitHub trending 写中文摘要页面），复用现有的 AI + 工具链（webfetch / websearch / create_page / update_page）就能覆盖大部分用例。

现在的障碍：
1. 后端没有任何调度基础设施（无 cron 库、无 ticker）
2. AIHandler 的 ReAct 循环和 SSE/HTTP 紧耦合，无法从 goroutine 调用
3. AI 工具集中 `ask_user` / permission 通道在无人值守下会卡死
4. 没有 UI 让用户配置任务、查看历史

## What Changes

- **新增 `cron_tasks` 和 `cron_runs` 表**，记录用户配置的任务和每次执行的元数据
- **新增 scheduler goroutine**：每 60s 扫表，对到点的任务启动执行
- **重构 `AIHandler.AIChat`**：抽出 `ReActEventSink` 接口和 `runReAct` 内部方法，HTTP 路径和 cron 路径共享同一套 ReAct 循环
- **新增 cron 模式系统提示词前缀**：在现有 wiki_maintainer prompt 顶部插入"autonomous 模式"声明，覆盖"先问再写"等冲突条款
- **新增 cron 模式工具过滤**：从 `chatReq.Tools` 中移除 `ask_user`，并短路 permission 闸门（auto_approve=true 时直接 `executeWriteTool`）
- **新增前端管理页** `/cron`：任务列表 / 新建编辑表单 / 运行历史
- **新增后端 HTTP 端点**：`/api/cron/tasks` (CRUD) + `/api/cron/tasks/{id}/runs` (list) + `/api/cron/tasks/{id}/run-now` (手动触发)
- **新增 Go 依赖** `github.com/robfig/cron/v3`（cron 表达式解析与下次执行时间计算）

## Capabilities

### New Capabilities
- `cron-tasks`: 用户可配置的 cron 定时任务系统。到点触发 AI 自主执行用户描述的任务，复用现有 AI 工具链写入 wiki。

### Modified Capabilities
无（不修改现有 spec 级别的需求行为；`AIHandler.AIChat` 的 HTTP 行为在重构后保持不变）

## Impact

- **后端代码**：
  - 新增 `internal/cron/` 包（scheduler、runner、models、http handler）
  - 重构 `internal/handler/ai.go`（抽出 `runReAct` + `ReActEventSink`，`AIChat` 改为薄包装）
  - 修改 `internal/ai/provider.go`（新增 `WikiToolsForCron()` 过滤 `ask_user`）
  - 新增 `internal/ai/cron_prompt.go`（cron 模式系统提示词前缀）
  - 修改 `cmd/server/main.go`（启动 scheduler goroutine，注入 AIHandler 引用）
- **数据库**：新增 `cron_tasks` 和 `cron_runs` 表（migration 013）
- **前端**：
  - 新增 `src/app/cron/page.tsx`（任务列表）
  - 新增 `src/app/cron/new/page.tsx` 和 `src/app/cron/[id]/page.tsx`（编辑/详情）
  - 新增 `src/components/CronTaskForm.tsx`、`CronRunHistory.tsx`
  - 新增 `src/lib/api/cron.ts`
- **依赖**：go.mod 新增 `github.com/robfig/cron/v3`
- **API**：8 个新 HTTP 端点（见 tasks.md）
- **文档**：ARCHITECTURE.md 新增 "Cron Tasks" 章节（实现时）
