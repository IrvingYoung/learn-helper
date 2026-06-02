# LLM Wiki 单机部署方案

**日期**: 2026-06-02
**状态**: 设计中

## 背景

LLM Wiki 是一个 LLM 维护的个人知识库 web 应用。当前在本地 `pnpm dev` 跑前后端分离的开发模式，但没有生产部署流程。本次目标：把项目部署到一台云厂商 VPS，提供 HTTPS 公网访问，让用户和小圈子（3-10 人）可以跨设备使用。

## 目标

- 在 1 台 Linux VPS 上稳定运行 LLM Wiki（Go 后端 + React 前端）
- HTTPS 公网访问，反代自动申请 Let's Encrypt 证书
- Git push 到 main 自动构建并部署
- 本地数据快照，保留 7 天
- 进程崩溃自动恢复

## 非目标

- 多实例 / 水平扩展
- 异地多活 / 高可用
- 容器化（k8s / Compose）
- 外部监控告警系统
- 数据库迁移到 PostgreSQL / MySQL
- 用户体系 / 多租户

## 架构

### 拓扑

```
Internet (静态 IP)
  └─► Caddy (:80, :443)  [Let's Encrypt TLS]
        ├─► /api/*   ──► 127.0.0.1:8080 (Go 单进程)
        ├─► /share/* ──► 127.0.0.1:8080 (Go SSR + og: meta)
        └─► 其他      ──► 127.0.0.1:8080 (go:embed 静态 SPA)

Go 进程 (systemd 管理)
  ├─► SQLite (WAL 模式, /opt/learn-helper/learn-helper.db)
  ├─► robfig/cron (进程内调度, 读 cron_tasks 表)
  └─► LLM Provider (Anthropic / DeepSeek over HTTPS)
```

### 机器规格

- **2 vCPU / 4 GB RAM / 40 GB SSD**（国内云厂商约 ¥50-80/月）
- Ubuntu 24.04 LTS
- 公网静态 IPv4 一个

### 目录结构

```
/opt/learn-helper/
├── learn-helper            # Go 单二进制 (go:embed 自带 SPA)
├── learn-helper.db         # SQLite 主库
├── learn-helper.db-wal     # WAL 文件
├── learn-helper.db-shm     # shared memory
├── .env                    # 环境变量 (chmod 600, learnhelper:learnhelper)
├── backups/                # 本地快照, 7 天滚动
└── learn-helper.bak        # 上次成功运行的二进制 (回滚用)

/etc/systemd/system/learn-helper.service
/etc/caddy/Caddyfile
/etc/cron.daily/learn-helper-backup
/etc/ufw/                  # 防火墙规则

/home/learnhelper/.ssh/    # CI 部署用的公钥
```

## 组件

### 1. Go 后端 (改动)

**核心改动**：`go:embed` 把 `dist/` 装进二进制。

`backend/cmd/server/main.go`：
```go
import "embed"

//go:embed all:dist
var spaDistFS embed.FS
```

**前提**：`backend/cmd/server/dist/.gitkeep` 存在（占位文件），保证 embed 编译通过。CI 构建前 `pnpm --filter learn-helper-frontend build` 产出真实 dist 覆盖占位。

**`backend/internal/handler/share.go:275 getDistIndexHTML()` 改造**：
```go
func getDistIndexHTML() ([]byte, error) {
    // 1) 优先读 go:embed
    if b, err := spaDistFS.ReadFile("dist/index.html"); err == nil {
        return b, nil
    }
    // 2) 失败时回退到磁盘 (开发模式 + 旧 LH_SPA_DIST 行为)
    //    ... 保留现有 candidates 列表 ...
}
```

dev 模式无 dist 时，embed 仍能编（因为有 `.gitkeep`），运行时报 "no such file"，触发回退走磁盘候选路径。生产模式 embed 命中，不依赖任何运行时路径。

### 2. systemd 服务单元

