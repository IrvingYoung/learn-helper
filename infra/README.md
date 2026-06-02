# `infra/` — server-side files

These files are deployed to the VPS by an operator (manually or via a future automation). They are **not** part of the running Go binary.

| File | Installed at | Purpose | Used in current setup? |
|---|---|---|---|
| `systemd/learn-helper.service` | `/etc/systemd/system/learn-helper.service` | systemd unit, runs Go binary | ✅ Yes |
| `caddy/Caddyfile` | `/etc/caddy/Caddyfile` | reverse proxy + TLS | ⏸ Not yet (waiting for a domain) |
| `backup/learn-helper-backup.sh` | `/etc/cron.daily/learn-helper-backup` | daily SQLite snapshot | ✅ Yes |

**Current setup: HTTP + IP, no reverse proxy.** The Go binary binds `0.0.0.0:8080` directly. When you buy a domain, the Caddyfile is ready to go — see `docs/deploy.md` → "Upgrading to HTTPS + domain".

For full setup, see [`../docs/deploy.md`](../docs/deploy.md).
For day-2 ops, see [`../docs/runbook.md`](../docs/runbook.md).
