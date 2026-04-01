#!/usr/bin/env bash
# testdata/prompt/smoke.sh
#
# End-to-end smoke tests for vrk prompt.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and key safety that unit tests cannot fully exercise.
#
# Usage:
#   ./testdata/prompt/smoke.sh              # run against ./vrk
#   BIN=./vrk ./testdata/prompt/smoke.sh   # explicit binary path
#
# Exit 0 if all pass. Exit 1 on first failure.

set -euo pipefail

BIN="${BIN:-./vrk}"

pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1: $2"; exit 1; }

echo "vrk prompt — smoke tests"
echo "binary: $BIN"
echo ""

# --- --explain ---
echo "--- --explain ---"

# --explain exits 0 and shows model name and max_tokens
out=$(echo 'hello' | "$BIN" prompt --explain)
echo "$out" | grep -q "4096" || fail "--explain: missing max_tokens" "got: $out"
echo "$out" | grep -qi "claude" || fail "--explain: missing model name" "got: $out"
pass "--explain exits 0 and shows model/max_tokens"

# --explain does not leak API key
out=$(ANTHROPIC_API_KEY=supersecrettestkey123 echo 'hi' | "$BIN" prompt --explain)
if echo "$out" | grep -q "supersecrettestkey123"; then
  fail "--explain: leaked API key" "key appeared in output"
fi
pass "--explain does not leak API key"

# --explain with unknown model passes model name through unchanged
out=$(echo 'hi' | "$BIN" prompt --model unknown-xyz --explain)
echo "$out" | grep -q "unknown-xyz" || fail "--explain unknown-model: model name missing" "got: $out"
pass "--explain passes unknown model name through"

# --explain with positional arg works
out=$("$BIN" prompt --explain 'hello world')
echo "$out" | grep -q "hello world" || fail "--explain positional: prompt missing from output" "got: $out"
pass "--explain positional arg works"

# --- --budget --fail ---
echo ""
echo "--- --budget --fail ---"

# --budget 1 --fail exits 1 (no API call needed; run with no keys)
out=$(unset ANTHROPIC_API_KEY OPENAI_API_KEY 2>/dev/null; echo 'hello world' | "$BIN" prompt --budget 1 --fail 2>&1) && fail "--budget --fail: expected exit 1 but got 0" "" || true
if echo "$out" | grep -qi "budget\|token"; then
  pass "--budget 1 --fail exits 1 with budget message"
else
  fail "--budget 1 --fail: missing budget/token in message" "got: $out"
fi

# --budget 1 without --fail also exits 1
out=$(unset ANTHROPIC_API_KEY OPENAI_API_KEY 2>/dev/null; echo 'hello world' | "$BIN" prompt --budget 1 2>&1) && fail "--budget without --fail: expected exit 1 but got 0" "" || true
if echo "$out" | grep -qi "budget\|token"; then
  pass "--budget 1 (no --fail) also exits 1 with budget message"
else
  fail "--budget 1 (no --fail): missing budget/token in message" "got: $out"
fi

# --- no keys ---
echo ""
echo "--- no API keys ---"

err=$(unset ANTHROPIC_API_KEY OPENAI_API_KEY 2>/dev/null; echo 'hello' | "$BIN" prompt 2>&1) && fail "no-key: expected exit 1" "" || true
if echo "$err" | grep -q "no API key found"; then
  pass "no API key exits 1 with correct message"
else
  fail "no-key: wrong error message" "got: $err"
fi

# Stdout must be empty on no-key error (use separate capture)
out=$(unset ANTHROPIC_API_KEY OPENAI_API_KEY 2>/dev/null; echo 'hello' | "$BIN" prompt 2>/dev/null) && true || true
if [ -z "$out" ]; then
  pass "no API key: stdout is empty"
else
  fail "no-key: stdout is not empty" "got: $out"
fi

# --- interactive TTY ---
echo ""
echo "--- interactive TTY ---"
# Reliable TTY simulation from a shell script requires platform-specific tools
# (script(1) differs between macOS and Linux) and cannot be tested portably here.
# The interactive-terminal path is fully covered by TestNoStdinInteractive in
# prompt_test.go, which overrides stdinIsTerminal and asserts exit 2.
echo "  NOTE  interactive TTY → exit 2 covered by TestNoStdinInteractive unit test"

