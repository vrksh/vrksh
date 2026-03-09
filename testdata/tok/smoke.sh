#!/usr/bin/env bash
# testdata/tok/smoke.sh
#
# End-to-end smoke tests for vrk tok.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/tok/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/tok/smoke.sh   # explicit binary path
#
# NOTE: TTY detection (exit 2 when run interactively with no pipe) cannot be
# automated here — /dev/null is not a character device, so os.ModeCharDevice
# is never set. Verify manually: run `vrk tok` in a terminal with no pipe.
#
# Exit 0 if all pass. Exit 1 on first failure.

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

assert_stdout_equals() {
  local desc=$1 expected=$2 actual=$3
  if [ "$actual" = "$expected" ]; then
    ok "$desc (stdout = '$expected')"
  else
    fail "$desc" "expected '$expected', got '$actual'"
  fi
}

assert_stdout_empty() {
  local desc=$1 actual=$2
  if [ -z "$actual" ]; then
    ok "$desc (stdout empty)"
  else
    fail "$desc" "expected empty stdout, got: $actual"
  fi
}

assert_stderr_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -q "$pattern"; then
    ok "$desc (stderr contains '$pattern')"
  else
    fail "$desc" "stderr did not contain '$pattern'. got: $actual"
  fi
}

assert_stdout_json_field() {
  local desc=$1 field=$2 expected=$3 actual=$4
  if command -v jq > /dev/null 2>&1; then
    got=$(echo "$actual" | jq -r ".$field" 2>/dev/null)
    if [ "$got" = "$expected" ]; then
      ok "$desc (.$field = '$expected')"
    else
      fail "$desc" ".$field = '$got', want '$expected'"
    fi
  else
    echo "  SKIP  $desc (jq not installed)"
  fi
}

echo "vrk tok — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Basic count — stdin form
# ------------------------------------------------------------
echo "--- basic count ---"

stdout=$(echo 'hello world' | "$VRK" tok 2>/dev/null)
exit_code=$(set +e; echo 'hello world' | "$VRK" tok > /dev/null 2>&1; echo $?)
assert_exit        "hello world: exit 0"        0    "$exit_code"
assert_stdout_equals "hello world: stdout = 2"  "2"  "$stdout"

# "Hello, world!" → 4 tokens
stdout=$(echo 'Hello, world!' | "$VRK" tok 2>/dev/null)
exit_code=$(set +e; echo 'Hello, world!' | "$VRK" tok > /dev/null 2>&1; echo $?)
assert_exit          "Hello, world!: exit 0"       0    "$exit_code"
assert_stdout_equals "Hello, world!: stdout = 4"  "4"  "$stdout"

# ------------------------------------------------------------
# 2. Basic count — positional arg form
# ------------------------------------------------------------
echo ""
echo "--- positional arg ---"

stdout=$("$VRK" tok 'hello world' 2>/dev/null)
exit_code=$(set +e; "$VRK" tok 'hello world' > /dev/null 2>&1; echo $?)
assert_exit        "arg: hello world: exit 0"       0    "$exit_code"
assert_stdout_equals "arg: hello world: stdout = 2" "2"  "$stdout"

# ------------------------------------------------------------
# 3. Empty input
# ------------------------------------------------------------
echo ""
echo "--- empty input ---"

# Empty pipe → 0 tokens, exit 0 (not a usage error)
stdout=$(echo '' | "$VRK" tok 2>/dev/null)
exit_code=$(set +e; echo '' | "$VRK" tok > /dev/null 2>&1; echo $?)
assert_exit        "empty pipe: exit 0"        0    "$exit_code"
assert_stdout_equals "empty pipe: stdout = 0"  "0"  "$stdout"

# /dev/null → 0 tokens, exit 0
stdout=$("$VRK" tok < /dev/null 2>/dev/null)
exit_code=$(set +e; "$VRK" tok < /dev/null > /dev/null 2>&1; echo $?)
assert_exit        "/dev/null: exit 0"        0    "$exit_code"
assert_stdout_equals "/dev/null: stdout = 0"  "0"  "$stdout"

# ------------------------------------------------------------
# 4. --json output
# ------------------------------------------------------------
echo ""
echo "--- --json ---"

stdout=$(echo 'hello world' | "$VRK" tok --json 2>/dev/null)
exit_code=$(set +e; echo 'hello world' | "$VRK" tok --json > /dev/null 2>&1; echo $?)
assert_exit "json: exit 0" 0 "$exit_code"
assert_stdout_json_field "json: tokens = 2"           "tokens" "2"            "$stdout"
assert_stdout_json_field "json: model = cl100k_base"  "model"  "cl100k_base"  "$stdout"

