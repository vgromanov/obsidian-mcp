#!/usr/bin/env bash
# Smoke tests for POST /local-smart-lookup/search/ (plan § Smoke test).
# Requires: OBSIDIAN_API_KEY, Obsidian REST API up, local-smart-lookup plugin routes registered.
set -euo pipefail

if [[ -z "${OBSIDIAN_API_KEY:-}" ]]; then
  if [[ -f "${HOME}/.cursor/mcp.json" ]]; then
    OBSIDIAN_API_KEY="$(python3 - <<'PY'
import json, os
p = os.path.expanduser("~/.cursor/mcp.json")
d = json.load(open(p))
print(d["mcpServers"]["obsidian-mcp"]["env"]["OBSIDIAN_API_KEY"])
PY
)"
    export OBSIDIAN_API_KEY
  fi
fi
: "${OBSIDIAN_API_KEY:?Set OBSIDIAN_API_KEY}"

BASE="${OBSIDIAN_BASE_URL:-https://127.0.0.1:27124}"
BASE="${BASE%/}"
QUERY="${SMOKE_QUERY:-local-first AI and user control}"
OMLX_BASE="${OMLX_BASE_URL:-http://127.0.0.1:8000/v1}"
OMLX_BASE="${OMLX_BASE%/}"
OMLX_KEY="${OMLX_API_KEY:-0000}"
CURL=(curl -sk)
if [[ "$BASE" == https://* ]]; then
  CURL+=(--insecure)
fi

pass=0
fail=0
skip=0

check_omlx() {
  echo "=== oMLX preflight (GET /models) ==="
  if code=$("${CURL[@]}" -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${OMLX_KEY}" "${OMLX_BASE}/models"); then
    echo "HTTP $code"
    if [[ "$code" == "200" ]]; then pass=$((pass + 1)); else fail=$((fail + 1)); fi
  else
    echo "unreachable"
    fail=$((fail + 1))
  fi
  echo
}

run_search() {
  local name="$1"
  local body="$2"
  echo "=== $name ==="
  local tmp
  tmp=$(mktemp)
  local code
  code=$("${CURL[@]}" -o "$tmp" -w "%{http_code}" -X POST "${BASE}/local-smart-lookup/search/" \
    -H "Authorization: Bearer ${OBSIDIAN_API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$body") || code=000
  echo "HTTP $code"
  head -c 600 "$tmp"
  echo
  if [[ "$code" == "200" ]]; then
    python3 - "$tmp" <<'PY' || true
import json, sys
d = json.load(open(sys.argv[1]))
r = d.get("results", d)
print("results_count:", len(r) if isinstance(r, list) else "n/a")
PY
    pass=$((pass + 1))
  elif [[ "$code" == "404" ]]; then
    echo "SKIP: route not found (enable local-smart-lookup + REST API extension)"
    skip=$((skip + 1))
  else
    fail=$((fail + 1))
  fi
  rm -f "$tmp"
  echo
}

check_omlx
run_search "baseline" "$(printf '{"query":"%s","limit":5}' "$QUERY")"
run_search "tags" "$(printf '{"query":"%s","tags":["research","ai"],"limit":5}' "$QUERY")"
run_search "frontmatter" "$(printf '{"query":"%s","frontmatter":{"status":"active","project":"Local Search"},"limit":5}' "$QUERY")"
run_search "dataviewSource" "$(python3 -c "import json; print(json.dumps({'query':'''$QUERY''','dataviewSource':'#research or \"Projects\"','limit':5}))")"
run_search "dataviewQuery" "$(python3 -c "import json; print(json.dumps({'query':'''$QUERY''','dataviewQuery':'LIST FROM #research WHERE status = \"active\"','limit':5}))")"
run_search "where" "$(printf '{"query":"%s","where":"type = '\''note'\''","limit":5}' "$QUERY")"
run_search "combined" "$(printf '{"query":"%s","tags":["research"],"frontmatter":{"status":"active"},"where":"type = '\''note'\''","limit":5}' "$QUERY")"
run_search "blank_query" '{"query":"","limit":5}'

echo "Summary: pass=$pass fail=$fail skip=$skip"
[[ "$fail" -eq 0 ]]
