#!/bin/bash
#
# Post-deploy smoke test. Run from a dev machine after `git push` and CI deploy.
#
# Usage:
#   ./scripts/verify-deploy.sh https://yourdomain.top
#
set -euo pipefail

HOST="${1:-https://yourdomain.top}"
fail=0

note() { printf "  %-40s %s\n" "$1" "$2"; }

# 1. HTTPS reachable
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

# 4. Backend NOT directly exposed
if curl -s --max-time 3 "$HOST:8080/" >/dev/null 2>&1; then
  note "Backend 127.0.0.1:8080 not exposed"  "FAIL (port reachable)"
  fail=1
else
  note "Backend 127.0.0.1:8080 not exposed"  "OK"
fi

# 5. Public ports scan
ports=$(nmap -p 22,80,443 "$HOST" 2>/dev/null | awk '/^PORT/{next} /^$/{exit} {print $1}' | sort -u)
if [[ "$ports" == "22"$'\n'"80"$'\n'"443" || "$ports" == $'22\n80\n443' ]]; then
  note "Ports 22/80/443 only"  "OK"
else
  note "Ports 22/80/443 only"  "WARN: extra ports open"
fi

if [[ "$fail" -ne 0 ]]; then
  echo "FAIL: smoke test failed"
  exit 1
fi
echo "OK: smoke test passed"
