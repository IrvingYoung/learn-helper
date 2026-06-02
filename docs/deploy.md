# Deploy LLM Wiki to a VPS

This guide walks you through the **first-time** deploy of LLM Wiki to a single Linux VPS. After it's done once, future deploys are `git push` to `main`.

## 0. Prerequisites

- A Linux VPS (Ubuntu 24.04 LTS recommended), 2 vCPU / 4 GB RAM / 40 GB SSD minimum
- A public static IPv4
- A domain you control (a `.top` costs ~$1–2/year at Cloudflare Registrar)
- SSH access to the VPS as a user with `sudo` rights

## 1. Domain & DNS

1. Buy a `.top` domain (or any cheap TLD) at Cloudflare Registrar / Porkbun / NameSilo.
2. If you used a non-Cloudflare registrar, point NS records to Cloudflare.
3. In Cloudflare DNS, add:
   - `A yourdomain.top → <VPS IP>` (proxy off — gray cloud)
4. Set Cloudflare SSL/TLS to **Full (Strict)**.
5. Verify: `dig +short yourdomain.top` returns your VPS IP.

## 2. Server bootstrap

SSH to the VPS as `root` (or your sudo user):

```bash
# Create deploy user
sudo useradd -m -s /bin/bash learnhelper
sudo mkdir -p /opt/learn-helper/backups
sudo chown -R learnhelper:learnhelper /opt/learn-helper

# Install Go runtime, sqlite3, Caddy
sudo apt update
sudo apt install -y sqlite3 caddy

# Go 1.25 — install via official tarball (apt is usually behind)
GO_VERSION=1.25.5
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | sudo tar -C /usr/local -xz
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh

# Open firewall
sudo ufw default deny
sudo ufw allow ssh
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable

# Harden SSH (after you've added your deploy key!)
sudo sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sudo systemctl restart sshd
```

## 3. Configure Caddy

Edit the `infra/caddy/Caddyfile` in the repo to replace `yourdomain.top` with your actual domain. Then:

```bash
# On the server
sudo tee /etc/caddy/Caddyfile >/dev/null <<'EOF'
# contents of infra/caddy/Caddyfile, after you edited it
EOF
sudo systemctl restart caddy
sudo journalctl -u caddy -f    # confirm cert issued
```

## 4. Configure systemd

```bash
sudo cp infra/systemd/learn-helper.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable learn-helper   # auto-start on boot
```

## 5. Configure the .env

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

## 6. Configure backups

```bash
sudo cp infra/backup/learn-helper-backup.sh /etc/cron.daily/learn-helper-backup
sudo chmod 755 /etc/cron.daily/learn-helper-backup
# Test it
sudo /etc/cron.daily/learn-helper-backup
ls -la /opt/learn-helper/backups/
```

## 7. Configure CI secrets

In your GitHub repo, go to **Settings → Secrets and variables → Actions** and add:

- `DEPLOY_HOST` — your VPS IP
- `DEPLOY_USER` — `learnhelper`
- `SSH_PRIVATE_KEY` — paste the private key (see step 8 for the public key part)

## 8. Set up the deploy key

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

## 9. First manual deploy

```bash
# On your local machine
cd <repo>
git push origin main    # triggers CI + deploy workflow
```

Watch the Actions tab. When deploy completes:

```bash
./scripts/verify-deploy.sh https://yourdomain.top
```

Expected: all checks pass. If something fails, see `docs/runbook.md`.

## 10. Confirm

- Open `https://yourdomain.top/` in a browser
- Log in, write a wiki page, refresh to confirm persistence
- `sudo systemctl status learn-helper` shows active
- `sudo journalctl -u learn-helper -n 50` shows clean logs
- `ls /opt/learn-helper/backups/` shows at least one snapshot after 24h
