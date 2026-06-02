# LLM Wiki Operations Runbook

Day-2 operations: deploying, restarting, rolling back, restoring, troubleshooting.

## Deploy

Just `git push origin main`. CI builds, deploys via SSH atomically, runs a health check. Watch the Actions tab.

If you need to deploy a specific commit locally without going through CI:

```bash
# Build locally
pnpm install
pnpm --filter learn-helper-frontend build
mkdir -p backend/cmd/server/dist
rsync -a --delete frontend/dist/ backend/cmd/server/dist/
cd backend && CGO_ENABLED=0 go build -ldflags="-s -w" -o learn-helper ./cmd/server

# Copy to server
scp backend/learn-helper learnhelper@HOST:/tmp/learn-helper.new
ssh learnhelper@HOST 'sudo install -m 0755 /tmp/learn-helper.new /opt/learn-helper/learn-helper && sudo systemctl restart learn-helper'
```

## Restart the service

```bash
ssh learnhelper@HOST 'sudo systemctl restart learn-helper'
```

Downtime: ~1-2 seconds. Active SSE streams will be interrupted.

## Roll back to the previous binary

The previous binary is always kept at `/opt/learn-helper/learn-helper.bak` (kept fresh on every CI deploy).

```bash
ssh learnhelper@HOST 'sudo install -m 0755 /opt/learn-helper/learn-helper.bak /opt/learn-helper/learn-helper && sudo systemctl restart learn-helper'
```

## Restore from a SQLite backup

```bash
# 1. List backups
ssh learnhelper@HOST 'ls -lh /opt/learn-helper/backups/'

# 2. Stop the service
ssh learnhelper@HOST 'sudo systemctl stop learn-helper'

# 3. Restore (preserve the broken one as .broken)
ssh learnhelper@HOST 'sudo mv /opt/learn-helper/learn-helper.db /opt/learn-helper/learn-helper.db.broken && sudo cp /opt/learn-helper/backups/lh-YYYY-MM-DD-HHMM.db /opt/learn-helper/learn-helper.db && sudo chown learnhelper:learnhelper /opt/learn-helper/learn-helper.db'

# 4. Start
ssh learnhelper@HOST 'sudo systemctl start learn-helper'

# 5. Verify
./scripts/verify-deploy.sh https://yourdomain.top
```

## View logs

```bash
# Live tail
ssh learnhelper@HOST 'sudo journalctl -u learn-helper -f'

# Last 200 lines
ssh learnhelper@HOST 'sudo journalctl -u learn-helper -n 200 --no-pager'

# Only errors
ssh learnhelper@HOST 'sudo journalctl -u learn-helper -p err --no-pager'

# Caddy logs
ssh learnhelper@HOST 'sudo journalctl -u caddy -f'
```

## Common problems

### "Health check failed" during deploy

The deploy rolled itself back automatically. Check:

```bash
ssh learnhelper@HOST 'sudo journalctl -u learn-helper -n 100 --no-pager'
```

Common causes:
- **Schema migration failed**: if you changed SQL in `main.go`'s `schemaSQL`, an old DB might fail to apply new constraints. Restore from backup and check the migration.
- **Port already in use**: another process is on 8080. `ssh learnhelper@HOST 'sudo lsof -i :8080'`.
- **Permission denied on DB**: `.env` paths wrong, or `learn-helper.db` owned by `root` from a manual copy.

### Site shows 502 Bad Gateway

Caddy can't reach Go on 127.0.0.1:8080. The Go process crashed or never started.

```bash
ssh learnhelper@HOST 'sudo systemctl status learn-helper'
ssh learnhelper@HOST 'sudo journalctl -u learn-helper -n 50 --no-pager'
```

### Share link preview doesn't show in IM

1. Verify og: meta: `curl -s 'https://yourdomain.top/share/<slug>?t=<token>' | grep og:`
2. IM crawlers cache aggressively. Test with `https://www.opengraph.xyz/url/https%3A%2F%2Fyourdomain.top%2Fshare%2F<slug>` (or any OG preview tool) to see what the crawler sees.
3. Make sure the URL is `https://` (not `http://`). Most IM clients refuse to preview plain http.
4. Cloudflare may cache — set Bypass Cache for `/share/*` in Cloudflare Page Rules.

### Caddy says "acme: error presenting challenge"

DNS is not pointing at the VPS, or port 80 is blocked. Check:

```bash
dig +short yourdomain.top      # should match VPS IP
ssh learnhelper@HOST 'sudo ufw status'   # should show 80/tcp ALLOW
```

### Disk full

```bash
ssh learnhelper@HOST 'df -h /opt'
```

WAL files can grow. Force a checkpoint:

```bash
ssh learnhelper@HOST 'sudo -u learnhelper sqlite3 /opt/learn-helper/learn-helper.db "PRAGMA wal_checkpoint(TRUNCATE);"'
```

Then prune old backups if needed (the daily script keeps 7 days, but WAL may bloat if disk fills before that):

```bash
ssh learnhelper@HOST 'ls -lhS /opt/learn-helper/backups/ | tail -5'
```

## Manually take a backup

```bash
ssh learnhelper@HOST 'sudo /etc/cron.daily/learn-helper-backup'
ls -lh /opt/learn-helper/backups/   # on the server
```

## Update Caddy config

```bash
# Local: edit infra/caddy/Caddyfile, commit, push.
git push origin main    # only deploys the binary; Caddyfile is on the server
```

To push the Caddyfile to the server, either:
- Add a deploy step to `.github/workflows/deploy.yml` that `scp`s it and `systemctl reload caddy`
- Or manually: `scp infra/caddy/Caddyfile learnhelper@HOST:/tmp/Caddyfile && ssh learnhelper@HOST 'sudo cp /tmp/Caddyfile /etc/caddy/Caddyfile && sudo systemctl reload caddy'`
