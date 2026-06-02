# Deploy LLM Wiki

One command. Docker.

## 1. Install Docker on the VPS

Skip if already installed. On Ubuntu:

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker
```

## 2. Clone and run

```bash
git clone https://github.com/IrvingYoung/learn-helper.git
cd learn-helper
docker compose up -d --build
```

That's it. The app is at `http://<VPS_IP>:8080`.

## 3. Open the firewall

```bash
sudo ufw allow 8080/tcp
```

## Updating

```bash
cd learn-helper
git pull
docker compose up -d --build
```

## Data

SQLite database is in `./data/learn-helper.db` on the host (mounted as `/app/data` in the container). Delete that file to reset.

## API keys

Configure in the web UI after first login — no env vars needed.
