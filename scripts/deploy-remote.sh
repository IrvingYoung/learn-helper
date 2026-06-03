#!/bin/bash
#
# Install a freshly-uploaded learn-helper binary and (re)start the service.
# Run on the VPS as the deploy user (typically ubuntu).
#
# Usage:
#   ./deploy-remote.sh <path-to-new-binary> [install-dir]
#
# Defaults:
#   install-dir: /opt/learn-helper
#
# Honors $HTTPS_PROXY / $HTTP_PROXY in the environment so the launched
# process uses the proxy for outbound AI calls.
#
set -euo pipefail

NEW_BIN="${1:-/tmp/learn-helper.new}"
INSTALL_DIR="${2:-/opt/learn-helper}"
SERVICE_BIN="$INSTALL_DIR/learn-helper"

if [[ ! -f "$NEW_BIN" ]]; then
  echo "ERROR: new binary not found at $NEW_BIN" >&2
  exit 1
fi

if [[ ! -d "$INSTALL_DIR" ]]; then
  echo "ERROR: install dir $INSTALL_DIR does not exist" >&2
  exit 1
fi

echo "==> Stopping existing process (if any)"
# Avoid pkill -f with the binary path; it can match pkill's own argv and
# kill the surrounding shell. pgrep -x matches on process name only.
pid=$(pgrep -x learn-helper || true)
if [ -n "$pid" ]; then
  kill "$pid"
  sleep 1
fi

echo "==> Installing binary to $SERVICE_BIN"
# Use install(1) for atomic-ish replace (writes to a temp name, then renames).
# Falls back to plain install if the temp name is already taken.
TMP_BIN="${SERVICE_BIN}.new.$$"
install -m 0755 "$NEW_BIN" "$TMP_BIN"
mv -f "$TMP_BIN" "$SERVICE_BIN"

# Ensure owner matches the deploy user. Don't fail if chown fails (e.g. running
# as the deploy user already owns the file).
chown "$(id -un):$(id -gn)" "$SERVICE_BIN" 2>/dev/null || true

echo "==> Starting service from $INSTALL_DIR"
cd "$INSTALL_DIR"
nohup ./learn-helper > server.log 2>&1 &

# Wait for the health check to come up.
for i in 1 2 3 4 5; do
  sleep 1
  if curl -fsS http://127.0.0.1:8080/health >/dev/null 2>&1; then
    echo "==> Service is up"
    exit 0
  fi
done

echo "ERROR: service did not become healthy within 5s. Tail of server.log:" >&2
tail -20 server.log >&2
exit 1
