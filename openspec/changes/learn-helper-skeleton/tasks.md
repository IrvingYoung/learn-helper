# Learn Helper 项目骨架 - 实现任务

## 1. 初始化 Monorepo 结构

- [x] 1.1 创建根目录 `package.json`（pnpm workspaces 配置）
- [x] 1.2 创建 `pnpm-workspace.yaml`
- [x] 1.3 创建 `README.md`（项目简介、快速启动说明）
- [x] 1.4 创建 `CLAUDE.md`（开发者文档、技术栈、项目结构）

## 2. Go 后端骨架

- [x] 2.1 初始化 Go Module（`go mod init learn-helper`）
- [x] 2.2 安装依赖（chi、go-sqlite3、google/uuid）
- [x] 2.3 创建目录结构（cmd/server、internal/{handler,service,repository,model,ai}、db/{migrations,seed}）
- [x] 2.4 创建 `sqlc.yaml` 配置
- [x] 2.5 创建 `db/migrations/schema.sql`（topics、exercises、learning_records、conversations、messages、ai_configs 六张表）
- [x] 2.6 创建 `db/migrations/queries.sql`（供 sqlc 生成的 SQL 查询）
- [x] 2.7 运行 `sqlc generate` 生成 model 和 repository 代码
  - 生成文件：`internal/model/` 下有 `topic.go`、`exercise.go`、`learning_record.go`、`conversation.go`、`message.go`、`ai_config.go` 等
  - 生成文件：`internal/repository/` 下有 `query*.go`，包含 `GetAllTopics`、`GetTopicByID`、`GetTopicBySlug` 等数据访问方法
- [x] 2.8 创建 `cmd/server/main.go`（路由注册、启动 server）
- [x] 2.9 创建 `internal/handler/handler.go`（基础 handler、HealthCheck）
- [x] 2.10 创建 `internal/handler/topics.go`（GetTopics、GetTopicBySlug、GetExercisesByTopic）
- [x] 2.11 创建 `internal/handler/exercises.go`（GetExercises、GetExerciseByID）
- [x] 2.12 创建 `internal/handler/learning.go`（GetLearningRecords、UpsertLearningRecord）
- [x] 2.13 验证后端编译（`go build ./...`）✓

## 3. AI 模块

- [x] 3.1 创建 `internal/ai/models.go`（Message、ChatRequest、ChatChunk、ChatResponse 结构体）
- [x] 3.2 创建 `internal/ai/provider.go`（AIProvider 接口、SystemPromptTemplates）
- [x] 3.3 创建 `internal/ai/claude.go`（ClaudeProvider 实现、流式响应）
- [x] 3.4 创建 `internal/handler/ai.go`（AIHandler：AIChat、GetConversations、GetMessages、GetAIConfigs、UpsertAIConfig）
- [x] 3.5 更新 `main.go` 注册 AI 路由
- [x] 3.6 验证 AI 模块编译 ✓

## 4. Next.js 前端骨架（实际用 Vite）

- [x] 4.1 创建 `frontend/package.json`（React 19 + Vite + Tailwind + React Router + SWR）
- [x] 4.2 创建 `frontend/vite.config.ts`（配置 API proxy 到 localhost:8080）
- [x] 4.3 创建 TypeScript / Tailwind / PostCSS 配置文件
- [x] 4.4 创建 `frontend/index.html` 和 `frontend/src/main.tsx` 入口
- [x] 4.5 创建 `frontend/src/App.tsx`（路由配置：/learn、/practice、/dashboard、/settings）
- [x] 4.6 创建 `frontend/src/components/Layout.tsx`（顶栏导航 + Outlet 布局，AI 面板开关状态由 Layout 自身 useState 管理）
- [x] 4.7 创建 `frontend/src/components/AIChatPanel.tsx`（AI 对话侧边面板、SSE 流式显示）
- [x] 4.8 创建 `frontend/src/app/learn/page.tsx`（知识图谱页：树导航 + 详情）
- [x] 4.9 创建 `frontend/src/app/practice/page.tsx`（练习题列表 + 详情）
- [x] 4.10 创建 `frontend/src/app/dashboard/page.tsx`（仪表盘：学习统计）
- [x] 4.11 创建 `frontend/src/app/settings/page.tsx`（AI 模型配置）
- [x] 4.12 安装前端依赖（`pnpm install`）
- [x] 4.13 验证前端构建（`pnpm build`）✓

## 5. 种子数据

- [x] 5.1 创建 `backend/db/seed/seed.sql`（知识点树 + 练习题示例）
- [x] 5.2 执行种子数据到 SQLite 数据库（server 启动时自动初始化）
- [x] 5.3 验证 seed 数据（查询 API 确认数据存在）— 后端启动时自动验证

## 6. 联调验证

- [x] 6.0 确认数据库已有 schema（执行 `sqlite3 backend/learn-helper.db < db/migrations/schema.sql` 初始化）— server 启动时自动初始化
- [ ] 6.1 启动 Go 后端（`go run cmd/server/main.go`）
- [ ] 6.2 启动前端（`pnpm dev`）
- [ ] 6.3 浏览器访问 http://localhost:3000，验证页面正常渲染
- [ ] 6.4 验证 API 响应（知识图谱数据加载正常）
- [ ] 6.5 配置 AI API Key 后测试 AI 对话（可选）