`infra/systemd/learn-helper.service`：
```ini
[Unit]
Description=LLM Wiki (Go backend)
After=network.target

[Service]
Type=simple
User=learnhelper
Group=learnhelper
WorkingDirectory=/opt/learn-helper
EnvironmentFile=/opt/learn-helper/.env
ExecStart=/opt/learn-helper/learn-helper
Restart=always
RestartSec=3
# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/learn-helper
# 资源限制
LimitNOFILE=65536
# 日志走 journald
StandardOutput=journal
StandardError=journal
SyslogIdentifier=learn-helper

[Install]
WantedBy=multi-user.target
```

`.env` 内容（部署到 `/opt/learn-helper/.env`，权限 600）：
```
PORT=8080
DB_PATH=/opt/learn-helper/learn-helper.db
LH_SKILLS_DIR=/opt/learn-helper/skills
LH_SPA_DIST=
ANTHROPIC_API_KEY=sk-ant-xxx
DEEPSEEK_API_KEY=sk-xxx
```

`ExecStart` 不传 `:8080` 之外的监听地址——Go 代码里直接 `http.Server.Addr = "127.0.0.1:8080"` 绑定 loopback，公网无法直连。

### 3. Caddy 反代

`infra/caddy/Caddyfile`：
```caddyfile
{
    email admin@yourdomain.top
}

yourdomain.top, www.yourdomain.top {
    encode gzip zstd

    @api path /api/*
    reverse_proxy @api 127.0.0.1:8080

    @share path /share/*
    reverse_proxy @share 127.0.0.1:8080

    # 其余路径: 前端 SPA + go:embed 静态
    reverse_proxy 127.0.0.1:8080
}
```

- `email` 用于 Let's Encrypt 账户注册 / 续期通知
- 多个域名用逗号分隔，主域 + www 自动跳转主域
- Caddy 自动：HTTP→HTTPS、OCSP stapling、证书续期
- 防火墙：`ufw default deny; ufw allow 80/tcp; ufw allow 443/tcp; ufw allow ssh`

### 4. CI/CD

`.github/workflows/deploy.yml`：
```yaml
name: deploy
on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
        with: { version: 9 }
      - uses: actions/setup-go@v5
        with: { go-version: '1.25' }
      - run: pnpm install --frozen-lockfile
      - run: pnpm --filter learn-helper-frontend build
      - run: |
          mkdir -p backend/cmd/server/dist
          rsync -a --delete frontend/dist/ backend/cmd/server/dist/
      - run: |
          cd backend
          CGO_ENABLED=0 go build -ldflags="-s -w" -o learn-helper ./cmd/server
      - uses: actions/upload-artifact@v4
        with: { name: learn-helper, path: backend/learn-helper }

  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/download-artifact@v4
        with: { name: learn-helper }
      - uses: appleboy/scp-action@v0.1.7
        with:
          host: ${{ secrets.DEPLOY_HOST }}
          username: ${{ secrets.DEPLOY_USER }}
          key: ${{ secrets.SSH_PRIVATE_KEY }}
          source: "learn-helper"
          target: "/tmp/learn-helper.new"
      - name: Atomic swap + restart
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.DEPLOY_HOST }}
          username: ${{ secrets.DEPLOY_USER }}
          key: ${{ secrets.SSH_PRIVATE_KEY }}
          script: |
            set -euo pipefail
            # 备份当前二进制
            sudo cp /opt/learn-helper/learn-helper /opt/learn-helper/learn-helper.bak
            # 原子替换 (mv 是同分区 rename, 不会读到半截文件)
            sudo mv /tmp/learn-helper.new /opt/learn-helper/learn-helper
            sudo chown learnhelper:learnhelper /opt/learn-helper/learn-helper
            sudo chmod 755 /opt/learn-helper/learn-helper
            # 重启
            sudo systemctl restart learn-helper
            sleep 2
            # 健康检查
            curl -fsS http://127.0.0.1:8080/api/healthz || {
              echo "Health check failed, rolling back"
              sudo cp /opt/learn-helper/learn-helper.bak /opt/learn-helper/learn-helper
              sudo systemctl restart learn-helper
              exit 1
            }
            echo "Deploy OK"
```

