# Deploy LLM Wiki to a VPS

Single-binary deploy. No Docker, no reverse proxy, no domain required.

## Architecture

```
Mac (build)                    VPS (run)
─────────────                  ─────────────
pnpm build   → embed ─┐
                     ├─→ scp ─→ /opt/learn-helper/learn-helper
go build  ────────────┘                              │
                                                      ▼
                                              Go binary :8080
                                                      │
                                              SQLite (./learn-helper.db)
```

The Go binary serves the SPA (embedded) and API on `:8080`. API keys are configured in the web UI, not in env vars.

## 0. One-time server setup (on VPS)

Ubuntu/Debian. Run once per fresh VM:

```bash
# Base tools
sudo apt update && sudo apt install -y sqlite3

# App directory owned by the deploy user (use your SSH user)
sudo mkdir -p /opt/learn-helper
sudo chown -R $USER:$USER /opt/learn-helper
```

If using Aliyun/Tencent Cloud, also open port 8080 in the security group (and on the OS):

```bash
sudo ufw allow 8080/tcp
```

## 1. Build & deploy (run on Mac)

```bash
cd <project-root>

# Build the frontend
cd frontend && pnpm install && pnpm build && cd ..

# Stage the built dist so go:embed picks it up
mkdir -p backend/cmd/server/dist
rsync -a --delete frontend/dist/ backend/cmd/server/dist/

# Cross-compile for Linux/amd64 (drop GOOS/GOARCH if building on the VPS)
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o learn-helper ./cmd/server
cd ..

# Push to VPS
scp backend/learn-helper ubuntu@<VPS_IP>:/tmp/learn-helper.new
```

## 2. Install & start (run on VPS)

```bash
# Install
sudo install -m 755 /tmp/learn-helper.new /opt/learn-helper/learn-helper

# Start (HTTPS_PROXY is required only if the VPS is in a region where the AI
# provider's host is blocked. Drop those two lines otherwise.)
cd /opt/learn-helper
HTTPS_PROXY=http://127.0.0.1:11111 HTTP_PROXY=http://127.0.0.1:11111 \
  nohup ./learn-helper > server.log 2>&1 &

# Verify
sleep 2 && curl http://127.0.0.1:8080/health
# → "ok"
```

Visit `http://<VPS_IP>:8080/` in a browser. Configure the AI provider key in the web UI on first use.

## 3. Sync the database

When you have notes locally that you want on the server (or vice versa).

**Local → VPS** (push):
```bash
# Stop VPS service
ssh ubuntu@<VPS_IP> 'pkill learn-helper; sleep 1'

# Take a consistent backup of the local DB
sqlite3 backend/learn-helper.db ".backup /tmp/lh-upload.db"

# Upload
scp /tmp/lh-upload.db ubuntu@<VPS_IP>:/tmp/

# Replace on VPS — IMPORTANT: remove the -shm and -wal alongside the .db
ssh ubuntu@<VPS_IP> <<'EOF'
  sudo rm -f /opt/learn-helper/learn-helper.db /opt/learn-helper/learn-helper.db-shm /opt/learn-helper/learn-helper.db-wal
  sudo cp /tmp/lh-upload.db /opt/learn-helper/learn-helper.db
  sudo chown $USER:$USER /opt/learn-helper/learn-helper.db
  cd /opt/learn-helper && HTTPS_PROXY=http://127.0.0.1:11111 \
    nohup ./learn-helper > server.log 2>&1 &
EOF
```

**VPS → Local** (pull): swap the directions in the steps above.

> Always remove all three files (`.db`, `.db-shm`, `.db-wal`) when replacing — leftover shm/wal from the old DB makes SQLite think the new DB is corrupt and you'll see only the empty default 概览 page.

## 4. Update / re-deploy

```bash
# Mac
cd <project-root> && git pull
cd frontend && pnpm build && cd ..
mkdir -p backend/cmd/server/dist
rsync -a --delete frontend/dist/ backend/cmd/server/dist/
cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o learn-helper ./cmd/server && cd ..
scp backend/learn-helper ubuntu@<VPS_IP>:/tmp/learn-helper.new
ssh ubuntu@<VPS_IP> 'pkill learn-helper; sudo install -m 755 /tmp/learn-helper.new /opt/learn-helper/learn-helper; cd /opt/learn-helper && HTTPS_PROXY=http://127.0.0.1:11111 nohup ./learn-helper > server.log 2>&1 &'
```

## Troubleshooting

- **502 from outside, but `curl 127.0.0.1:8080/health` works on the VPS** → Tencent Cloud / Aliyun security group is blocking 8080. Add an inbound rule.
- **`SPA not loaded` on the catch-all route** → Your binary is older; rebuild with the latest `fs.Sub`-based embed. `go test ./internal/handler/ -run TestGetDistIndexHTML` should pass.
- **Copy-link button is greyed out on a public-share page** → The page has no `share_token` yet. Visit the page once as the owner to generate one.
- **AI calls fail with `EOF`** → VPS can't reach the AI provider. Set `HTTPS_PROXY` (and `HTTP_PROXY`) when starting the binary, as shown above.
- **`/assets/*.js` 404 in browser console, page is blank** → The embed FS isn't being served; the binary on the VPS is older. Re-deploy.
