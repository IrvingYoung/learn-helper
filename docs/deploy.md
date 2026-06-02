# Deploy LLM Wiki to a VPS

This guide walks you through the **first-time** deploy of LLM Wiki to a single Linux VPS. After it's done once, future deploys are `git push` to `main`.

**Current setup: HTTP + IP (no domain).** The Go binary binds `0.0.0.0:8080` directly. There is no TLS, no reverse proxy. For personal use this is fine. **When you get a domain, see [Upgrading to HTTPS + domain](#upgrading-to-https--domain) at the bottom of this file.**

## 0. Prerequisites

- A Linux VPS (Ubuntu 24.04 LTS recommended), 2 vCPU / 4 GB RAM / 40 GB SSD minimum
- A public static IPv4
- SSH access to the VPS as a user with `sudo` rights

## 1. Server bootstrap

SSH to the VPS as `root` (or your sudo user):

```bash
# Create deploy user
sudo useradd -m -s /bin/bash learnhelper
sudo mkdir -p /opt/learn-helper/backups
sudo chown -R learnhelper:learnhelper /opt/learn-helper

# Install Go runtime + sqlite3 CLI
sudo apt update
sudo apt install -y sqlite3

# Go 1.25 — install via official tarball (apt is usually behind)
GO_VERSION=1.25.5
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | sudo tar -C /usr/local -xz
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh

# Open firewall — only SSH + the Go port
sudo ufw default deny
sudo ufw allow ssh
sudo ufw allow 8080/tcp
sudo ufw enable

# Harden SSH (after you've added your deploy key!)
sudo sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sudo systemctl restart sshd
```

## 2. Configure systemd

```bash
sudo cp infra/systemd/learn-helper.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable learn-helper   # auto-start on boot
```

## 3. Configure the .env

```bash
sudo tee /opt/learn-helper/.env >/dev/null <<'EOF'
PORT=8080
DB_PATH=/opt/learn-helper/learn-helper.db
LH_SKILLS_DIR=/opt/learn-helper/skills
LH_SPA_DIST=
ANTHROPIC_API_KEY=sk-ant-...
DEEPSEEK_API_KEY=sk-...
EOF
sudo chown learnhelper:learnhelper /opt/learn-helper/.env
sudo chmod 600 /opt/learn-helper/.env
```

## 4. Configure backups

```bash
sudo cp infra/backup/learn-helper-backup.sh /etc/cron.daily/learn-helper-backup
sudo chmod 755 /etc/cron.daily/learn-helper-backup
# Test it
sudo /etc/cron.daily/learn-helper-backup
ls -la /opt/learn-helper/backups/
```

## 5. Configure CI secrets

In your GitHub repo, go to **Settings → Secrets and variables → Actions** and add:

- `DEPLOY_HOST` — your VPS IP
- `DEPLOY_USER` — `learnhelper`
- `SSH_PRIVATE_KEY` — paste the private key (see step 8 for the public key part)

## 6. Set up the deploy key

On your **local** machine:

```bash
ssh-keygen -t ed25519 -C 'github-actions-deploy' -f ~/.ssh/learn-helper-deploy -N ''
cat ~/.ssh/learn-helper-deploy.pub
```

On the **server**:

```bash
sudo mkdir -p /home/learnhelper/.ssh
sudo tee /home/learnhelper/.ssh/authorized_keys >/dev/null <<'EOF'
# paste the .pub contents from above
EOF
sudo chown -R learnhelper:learnhelper /home/learnhelper/.ssh
sudo chmod 700 /home/learnhelper/.ssh
sudo chmod 600 /home/learnhelper/.ssh/authorized_keys
```

Paste the **private** key into the `SSH_PRIVATE_KEY` GitHub secret.

## 7. First manual deploy

```bash
# On your local machine
cd <repo>
git push origin main    # triggers CI + deploy workflow
```

Watch the Actions tab. When deploy completes:

```bash
./scripts/verify-deploy.sh http://<VPS_IP>:8080
```

Expected: all checks pass. If something fails, see `docs/runbook.md`.

## 8. Confirm

- Open `http://<VPS_IP>:8080/` in a browser
- Log in, write a wiki page, refresh to confirm persistence
- `sudo systemctl status learn-helper` shows active
- `sudo journalctl -u learn-helper -n 50` shows clean logs
- `ls /opt/learn-helper/backups/` shows at least one snapshot after 24h

---

## Upgrading to HTTPS + domain

When you buy a domain and want TLS, the migration is:

1. Buy a `.top` (or any) domain. Point an A record at your VPS IP.
2. On the server, install Caddy: `sudo apt install -y caddy`
3. Edit `infra/caddy/Caddyfile` to replace `yourdomain.top` with your real domain. Copy it to `/etc/caddy/Caddyfile` on the server.
4. **Change the Go binary's bind address** from `0.0.0.0:8080` to `127.0.0.1:8080`. Either:
   - Add `BIND_ADDR=127.0.0.1:8080` to the systemd unit's `ExecStart` (preferred), or
   - Edit the .env / source to bind loopback only.
5. Close port 8080 in the firewall: `sudo ufw delete allow 8080/tcp`
6. Open 80 and 443: `sudo ufw allow 80/tcp; sudo ufw allow 443/tcp`
7. `sudo systemctl restart caddy` — Caddy will auto-issue a Let's Encrypt cert.
8. Update `verify-deploy.sh` calls in CI / runbook to use `https://yourdomain.top`.

After the upgrade, the share-link og: meta will start showing in IM clients (微信/钉钉/Slack).
