#!/bin/bash
#
# Post-deploy smoke test. Run from a dev machine after `git push` and CI deploy.
#
# Usage:
#   ./scripts/verify-deploy.sh http://VPS_IP:8080
#
# Default (if no arg): http://127.0.0.1:8080 — useful for local testing.
#
set -euo pipefail

HOST="${1:-http://127.0.0.1:8080}"
fail=0

note() { printf "  %-40s %s\n" "$1" "$2"; }

# 1. HTTP root reachable
code=$(curl -s -o /dev/null -w '%{http_code}' "$HOST/")
if [[ "$code" == "200" ]]; then note "GET /"  "OK ($code)";
else note "GET /"  "FAIL ($code)"; fail=1; fi

# 2. /health
code=$(curl -s -o /dev/null -w '%{http_code}' "$HOST/health")
if [[ "$code" == "200" ]]; then note "GET /health"  "OK ($code)";
else note "GET /health"  "FAIL ($code)"; fail=1; fi

# 3. /share/ (SSR + og: meta). We use a fake slug — expect 404 but the response
#    should still be HTML (not JSON or 502).
body=$(curl -s "$HOST/share/__nonexistent__")
if echo "$body" | grep -qi 'og:'; then
  note "GET /share/<slug> (og: meta)"  "OK"
else
  note "GET /share/<slug> (og: meta)"  "WARN: no og: meta in response"
fi

# 4. Public ports scan (SSH + 8080 only).
#    Strip scheme and port from $HOST to get just the hostname/IP.
host_only="${HOST#http://}"; host_only="${host_only#https://}"
host_only="${host_only%%/*}"; host_only="${host_only%%:*}"
ports=$(nmap -p 22,8080 "$host_only" 2>/dev/null \
  | awk '/^[0-9]+\/tcp/{sub(/\/tcp$/,"",$1); print $1}' | sort -nu) || ports=""
expected=$'22\n8080'
if [[ "$ports" == "$expected" ]]; then
  note "Ports 22/8080 only"  "OK"
else
  note "Ports 22/8080 only"  "WARN: ports = $(printf '%s' "$ports" | tr '\n' ',')"
fi

if [[ "$fail" -ne 0 ]]; then
  echo "FAIL: smoke test failed"
  exit 1
fi
echo "OK: smoke test passed"
