# Learn Helper 项目骨架设计

## Context

当前 Learn Helper 项目处于 PRD 和设计文档阶段，没有任何代码实现。需要搭建完整的 Monorepo 项目骨架作为后续功能开发的基础。

**当前状态：**
- 有 `docs/prd/2026-05-28-learn-helper-prd.md` — 完整的产品需求文档
- 有 `docs/superpowers/specs/2026-05-27-learn-helper-design.md` — 架构设计文档（MVP 范围已同步）
- 无代码实现

**技术栈约束：**
- 前端：React + Vite + Tailwind + React Router + SWR（proposal 中已明确）
- 后端：Go 1.25 + Chi router + sqlc + go-sqlite3
- AI：Claude Provider (MVP)，抽象层预留扩展
- 数据库：SQLite（单用户零运维）
- 通信：REST API + SSE

**MVP 范围约束（与 PRD 5.3 节对齐）：**
- 数据结构与算法知识体系
- 内置题库（算法题为主）
- AI 两角色（知识讲解、解题辅导），学习规划延后
- 基本学习进度追踪，无间隔重复提醒

## Goals / Non-Goals

**Goals:**
- 搭建可运行的项目骨架，前后端能同时启动并正常通信
- 生成 SQLite schema 并通过 sqlc 生成类型安全的 Go 数据访问层
- 实现所有核心 CRUD API（topics / exercises / learning_records / conversations / messages / ai_configs）
- 实现 AI Provider 抽象接口 + Claude Provider 流式实现
- 搭建四个前端页面骨架（知识图谱 / 练习 / 仪表盘 / 设置）+ AI 对话侧边面板
- 准备种子数据（知识点树 + 练习题），跑通一个知识点的完整学习闭环

**Non-Goals:**
- 真实 AI 对话功能（需要配置 API Key 后才能测试）
- shadcn/ui 组件库的具体组件（骨架先用 HTML/Tailwind，后续按需引入）
- 学习规划角色 AI
- 仪表盘薄弱点分析
- 间隔重复复习提醒

## Decisions

### D1：前端用 Vite 替代 Next.js（实际开发时）

**决策：** 前端项目使用 `create-vite` 初始化而非 `create-next-app`，保留 React Router + Vite 技术栈。

**理由：**
- Next.js App Router 的服务端能力（SSR/SSG）对于单用户本地应用不需要
- Vite 热更新更快，开发体验更好
- 与设计文档的"Next.js"名称有出入，但功能等价——React + TypeScript + Tailwind + 路由，实际用户体验一致

**替代方案考虑：**
- 用 `create-next-app` 初始化，保留 App Router → 保留了 SSR 能力但增加复杂度，MVP 不需要

### D2：Go 后端用原始 SQL + sqlc 生成类型安全代码

**决策：** 直接写 SQL 语句 + sqlc 生成类型安全的 repository 层，不使用 GORM 等 ORM 框架。

**理由：**
- sqlc 根据 SQL 自动生成 Go 代码，保证编译时类型安全
- SQL 语句直接可见，易于优化和调试
- 单用户场景没有复杂的关联查询，原始 SQL 完全够用
- 避免 ORM 的隐藏行为和学习成本

**sqlc 生成产物：**
执行 `sqlc generate` 后，会在 `internal/model/` 生成 SQL schema 对应的 Go 结构体（如 `Topic`, `Exercise`），在 `internal/repository/` 生成数据访问方法（如 `GetAllTopics`, `GetTopicBySlug`）。生成的代码受版本控制，不需每次重新生成。

**替代方案考虑：**
- 用 GORM → 增加依赖，学习成本，但操作简单
- 用 sqlx → 手动写 SQL，类型安全不如 sqlc

### D3：AI Provider 流式输出用 SSE

**决策：** AI 对话采用 Server-Sent Events (SSE) 实现服务端推送流式响应。

**理由：**
- 与设计文档 PRD 一致，PRD 10.2.2 节明确要求 SSE 流式输出
- Go 的 HTTP 响应支持 SSE 简单直接
- 前端 `fetch` API 原生支持 `ReadableStream`，实现简单

**替代方案考虑：**
- WebSocket → 能力更强但复杂度也更高，MVP 阶段 SSE 够用

### D4：前后端开发联调用 Vite Proxy

**决策：** 前端开发时通过 Vite 的 proxy 配置将 `/api` 请求转发到 Go 服务（localhost:8080）。

**理由：**
- 避免跨域问题，前端开发只需一个端口（3000）
- Go 服务和前端独立启动，调试方便
- 无需额外部署 API Gateway 或代理服务器

**替代方案考虑：**
- 前端也跑在 Go 服务里 → 增加 Go 复杂度，不必要
- CORS 配置 → 每个请求都要处理，不如 proxy 简单

### D5：Monorepo 使用 pnpm workspaces

**决策：** 根目录用 pnpm-workspace.yaml 管理 frontend 和 backend 两个包。

**理由：**
- pnpm workspaces 提供可靠的 Monorepo 支持，依赖共享和 hoist
- 根目录 `pnpm dev` 可以同时启动前后端
- 种子数据脚本可以作为 backend 的 script 方便执行

### D6：种子数据直接在 SQLite 执行 SQL 文件

**决策：** 种子数据通过 `sqlite3 learn-helper.db < seed.sql` 直接导入，不做 Go migration 工具。

**理由：**
- 单用户场景，不需要在线迁移
- 直接执行 SQL 简单直观，可手动验证
- Go 后端启动时不需要等待 migration 完成

## Risks / Trade-offs

| 风险 | 影响 |  Mitigation |
|------|------|-------------|
| 前端用 Vite 而非 Next.js，与设计文档描述有出入 | 文档和代码不完全对齐 | 在 CLAUDE.md 中明确注明实际用 Vite + React Router |
| seed 数据导入后无法追踪状态 | 不知道哪些数据是 seed 哪些是用户创建的 | 种子数据统一用固定 ID 插入（如 id=1,2,3...），用户数据从 id=1000 开始 |
| AI API Key 明文存储在 SQLite | 安全风险 | API Key 加密存储在 ai_configs 表，MVP 阶段可接受，后续可加加密层 |
| 前端 AI 面板 SSE 连接断开会丢失最后几字 | 用户体验小问题 | 前端重连时回显已有对话内容（从 API 获取历史） |

## Migration Plan

详见 `tasks.md`，以下为概要：

| 步骤 | 产出 | 关键验证 |
|------|------|---------|
| 1. Monorepo 配置 | package.json, pnpm-workspace.yaml, README.md, CLAUDE.md | `pnpm install` 无报错 |
| 2. Go 后端骨架 | schema.sql → sqlc generate → handler 代码 | `go build ./...` 成功 |
| 3. AI 模块 | provider.go, claude.go, ai.go handlers | `go build ./...` 成功 |
| 4. 前端骨架 | Vite + React 项目，4 个页面 + AI 面板 | `pnpm build` 成功 |
| 5. 种子数据 | seed.sql 导入 topics + exercises 表 | 查询 API 返回数据 |
| 6. 联调 | 前后端同时运行 | 浏览器访问 localhost:3000 正常 |

**回滚：** 删除 `backend/learn-helper.db`，重新执行 `schema.sql` + `seed.sql` 即可恢复干净状态。

## Open Questions

| 问题 | 状态 | 说明 |
|------|------|------|
| 是否需要 pre-commit hook？ | 延后 | MVP 阶段不需要，后续按需添加 |
| 前端是否需要 storybook？ | 否 | 单用户工具，不需要 |
| API 版本控制？ | 否 | MVP 单用户，单一版本足够 |