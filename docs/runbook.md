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
./scripts/verify-deploy.sh http://<VPS_IP>:8080
```

## View logs

```bash
# Live tail
ssh learnhelper@HOST 'sudo journalctl -u learn-helper -f'

# Last 200 lines
ssh learnhelper@HOST 'sudo journalctl -u learn-helper -n 200 --no-pager'

# Only errors
ssh learnhelper@HOST 'sudo journalctl -u learn-helper -p err --no-pager'
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

### Site shows connection refused / 502

The Go process crashed or never started. Without Caddy, a 502 from a reverse proxy isn't possible — you'd see "connection refused" or a blank page.

```bash
ssh learnhelper@HOST 'sudo systemctl status learn-helper'
ssh learnhelper@HOST 'sudo journalctl -u learn-helper -n 50 --no-pager'
```

### Share link preview doesn't show in IM

1. Verify og: meta: `curl -s 'http://<VPS_IP>:8080/share/<slug>?t=<token>' | grep og:`
2. **IM previews require HTTPS** — without a domain + TLS, no IM client will show a preview card. This is by design. See `docs/deploy.md` → "Upgrading to HTTPS + domain" when you have a domain.

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
