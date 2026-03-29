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

assert_stderr_empty() {
  local desc=$1 stderr=$2
  if [ -z "$stderr" ]; then
    ok "$desc (stderr empty)"
  else
    fail "$desc" "expected empty stderr, got: $stderr"
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
# 5. --check: within limit
# ------------------------------------------------------------
echo ""
echo "--- --check (within limit) ---"

out=$(echo 'hello world' | "$VRK" tok --check 8000 2>/dev/null)
exit_code=$(set +e; echo 'hello world' | "$VRK" tok --check 8000 > /dev/null 2>&1; echo $?)
assert_exit          "--check within limit: exit 0"               0              "$exit_code"
assert_stdout_equals "--check within limit: input passed through" "hello world"  "$out"

# stderr must be empty on success (silent success)
stderr=$(echo 'hello world' | "$VRK" tok --check 8000 2>&1 >/dev/null)
assert_stderr_empty  "--check within limit: stderr empty"  "$stderr"

# ------------------------------------------------------------
# 6. --check: over limit
# ------------------------------------------------------------
echo ""
echo "--- --check (over limit) ---"

exit_code=$(set +e; echo 'hello world' | "$VRK" tok --check 1 > /dev/null 2>&1; echo $?)
assert_exit "--check over limit: exit 1" 1 "$exit_code"

out=$(echo 'hello world' | "$VRK" tok --check 1 2>/dev/null || true)
assert_stdout_empty "--check over limit: stdout empty" "$out"

# ------------------------------------------------------------
# 7. --check without value: exit 2
# ------------------------------------------------------------
echo ""
echo "--- --check without value ---"

exit_code=$(set +e +o pipefail; echo 'hello world' | "$VRK" tok --check 2>/dev/null; echo $?)
assert_exit "--check without value: exit 2" 2 "$exit_code"

# ------------------------------------------------------------
# 8. --check + --json over limit
# ------------------------------------------------------------
echo ""
echo "--- --check + --json (over limit) ---"

out=$(echo 'hello world' | "$VRK" tok --check 1 --json 2>/dev/null || true)
exit_code=$(set +e; echo 'hello world' | "$VRK" tok --check 1 --json > /dev/null 2>&1; echo $?)
assert_exit "--check --json over limit: exit 1" 1 "$exit_code"
if echo "$out" | grep -q '"error"' && echo "$out" | grep -q '"limit"'; then
  ok "--check --json over limit: JSON has error and limit fields"
else
  fail "--check --json over limit: missing JSON fields" "got: $out"
fi

# stderr must be empty when --json is active
stderr=$(echo 'hello world' | "$VRK" tok --check 1 --json 2>&1 >/dev/null || true)
assert_stderr_empty "--check --json: stderr empty" "$stderr"

# ------------------------------------------------------------
# 8b. --check byte-for-byte transparency
# ------------------------------------------------------------
echo ""
echo "--- --check byte-for-byte ---"

input='{"a":1,  "b":2}'
out=$(printf '%s' "$input" | "$VRK" tok --check 8000 2>/dev/null)
assert_stdout_equals "--check byte-for-byte: exact match" "$input" "$out"

# ------------------------------------------------------------
# 8c. Pipeline: tok --check gates correctly
# ------------------------------------------------------------
echo ""
echo "--- pipeline ---"

out=$(echo 'hello world' | "$VRK" tok --check 8000 2>/dev/null | cat)
assert_stdout_equals "pipeline: tok --check + cat" "hello world" "$out"

# Pipeline gate: over limit stops downstream
byte_count=$(echo 'hello world' | { "$VRK" tok --check 1 2>/dev/null || true; } | wc -c | tr -d ' ')
assert_stdout_equals "pipeline: over limit stops downstream" "0" "$byte_count"

# ------------------------------------------------------------
# 8d. --budget and --fail rejected as unknown flags
# ------------------------------------------------------------
echo ""
echo "--- removed flags ---"

exit_code=$(set +e; echo 'hello world' | "$VRK" tok --budget 5 > /dev/null 2>&1; echo $?)
assert_exit "--budget: exit 2 (unknown flag)" 2 "$exit_code"

exit_code=$(set +e; echo 'hello world' | "$VRK" tok --fail > /dev/null 2>&1; echo $?)
assert_exit "--fail: exit 2 (unknown flag)" 2 "$exit_code"

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
# 12. --quiet
# ------------------------------------------------------------
echo ""
echo "--- --quiet ---"

# --check + --quiet within limit: passes through silently
out=$(echo 'hello world' | "$VRK" tok --check 8000 --quiet 2>/dev/null)
exit_code=$(set +e; echo 'hello world' | "$VRK" tok --check 8000 --quiet > /dev/null 2>&1; echo $?)
stderr=$(echo 'hello world' | "$VRK" tok --check 8000 --quiet 2>&1 >/dev/null)
assert_exit            "--quiet --check within: exit 0"          0              "$exit_code"
assert_stdout_equals   "--quiet --check within: passthrough"     "hello world"  "$out"
assert_stderr_empty    "--quiet --check within: stderr empty"                   "$stderr"

# --check + --quiet over limit: exit 1, stdout empty, stderr empty
exit_code=$(set +e; echo 'hello world' | "$VRK" tok --check 1 --quiet > /dev/null 2>&1; echo $?)
out=$(echo 'hello world' | "$VRK" tok --check 1 --quiet 2>/dev/null || true)
stderr=$(echo 'hello world' | "$VRK" tok --check 1 --quiet 2>&1 >/dev/null || true)
assert_exit            "--quiet --check over: exit 1"            1              "$exit_code"
assert_stdout_empty    "--quiet --check over: stdout empty"                     "$out"
assert_stderr_empty    "--quiet --check over: stderr empty"                     "$stderr"

# Valid count with quiet should still produce stdout
stdout=$(echo 'hello world' | "$VRK" tok --quiet 2>/dev/null) || true
exit_code=$(set +e; echo 'hello world' | "$VRK" tok --quiet > /dev/null 2>&1; echo $?)
assert_exit            "--quiet success: exit 0"        0        "$exit_code"
assert_stdout_equals   "--quiet success: value" "2"              "$stdout"

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
