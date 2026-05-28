# Learn Helper

面向软件工程师的面试学习助手。知识体系 + AI 辅导 + 练习巩固。

## Quick Start

```bash
pnpm install
pnpm db:generate   # 生成 Go 数据访问层代码
pnpm db:seed       # 导入种子数据
pnpm dev           # 启动前端 (3000) + 后端 (8080)
```

## 技术栈

- Frontend: React + Vite + Tailwind CSS + React Router + SWR
- Backend: Go 1.25 + Chi + sqlc + SQLite
- AI: Claude Provider (MVP)

## 项目结构

```
learn-helper/
├── frontend/           # React + Vite 前端
├── backend/           # Go 服务
│   ├── cmd/server/    # 入口
│   ├── internal/
│   │   ├── handler/  # HTTP handlers
│   │   ├── service/   # 业务逻辑
│   │   ├── repository/ # sqlc 生成的数据访问层
│   │   ├── model/     # sqlc 生成的模型
│   │   └── ai/       # AI Provider
│   └── db/
│       ├── migrations/ # SQLite schema
│       └── seed/      # 种子数据
└── CLAUDE.md          # 开发者文档
```

## 开发

```bash
# 初始化数据库
cd backend && sqlite3 learn-helper.db < db/migrations/schema.sql && sqlite3 learn-helper.db < db/seed/seed.sql

# 前后端同时启动
pnpm dev
```

## 环境要求

- Node.js >= 20.0.0
- pnpm >= 9.0.0
- Go >= 1.25
- sqlc (go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest)
- sqlite3 (可选，用于手动操作数据库)