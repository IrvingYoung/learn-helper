#!/bin/bash
#
# Build the LLM Wiki Go binary (with the embedded SPA) and push it to a VPS.
#
# Usage:
#   VPS_HOST=124.222.4.227 VPS_USER=ubuntu ./scripts/deploy.sh
#
# Optional env vars:
#   VPS_HOST        (required) — VPS IP or hostname
#   VPS_USER        (default: ubuntu)
#   VPS_PORT        (default: 22)
#   VPS_BIN_DIR     (default: /opt/learn-helper) — install path on the VPS
#   HTTPS_PROXY     (default: unset) — e.g. http://127.0.0.1:11111 if VPS
#                   needs a proxy to reach the AI provider
#
set -euo pipefail

cd "$(dirname "$0")/.."
PROJECT_ROOT="$(pwd)"

: "${VPS_HOST:?Usage: VPS_HOST=<ip> $0}"
VPS_USER="${VPS_USER:-ubuntu}"
VPS_PORT="${VPS_PORT:-22}"
VPS_BIN_DIR="${VPS_BIN_DIR:-/opt/learn-helper}"

REMOTE_SCRIPT="scripts/deploy-remote.sh"

echo "==> Building frontend"
cd frontend
pnpm install --frozen-lockfile
pnpm build
cd "$PROJECT_ROOT"

echo "==> Staging frontend dist for embed"
mkdir -p backend/cmd/server/dist
rsync -a --delete frontend/dist/ backend/cmd/server/dist/

echo "==> Building Go binary (linux/amd64)"
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o learn-helper ./cmd/server
cd "$PROJECT_ROOT"

echo "==> Pushing binary to ${VPS_USER}@${VPS_HOST}"
ssh -p "$VPS_PORT" "${VPS_USER}@${VPS_HOST}" "pkill -f '$VPS_BIN_DIR/learn-helper' || true; sleep 1"
scp -P "$VPS_PORT" backend/learn-helper "${VPS_USER}@${VPS_HOST}:/tmp/learn-helper.new"
scp -P "$VPS_PORT" "$REMOTE_SCRIPT" "${VPS_USER}@${VPS_HOST}:/tmp/deploy-remote.sh"

echo "==> Running remote install script"
PROXY_PREFIX=""
if [[ -n "${HTTPS_PROXY:-}" ]]; then
  PROXY_PREFIX="HTTPS_PROXY=${HTTPS_PROXY} HTTP_PROXY=${HTTPS_PROXY}"
fi

ssh -p "$VPS_PORT" "${VPS_USER}@${VPS_HOST}" \
  "chmod +x /tmp/deploy-remote.sh && ${PROXY_PREFIX} /tmp/deploy-remote.sh /tmp/learn-helper.new '$VPS_BIN_DIR'"

echo "==> Done. Verify: http://${VPS_HOST}:8080/health"
