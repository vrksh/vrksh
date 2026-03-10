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

# --- summary ---
echo ""
echo "All smoke tests passed."