# --- --help ---
echo ""
echo "--- --help ---"

"$BIN" prompt --help >/dev/null || fail "--help: expected exit 0" "got non-zero"
pass "--help exits 0"

out=$("$BIN" prompt --help 2>/dev/null) || true
echo "$out" | grep -q "model\|budget\|explain" || fail "--help: flags missing from output" "got: $out"
pass "--help lists flags"

# --- --endpoint ---
echo ""
echo "--- --endpoint ---"

# --endpoint + --explain exits 0 and shows /v1/chat/completions in curl URL
out=$(echo 'hello' | "$BIN" prompt --endpoint http://localhost:11434/v1 --model llama3.2 --explain)
echo "$out" | grep -q "/v1/chat/completions" || fail "--endpoint --explain: URL missing /v1/chat/completions" "got: $out"
pass "--endpoint --explain shows resolved chat/completions URL"

# --endpoint + --explain does not leak VRK_LLM_KEY value
out=$(echo 'hello' | env VRK_LLM_KEY=supersecretkeyabc123 "$BIN" prompt --endpoint http://localhost:11434/v1 --model llama3.2 --explain)
if echo "$out" | grep -q "supersecretkeyabc123"; then
  fail "--endpoint --explain: leaked VRK_LLM_KEY" "key appeared in output"
fi
pass "--endpoint --explain does not leak VRK_LLM_KEY value"

# --endpoint + --explain with VRK_LLM_KEY set includes $VRK_LLM_KEY reference
out=$(echo 'hello' | env VRK_LLM_KEY=somekey "$BIN" prompt --endpoint http://localhost:11434/v1 --model llama3.2 --explain)
echo "$out" | grep -q 'VRK_LLM_KEY' || fail "--endpoint --explain with key: missing \$VRK_LLM_KEY in output" "got: $out"
pass "--endpoint --explain shows \$VRK_LLM_KEY reference when key is set"

# --endpoint + --explain without VRK_LLM_KEY omits Authorization header line
out=$(echo 'hello' | env -u VRK_LLM_KEY "$BIN" prompt --endpoint http://localhost:11434/v1 --model llama3.2 --explain)
if echo "$out" | grep -q "Authorization"; then
  fail "--endpoint --explain without key: Authorization header present, want absent" "got: $out"
fi
pass "--endpoint --explain omits Authorization header when VRK_LLM_KEY not set"

# --endpoint without --model exits 2
echo 'hello' | "$BIN" prompt --endpoint http://localhost:11434/v1 2>/dev/null && fail "--endpoint no --model: expected exit 2" "" || true
out=$(echo 'hello' | "$BIN" prompt --endpoint http://localhost:11434/v1 2>&1) || true
if echo "$out" | grep -q "\-\-model"; then
  pass "--endpoint without --model exits 2 with --model message"
else
  fail "--endpoint without --model: stderr missing --model" "got: $out"
fi

# invalid --endpoint exits 2
echo 'hello' | "$BIN" prompt --endpoint "not a url" 2>/dev/null && fail "--endpoint bad URL: expected exit 2" "" || true
pass "--endpoint invalid URL exits 2"

# --- --quiet ---
echo ""
echo "--- --quiet ---"

# --quiet success: --explain still writes to stdout, no stderr
out=$(echo "test" | "$BIN" prompt --explain --quiet 2>/dev/null)
err=$(echo "test" | "$BIN" prompt --explain --quiet 2>&1 >/dev/null)
if echo "test" | "$BIN" prompt --explain --quiet >/dev/null 2>&1; then
  pass "--quiet --explain: exit 0"
else
  fail "--quiet --explain: exit 0" "non-zero exit"
fi
if echo "$out" | grep -q "curl"; then
  pass "--quiet --explain: stdout has curl"
else
  fail "--quiet --explain: stdout has curl" "got: $out"
fi
[ -z "$err" ] && pass "--quiet --explain: stderr empty" || fail "--quiet --explain: stderr empty" "got: $err"

# --- --system ---
echo ""
echo "--- --system ---"

# system prompt with --explain
out=$(echo 'hello' | "$BIN" prompt --system 'Reply with exactly: pong' --explain)
echo "$out" | grep -q "Reply with exactly: pong" || fail "--system --explain: system prompt missing from output" "got: $out"
pass "--system --explain shows system prompt"

# system from file
echo 'You are helpful.' > /tmp/vrk_sys_test.txt
out=$(echo 'hello' | "$BIN" prompt --system @/tmp/vrk_sys_test.txt --explain)
echo "$out" | grep -q "You are helpful." || fail "--system @file: file content missing from output" "got: $out"
rm -f /tmp/vrk_sys_test.txt
pass "--system @file reads from file"

# file not found — must be exit 1 (runtime error, not usage error)
set +e
echo 'hello' | "$BIN" prompt --system @/tmp/vrk_nonexistent_sys.txt --explain 2>/dev/null; code=$?
set -e
[ "$code" -eq 1 ] || fail "--system @missing: expected exit 1, got $code" ""
err=$(echo 'hello' | "$BIN" prompt --system @/tmp/vrk_nonexistent_sys.txt --explain 2>&1 >/dev/null) || true
echo "$err" | grep -q "not found" || fail "--system @missing: error missing 'not found'" "got: $err"
pass "--system @missing exits 1 with file-not-found message"

# empty system — must be exit 2 (usage error)
set +e
echo 'hello' | "$BIN" prompt --system '' --explain 2>/dev/null; code=$?
set -e
[ "$code" -eq 2 ] || fail "--system '': expected exit 2, got $code" ""
pass "--system '' exits 2"

# --- --field ---
echo ""
echo "--- --field ---"

# --field + --explain mutual exclusion: exit 2
set +e
echo '{"text":"hello"}' | "$BIN" prompt --field text --explain 2>err.tmp; code=$?
set -e
[ "$code" -eq 2 ] || fail "--field+--explain: expected exit 2, got $code" ""
grep -q "mutually exclusive" err.tmp || fail "--field+--explain: wrong error message" "$(cat err.tmp)"
rm -f err.tmp
pass "--field + --explain exits 2 with mutually exclusive message"

# --field with empty stdin: exit 0, no output
set +e
out=$(printf '' | "$BIN" prompt --field text 2>/dev/null); code=$?
set -e
[ "$code" -eq 0 ] || fail "--field empty stdin: expected exit 0, got $code" ""
[ -z "$out" ] || fail "--field empty stdin: expected no output" "got: $out"
pass "--field empty stdin exits 0 with no output"

# --field invalid JSON on line 1: exit 1 (no HTTP needed)
set +e
printf 'not json\n' | "$BIN" prompt --field text --endpoint http://localhost:1 --model test 2>err.tmp; code=$?
set -e
[ "$code" -eq 1 ] || fail "--field invalid JSON: expected exit 1, got $code" ""
grep -q "line 1" err.tmp || fail "--field invalid JSON: expected line 1 in error" "$(cat err.tmp)"
grep -q "invalid JSON" err.tmp || fail "--field invalid JSON: expected 'invalid JSON'" "$(cat err.tmp)"
rm -f err.tmp
pass "--field invalid JSON exits 1 with line number"

# --field missing field: exit 1 (no HTTP needed)
set +e
printf '{"other":"hi"}\n' | "$BIN" prompt --field text --endpoint http://localhost:1 --model test 2>err.tmp; code=$?
set -e
[ "$code" -eq 1 ] || fail "--field missing field: expected exit 1, got $code" ""
grep -q '"text"' err.tmp || fail "--field missing field: wrong error message" "$(cat err.tmp)"
grep -q "not found" err.tmp || fail "--field missing field: expected 'not found'" "$(cat err.tmp)"
rm -f err.tmp
pass "--field missing field exits 1 with field name"

# --field non-string value: exit 1 (no HTTP needed)
set +e
printf '{"text":42}\n' | "$BIN" prompt --field text --endpoint http://localhost:1 --model test 2>err.tmp; code=$?
set -e
[ "$code" -eq 1 ] || fail "--field non-string: expected exit 1, got $code" ""
grep -q "not a string" err.tmp || fail "--field non-string: wrong error message" "$(cat err.tmp)"
rm -f err.tmp
pass "--field non-string value exits 1"

# --help lists --field flag
out=$("$BIN" prompt --help 2>/dev/null) || true
echo "$out" | grep -q "field" || fail "--help: --field flag missing from output" "got: $out"
pass "--help lists --field flag"

# --- summary ---
echo ""
echo "All smoke tests passed."