需要的 GitHub Secrets：
- `DEPLOY_HOST` - VPS IP 或域名
- `DEPLOY_USER` - `learnhelper`
- `SSH_PRIVATE_KEY` - 部署专用 SSH 私钥（公钥加到 `learnhelper@host:.ssh/authorized_keys`）

### 5. 备份脚本

`/etc/cron.daily/learn-helper-backup`：
```bash
#!/bin/bash
set -euo pipefail

APP_DIR=/opt/learn-helper
BACKUP_DIR=$APP_DIR/backups
TS=$(date +%Y-%m-%d-%H%M)
RETENTION_DAYS=7

# 一致快照 (sqlite3 .backup 是 ACID-safe 的, 走 WAL checkpoint 后的状态)
sudo -u learnhelper sqlite3 "$APP_DIR/learn-helper.db" ".backup '$BACKUP_DIR/lh-$TS.db'"

# 清理 7 天前的快照
find "$BACKUP_DIR" -name 'lh-*.db' -mtime +$RETENTION_DAYS -delete

# 留个最近 1 个的软链方便回滚
ln -sfn "$BACKUP_DIR/lh-$TS.db" "$BACKUP_DIR/lh-latest.db"
```

权限：chmod 755，`/etc/cron.daily/` 由 cron.daily 自动每天凌晨执行。

### 6. 域名与 DNS（部署前一次性）

- 注册 .top 域名（Cloudflare Registrar / Porkbun / NameSilo，约 $1-2/年）
- 在域名注册商处改 NS 记录到 Cloudflare（推荐，便于管理）
- Cloudflare DNS 添加：`A yourdomain.top → VPS 静态 IP`
- Cloudflare SSL/TLS 设为 **Full (Strict)**
- 80/443 端口在云厂商安全组放开

## 数据流

### 普通请求

1. 浏览器 `GET /`
2. DNS 解析 → 静态 IP
3. Caddy 接收，证书匹配，走 reverse_proxy
4. Go handler 收到，`go:embed dist/index.html` 返回 SPA HTML
5. 浏览器加载 JS bundle，React Router 处理路由
6. SPA 调 `GET /api/wiki/tree` → Caddy → Go → SQLite

### Share 链接 + og: 注入

1. 外部 IM 用户 `GET /share/abc?t=token`
2. Caddy 命中 `@share` 规则 → Go SSR handler
3. SSR handler 读 SQLite 拿页面元数据
4. 注入 `<meta property="og:title" ...>` 等到 `dist/index.html`
5. 返回带 og: 的 HTML
6. IM 爬虫抓 meta → 渲染预览卡

### Cron 任务

1. Go 启动时 `robfig/cron` 启动调度器
2. 读 `cron_tasks` 表，每个 enabled 任务注册到 cron
3. 到点触发 → engine → 调 AI provider → 写回数据库 / 写 wiki

## 安全

- **后端不暴露公网**：Go `http.Server.Addr = "127.0.0.1:8080"`，Caddy 是唯一入口
- **Caddy 监听 80/443**：唯一对外端口（SSH 改高位端口）
- **API 鉴权**：v1 暂用"信任网络 + URL 难猜"（owner 域），如果开放公网再考虑加 token / session
- **.env 权限** 600，仅 learnhelper 用户可读
- **systemd hardening**：`NoNewPrivileges`、`ProtectSystem=strict`、`PrivateTmp`
- **SSH key-only**：禁用密码登录，`PasswordAuthentication no`
- **ufw 防火墙**：仅 80/443/SSH 端口
- **云厂商安全组**：只对 0.0.0.0/0 开放 80/443，SSH 仅对个人 IP

## 测试 & 验证

