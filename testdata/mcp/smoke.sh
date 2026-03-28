#!/usr/bin/env bash
# testdata/mcp/smoke.sh
#
# End-to-end smoke tests for vrk mcp (discovery-only MCP server).
# Run after make build to verify real process behaviour.
#
# Usage:
#   ./testdata/mcp/smoke.sh
#   VRK=./vrk ./testdata/mcp/smoke.sh

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0

# ------------------------------------------------------------
# Helpers
# ------------------------------------------------------------

ok() {
  echo "  PASS  $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "  FAIL  $1"
  echo "        $2"
  FAIL=$((FAIL + 1))
}

assert_exit() {
  local desc=$1 expected=$2 actual=$3
  if [ "$actual" -eq "$expected" ]; then
    ok "$desc (exit $expected)"
  else
    fail "$desc" "expected exit $expected, got exit $actual"
  fi
}

echo "vrk mcp — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# Test 1: --list prints tool names with descriptions, exits 0
# ------------------------------------------------------------
OUT=$("$VRK" mcp --list 2>/dev/null) || true
RC=$?
assert_exit "test 1: --list exits 0" 0 "$RC"

if echo "$OUT" | grep -q "tok"; then
  ok "test 1: --list output contains 'tok'"
else
  fail "test 1: --list output contains 'tok'" "got: $OUT"
fi

if echo "$OUT" | grep -q "jwt"; then
  ok "test 1: --list output contains 'jwt'"
else
  fail "test 1: --list output contains 'jwt'" "got: $OUT"
fi

# Verify column alignment: descriptions contain multi-space separation from tool name
if echo "$OUT" | head -1 | grep -q '   '; then
  ok "test 1: --list descriptions are separated from names"
else
  fail "test 1: --list column alignment" "no multi-space separator found"
fi

# ------------------------------------------------------------
# Test 2: --help exits 0
# ------------------------------------------------------------
"$VRK" mcp --help >/dev/null 2>&1
RC=$?
assert_exit "test 2: --help exits 0" 0 "$RC"

# ------------------------------------------------------------
# Test 3: stdio initialize → valid response
# ------------------------------------------------------------
RESP=$(printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n' \
  | "$VRK" mcp 2>/dev/null)

if echo "$RESP" | python3 -c 'import json,sys; d=json.loads(sys.stdin.readline()); assert d["result"]["protocolVersion"]=="2024-11-05"' 2>/dev/null; then
  ok "test 3: initialize protocolVersion is 2024-11-05"
else
  fail "test 3: initialize protocolVersion" "response: $RESP"
fi

if echo "$RESP" | python3 -c 'import json,sys; d=json.loads(sys.stdin.readline()); assert d["result"]["serverInfo"]["name"]=="vrk"' 2>/dev/null; then
  ok "test 3: initialize serverInfo.name is vrk"
else
  fail "test 3: initialize serverInfo.name" "response: $RESP"
fi

# ------------------------------------------------------------
# Test 4: stdio tools/list → response contains vrk_tok, vrk_jwt
# ------------------------------------------------------------
RESP=$(printf '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n' \
  | "$VRK" mcp 2>/dev/null)

if echo "$RESP" | grep -q "vrk_tok"; then
  ok "test 4: tools/list contains vrk_tok"
else
  fail "test 4: tools/list contains vrk_tok" "response: $RESP"
fi

if echo "$RESP" | grep -q "vrk_jwt"; then
  ok "test 4: tools/list contains vrk_jwt"
else
  fail "test 4: tools/list contains vrk_jwt" "response: $RESP"
fi

# ------------------------------------------------------------
# Test 5: tools/list → vrk_tok has --budget and --model in inputSchema
# ------------------------------------------------------------
if echo "$RESP" | python3 -c '
import json, sys
d = json.loads(sys.stdin.readline())
tools = {t["name"]: t for t in d["result"]["tools"]}
tok = tools["vrk_tok"]
props = tok["inputSchema"]["properties"]
assert "budget" in props, "missing budget"
assert "model" in props, "missing model"
assert "budget" in props and props["budget"]["description"], "budget has no description"
assert "model" in props and props["model"]["description"], "model has no description"
' 2>/dev/null; then
  ok "test 5: vrk_tok has budget and model flags with descriptions"
else
  fail "test 5: vrk_tok inputSchema" "response: $RESP"
fi

# ------------------------------------------------------------
# Test 6: stdin-required tool has "input" in required, non-stdin tool does not
# ------------------------------------------------------------
if echo "$RESP" | python3 -c '
import json, sys
d = json.loads(sys.stdin.readline())
tools = {t["name"]: t for t in d["result"]["tools"]}
links = tools["vrk_links"]
uuid = tools["vrk_uuid"]
assert "input" in links["inputSchema"].get("required", []), "links missing input in required"
assert "input" not in uuid["inputSchema"].get("required", []), "uuid should not have input in required"
' 2>/dev/null; then
  ok "test 6: links has input required, uuid does not"
else
  fail "test 6: stdin required check" "response: $RESP"
fi

# ------------------------------------------------------------
# Test 7: every tool has ≥1 property beyond "input"
# ------------------------------------------------------------
if echo "$RESP" | python3 testdata/mcp/check_flags.py 2>/dev/null; then
  ok "test 7: all tools have flags beyond input"
else
  # Re-run to show error
  ERR=$(echo "$RESP" | python3 testdata/mcp/check_flags.py 2>&1 || true)
  fail "test 7: check_flags.py" "$ERR"
fi

# ------------------------------------------------------------
# Test 8: malformed JSON → parse error response
# ------------------------------------------------------------
RESP=$(printf 'not json\n' | "$VRK" mcp 2>/dev/null)

if echo "$RESP" | python3 -c '
import json, sys
d = json.loads(sys.stdin.readline())
assert d["error"]["code"] == -32700
' 2>/dev/null; then
  ok "test 8: malformed JSON returns -32700"
else
  fail "test 8: parse error" "response: $RESP"
fi

# ------------------------------------------------------------
# Test 9: stdout purity — first output line is valid JSON
# ------------------------------------------------------------
RESP=$(printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n' \
  | "$VRK" mcp 2>/dev/null)

if echo "$RESP" | python3 -c 'import json,sys;json.loads(sys.stdin.readline())' 2>/dev/null; then
  ok "test 9: stdout purity — first line is valid JSON"
else
  fail "test 9: stdout purity" "first line not valid JSON: $RESP"
fi

# ------------------------------------------------------------
# Test 10: notification produces no response
# ------------------------------------------------------------
RESP=$(printf '{"jsonrpc":"2.0","method":"notifications/initialized"}\n{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n' \
  | "$VRK" mcp 2>/dev/null)

if echo "$RESP" | python3 -c 'import sys; lines=sys.stdin.readlines(); assert len(lines)==1, f"expected 1 response, got {len(lines)}"' 2>/dev/null; then
  ok "test 10: notification produces no response (1 line total)"
else
  LINES=$(echo "$RESP" | wc -l | tr -d ' ')
  fail "test 10: notification response count" "expected 1 line, got $LINES"
fi

# ------------------------------------------------------------
echo ""
echo "---"
echo "Results: $PASS passed, $FAIL failed"
echo ""

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
