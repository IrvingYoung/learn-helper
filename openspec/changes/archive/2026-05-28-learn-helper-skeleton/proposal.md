# Learn Helper 项目骨架搭建

## Why

当前项目只有 PRD 和设计文档，没有任何代码。需要搭建完整的 Monorepo 项目骨架——Go 后端（Chi + sqlc + SQLite）+ Next.js 15 前端（Vite + Tailwind + shadcn/ui），为后续功能开发提供可运行的基础架构。MVP 阶段 AI 只实现 Claude Provider。

## What Changes

- 初始化 Monorepo 项目结构（pnpm workspaces）
- 搭建 Go 后端骨架：SQLite schema、CRUD API handlers、AI Provider 抽象 + Claude 实现
- 搭建 Next.js 前端骨架：路由、布局、四个页面（知识图谱/练习/仪表盘/设置）、AI 对话侧边面板
- 准备种子数据（知识点树 + 练习题示例）
- 配置 API proxy 实现前后端联调

## Capabilities

### New Capabilities

- **project-skeleton**: 项目整体结构、目录组织、技术栈选型
- **backend-api**: Go 后端 REST API handlers、数据模型、SQLite schema
- **ai-provider**: AI Provider 抽象接口、Claude 实现、SSE 流式输出
- **frontend-pages**: React + Vite + Tailwind + React Router 前端页面骨架，包含四个页面（知识图谱/练习/仪表盘/设置）和 AI 对话侧边面板
- **seed-data**: 知识点树和练习题的种子数据

### Modified Capabilities

（无，PRD 和设计文档阶段，暂无已有 capabilities）

## Impact

- 新增 `frontend/` 和 `backend/` 目录
- 后端依赖：Go 1.25 + Chi + sqlc + modernc.org/sqlite
- 前端依赖：Next.js 15 + Vite + Tailwind + shadcn/ui
- 前后端通过 `/api` 端点通信，后端监听 8080，前端开发时 proxy 转发