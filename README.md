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

## Deployment

For full deployment instructions (VPS provisioning, domain, CI/CD, systemd, Caddy, backups), see:

- [`docs/deploy.md`](docs/deploy.md) — first-time setup
- [`docs/runbook.md`](docs/runbook.md) — day-2 ops (restart, rollback, restore, troubleshooting)

如果要把 wiki 页面通过公网 URL 分享出去(在 IM 里粘贴链接时有 og: 预览卡),
需要走一个反代,把特定路径转发给 Go,其他路径给前端。

### 反代规则表(nginx / caddy / frp / ngrok 都适用)

| 路径前缀 | 转发目标 | 用途 |
|---|---|---|
| `/api/*` | `http://localhost:8080` | 主人的读写 API |
| `/share/*` | `http://localhost:8080` | 公网 permalink SSR + 公开内容 API |
| 其他 (`/`, `/wiki/*`, `/assets/*` 等) | 前端静态(dist/) | SPA |

### Vite dev proxy

`frontend/vite.config.ts` 只 proxy `/api/*` 给 Go(走 owner API)。  
**`/share/*` 在 dev 模式下不走 Go** —— Vite 直接交给 React Router,渲染
公开版 UI(没有 tree/chat、没有 og: meta 注入)。

原因:dev 用 Vite 的 HMR bundle,Go 拿 production 的 `dist/index.html`
会引用不存在的 `/assets/index-{hash}.js`,浏览器 404。

### 验证 og: meta(必须用 production build)

`pnpm dev` 跑不出来 og: 预览卡。要验 IM 链接预览,得:

```bash
# 1. 构建前端
cd frontend && pnpm build

# 2. 让 Go 能找到 dist/(自动找 ./frontend/dist 或设 LH_SPA_DIST)
cd ../backend && go run ./cmd/server

# 3. 启动反代,把 /api/* 和 /share/* 转给 :8080,其他给 dist/ 静态
# (caddy / nginx 配置略)
```

跑起来后:
- `curl -s 'http://localhost:8080/share/{slug}?t={token}' | head -30` 应该看到 og:* meta
- IM 里粘 `https://your-public-host/share/{slug}?t={token}` 应该显示预览卡

dev 模式只看 `pnpm dev` 主页 + 用分享按钮复制链接是否拿到完整 URL,够了。

### ⚠️ 安全

**后端 :8080 绝对不能直接公网暴露**(只挂反代,反代只暴露 80/443)。  
直接暴露 :8080 等于把 `/api/wiki/{id}` 这种写接口也开放给公网,任何人
拿到 URL 就能增删改你的 wiki 内容。go:embed 的 SPA 路径同理。

### og:image 替换

把 logo/品牌图(`1200x630` PNG)放到 `frontend/public/og-default.png`,
IM 抓链接时就会用这张图作预览卡缩略图。v1 不带这个文件也能跑,只是
预览没有图。