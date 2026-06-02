# `infra/` — server-side files

These files are deployed to the VPS by an operator (manually or via a future automation). They are **not** part of the running Go binary.

| File | Installed at | Purpose |
|---|---|---|
| `systemd/learn-helper.service` | `/etc/systemd/system/learn-helper.service` | systemd unit, runs Go binary |
| `caddy/Caddyfile` | `/etc/caddy/Caddyfile` | reverse proxy + TLS |
| `backup/learn-helper-backup.sh` | `/etc/cron.daily/learn-helper-backup` | daily SQLite snapshot |

For full setup, see [`../docs/deploy.md`](../docs/deploy.md).
For day-2 ops, see [`../docs/runbook.md`](../docs/runbook.md).
