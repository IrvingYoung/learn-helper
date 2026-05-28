# CLAUDE.md

## 项目概述

Learn Helper 是一个面向软件工程师的面试学习助手，核心功能：知识图谱学习 + AI 辅导 + 练习题库 + 学习进度追踪。

MVP 聚焦：数据结构与算法，AI 仅支持知识讲解和解题辅导两角色。

## 技术栈

- Frontend: React 19 + Vite + Tailwind CSS + React Router + SWR + TypeScript
- Backend: Go 1.25 + Chi router + sqlc + go-sqlite3
- AI: Claude Provider (MVP)，抽象层预留扩展
- DB: SQLite（单用户零运维，文件位于 `backend/learn-helper.db`）
- 通信: REST API + SSE

## 项目结构

```
learn-helper/
├── frontend/           # React + Vite 应用
│   ├── src/
│   │   ├── app/       # 页面组件 (learn/practice/dashboard/settings)
│   │   ├── components/ # 布局和 AI 面板
│   │   ├── lib/       # API 客户端
│   │   └── types/     # TypeScript 类型
│   ├── package.json
│   └── vite.config.ts
├── backend/           # Go 服务
│   ├── cmd/server/    # 入口 main.go
│   ├── internal/
│   │   ├── handler/  # HTTP handler（路由注册、请求处理）
│   │   ├── service/   # 业务逻辑
│   │   ├── repository/ # sqlc 生成的数据访问层
│   │   ├── model/     # sqlc 生成的模型（Topic/Exercise 等）
│   │   └── ai/       # AI Provider 抽象 + Claude 实现
│   ├── db/
│   │   ├── migrations/ # schema.sql + queries.sql
│   │   └── seed/      # 种子数据
│   ├── sqlc.yaml
│   └── go.mod
```

## 开发命令

```bash
pnpm install          # 安装所有依赖
pnpm db:generate      # 运行 sqlc generate
pnpm db:seed          # 导入种子数据
pnpm dev              # 前后端同时启动（前端 3000，后端 8080）
```

## 关键约定

- **API Proxy**: 前端 Vite 配置 proxy 将 `/api` 转发到 `http://localhost:8080`
- **AI Provider 抽象**: `backend/internal/ai/provider.go` 定义 `AIProvider` interface，Claude 实现在 `claude.go`
- **sqlc**: 配置在 `backend/sqlc.yaml`，生成的代码在 `backend/internal/model/` 和 `backend/internal/repository/`
- **数据库**: `backend/learn-helper.db`，初始由 `db/migrations/schema.sql` 创建
- **种子数据**: 知识点 ID 1-13，练习题 ID 1-6，用户数据 ID 从 1000 开始
- **AI System Prompt**: 定义在 `backend/internal/ai/provider.go` 的 `SystemPromptTemplates`

## MVP 边界

**包含:**
- 知识图谱（树状导航 + 详情）
- AI 两角色（知识讲解、解题辅导）
- 练习题库（按知识点/难度筛选）
- 基本学习记录追踪

**不包含（第二期）:**
- AI 学习规划角色
- 仪表盘薄弱点分析
- 间隔重复复习提醒
- 系统设计/八股文深度题库