# --json with --model flag
stdout=$(echo 'hello world' | "$VRK" tok --json --model cl100k_base 2>/dev/null)
exit_code=$(set +e; echo 'hello world' | "$VRK" tok --json --model cl100k_base > /dev/null 2>&1; echo $?)
assert_exit "json+model: exit 0" 0 "$exit_code"

# ------------------------------------------------------------
# 5. --budget: within budget
# ------------------------------------------------------------
echo ""
echo "--- --budget (within) ---"

stdout=$(echo 'hello world' | "$VRK" tok --budget 5 2>/dev/null)
exit_code=$(set +e; echo 'hello world' | "$VRK" tok --budget 5 > /dev/null 2>&1; echo $?)
assert_exit        "budget 5: exit 0"        0    "$exit_code"
assert_stdout_equals "budget 5: stdout = 2"  "2"  "$stdout"

# ------------------------------------------------------------
# 6. --budget: over budget (no --fail)
# ------------------------------------------------------------
echo ""
echo "--- --budget (exceeded) ---"

stdout=$(set +e; echo 'hello world' | "$VRK" tok --budget 1 2>/dev/null; true)
stderr=$(set +e; echo 'hello world' | "$VRK" tok --budget 1 2>&1 >/dev/null; true)
exit_code=$(set +e; echo 'hello world' | "$VRK" tok --budget 1 > /dev/null 2>&1; echo $?)
assert_exit           "budget 1: exit 1"            1                          "$exit_code"
assert_stdout_empty   "budget 1: stdout empty"       "$stdout"
assert_stderr_contains "budget 1: stderr message"   "2 tokens exceeds budget of 1"  "$stderr"

# ------------------------------------------------------------
# 7. --budget + --fail: same result (--fail is redundant on tok)
# ------------------------------------------------------------
echo ""
echo "--- --budget --fail ---"

stdout=$(set +e; echo 'hello world' | "$VRK" tok --budget 1 --fail 2>/dev/null; true)
stderr=$(set +e; echo 'hello world' | "$VRK" tok --budget 1 --fail 2>&1 >/dev/null; true)
exit_code=$(set +e; echo 'hello world' | "$VRK" tok --budget 1 --fail > /dev/null 2>&1; echo $?)
assert_exit           "budget+fail: exit 1"           1                          "$exit_code"
assert_stdout_empty   "budget+fail: stdout empty"     "$stdout"
assert_stderr_contains "budget+fail: stderr message" "2 tokens exceeds budget of 1"  "$stderr"

# ------------------------------------------------------------
# 8. Pipeline safety: nothing passes through on exit 1
# ------------------------------------------------------------
echo ""
echo "--- pipeline safety ---"

# echo 'hello world' | vrk tok --budget 1 --fail | wc -c → 0
# vrk tok exits 1 here, so wrap it in `|| true` to stop pipefail from aborting
# the outer pipeline — we want to count the (empty) bytes that reach wc -c.
byte_count=$(echo 'hello world' | { "$VRK" tok --budget 1 --fail 2>/dev/null || true; } | wc -c | tr -d ' ')
assert_stdout_equals "pipeline: wc -c = 0" "0" "$byte_count"

# ------------------------------------------------------------
# 9. 100-token fixture file
# ------------------------------------------------------------
echo ""
echo "--- 100-token fixture ---"

if [ -f "testdata/tok/hundred-tokens.txt" ]; then
  stdout=$(cat testdata/tok/hundred-tokens.txt | "$VRK" tok 2>/dev/null)
  exit_code=$(set +e; cat testdata/tok/hundred-tokens.txt | "$VRK" tok > /dev/null 2>&1; echo $?)
  assert_exit        "hundred-tokens.txt: exit 0"          0      "$exit_code"
  assert_stdout_equals "hundred-tokens.txt: stdout = 100"  "100"  "$stdout"
else
  echo "  SKIP  hundred-tokens.txt not found (run from repo root)"
fi

# ------------------------------------------------------------
# 10. Usage errors
# ------------------------------------------------------------
echo ""
echo "--- usage errors ---"

exit_code=$(set +e; "$VRK" tok --unknown-flag > /dev/null 2>&1; echo $?)
assert_exit "unknown flag: exit 2" 2 "$exit_code"

# ------------------------------------------------------------
# 11. --help
# ------------------------------------------------------------
echo ""
echo "--- --help ---"

stdout=$("$VRK" tok --help 2>/dev/null) || true
exit_code=$(set +e; "$VRK" tok --help > /dev/null 2>&1; echo $?)
assert_exit "help: exit 0" 0 "$exit_code"
if echo "$stdout" | grep -q "tok"; then
  ok "help: contains 'tok'"
else
  fail "help: contains 'tok'" "got: $stdout"
fi

# ------------------------------------------------------------
# Summary
# ------------------------------------------------------------
echo ""
echo "---"
echo "Results: $PASS passed, $FAIL failed"
echo ""

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