部署后必须通过的清单：

- [ ] `curl -I https://yourdomain.top/` → 200，HTML，Content-Type: text/html
- [ ] `curl https://yourdomain.top/api/healthz` → 200
- [ ] `curl -s 'https://yourdomain.top/share/<slug>?t=<token>' | grep og:title` → 看到 og:* meta
- [ ] 在 IM（微信/钉钉/Slack）粘贴 share URL → 显示预览卡
- [ ] 浏览器登录 → 写一次 wiki → 刷新看到内容
- [ ] `sudo systemctl restart learn-helper` → 1-2 秒恢复，浏览器无感
- [ ] `cp backups/lh-latest.db learn-helper.db && systemctl restart learn-helper` → 数据还原
- [ ] 模拟崩溃 `sudo kill -9 $(pgrep learn-helper)` → 3 秒内 systemd 自动拉起
- [ ] `nmap yourdomain.top` → 仅 22, 80, 443 开放
- [ ] `curl http://yourdomain.top:8080/api/healthz` → connection refused（确认 Go 不在公网监听）

## 风险 & 缓解

| 风险 | 缓解 |
|---|---|
| SQLite 单点写性能瓶颈 | 10 人 + 偶发 cron 调用完全够用，监控下做未来切 PG 的决策 |
| WAL 文件膨胀 | 已有 checkpoint 机制；备份脚本里加 `PRAGMA wal_checkpoint(TRUNCATE)` |
| VPS 整机故障 | 本地快照保留 7 天，可手动 rsync 到新机器恢复 |
| CI 私钥泄露 | GitHub Secrets + 部署专用 key，可单独 rotate；learnhelper 用户无 sudo 权 |
| Caddy 证书过期 | Let's Encrypt 90 天自动续期 + Caddy 内置；证书失败 Caddy 启动报错 journal 可看 |
| go:embed dist/ 不存在导致编译失败 | `dist/.gitkeep` 占位 + CI 强制先 build frontend；本地开发 embed miss 自动回退磁盘 |
| 部署时数据库 schema 迁移 | 现状 schema 内嵌 main.go 启动时 `CREATE TABLE IF NOT EXISTS`，无独立迁移。后续加 manual migration 时要 back-compat 策略 |

## 未来工作（不在本次范围）

- 监控告警（UptimeRobot / 健康探针外发）
- 数据库迁移到 PostgreSQL（多写并发需要时）
- 用户体系 + 鉴权（owner / viewer 角色）
- 异地备份推送 (OSS / S3)
- 容器化（如果多环境需要）

## 文件清单（plan 阶段要落地的）

1. **新增**：`backend/cmd/server/dist/.gitkeep`
2. **修改**：`backend/cmd/server/main.go`（加 embed.FS）
3. **修改**：`backend/internal/handler/share.go:getDistIndexHTML()`（embed 优先，磁盘回退）
4. **新增**：`infra/systemd/learn-helper.service`
5. **新增**：`infra/caddy/Caddyfile`
6. **新增**：`infra/backup/learn-helper-backup.sh`
7. **新增**：`.github/workflows/deploy.yml`
8. **新增**：`docs/runbook.md`（部署 / 回滚 / 故障排查）
9. **新增**：`docs/deploy.md`（一次性配置：域名、SSH key、安全组）

## 决策记录

- **2026-06-02**：选 systemd 而非 supervisor（系统原生，零依赖）
- **2026-06-02**：选 Caddy 而非 Nginx（自动 HTTPS、配置少 50%）
- **2026-06-02**：go:embed 改造而非外部 dist/ 目录（操作体验：部署 = 拷 1 个文件）
- **2026-06-02**：买 .top 域名而非裸 IP（$1-2/年，IM 预览卡 + Let's Encrypt 必要）
- **2026-06-02**：GitHub Actions 部署（项目已在 GitHub，最少配置）
- **2026-06-02**：本地 7 天快照，不推异地（用户接受此风险）
