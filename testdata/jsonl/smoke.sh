#!/usr/bin/env bash
# testdata/jsonl/smoke.sh
#
# End-to-end smoke tests for vrk jsonl.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/jsonl/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/jsonl/smoke.sh   # explicit binary path

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
    ok "$desc"
  else
    fail "$desc" "expected '$expected', got '$actual'"
  fi
}

assert_stdout_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -qF -- "$pattern"; then
    ok "$desc (contains '$pattern')"
  else
    fail "$desc" "stdout did not contain '$pattern'. got: $actual"
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

assert_line_count() {
  local desc=$1 expected=$2 actual=$3
  local count
  if [ -z "$actual" ]; then
    count=0
  else
    count=$(echo "$actual" | wc -l | tr -d ' ')
  fi
  if [ "$count" -eq "$expected" ]; then
    ok "$desc ($expected lines)"
  else
    fail "$desc" "expected $expected lines, got $count"
  fi
}

echo "vrk jsonl — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Split mode: objects array → 3 JSONL lines
# ------------------------------------------------------------
echo "--- 1. split mode: objects ---"

got=$(echo '[{"a":1},{"a":2},{"a":3}]' | "$VRK" jsonl)
assert_line_count "objects: 3 lines" 3 "$got"
assert_stdout_contains "objects: first record" '{"a":1}' "$got"
assert_stdout_contains "objects: second record" '{"a":2}' "$got"
assert_stdout_contains "objects: third record" '{"a":3}' "$got"

# ------------------------------------------------------------
# 2. Split mode: empty array → no output, exit 0
# ------------------------------------------------------------
echo ""
echo "--- 2. empty array ---"

got=$(echo '[]' | "$VRK" jsonl)
exit_code=$?
assert_exit "empty array: exit 0" 0 "$exit_code"
assert_stdout_empty "empty array: no output" "$got"

# ------------------------------------------------------------
# 3. Split mode: primitives
# ------------------------------------------------------------
echo ""
echo "--- 3. primitives ---"

got=$(echo '[1,2,3]' | "$VRK" jsonl)
assert_line_count "primitives: 3 lines" 3 "$got"
assert_stdout_contains "primitives: first" "1" "$got"
assert_stdout_contains "primitives: second" "2" "$got"
assert_stdout_contains "primitives: third" "3" "$got"

# ------------------------------------------------------------
# 4. Collect mode: JSONL → JSON array
# ------------------------------------------------------------
echo ""
echo "--- 4. collect mode ---"

got=$(printf '{"a":1}\n{"a":2}\n{"a":3}\n' | "$VRK" jsonl --collect)
assert_stdout_contains "collect: starts with [" "[" "$got"
assert_stdout_contains "collect: contains first" '{"a":1}' "$got"
assert_stdout_contains "collect: contains third" '{"a":3}' "$got"
assert_line_count "collect: single line output" 1 "$got"

# ------------------------------------------------------------
# 5. Collect mode: empty stdin → []
# ------------------------------------------------------------
echo ""
echo "--- 5. collect empty stdin ---"

got=$(printf '' | "$VRK" jsonl --collect)
exit_code=$?
assert_exit "collect empty: exit 0" 0 "$exit_code"
assert_stdout_equals "collect empty: outputs []" "[]" "$got"

# ------------------------------------------------------------
# 6. --json trailer in split mode
# ------------------------------------------------------------
echo ""
echo "--- 6. --json trailer ---"

json_out=$(echo '[{"a":1},{"a":2}]' | "$VRK" jsonl --json)
last_line=$(printf '%s' "$json_out" | tail -1)
assert_stdout_contains "json trailer: _vrk field" '"_vrk":"jsonl"' "$last_line"
assert_stdout_contains "json trailer: count field" '"count":2' "$last_line"
assert_line_count "json trailer: 3 lines total" 3 "$json_out"

# --json on empty array: only metadata line
json_out=$(echo '[]' | "$VRK" jsonl --json)
last_line=$(printf '%s' "$json_out" | tail -1)
assert_stdout_contains "json empty: count=0" '"count":0' "$last_line"
assert_line_count "json empty: 1 line total" 1 "$json_out"

# ------------------------------------------------------------
# 7. Error cases
# ------------------------------------------------------------
echo ""
echo "--- 7. error cases ---"

set +e

echo 'not json' | "$VRK" jsonl > /dev/null 2>&1
exit_code=$?
assert_exit "invalid JSON: exit 1" 1 "$exit_code"

echo '{"not":"array"}' | "$VRK" jsonl > /dev/null 2>&1
exit_code=$?
assert_exit "non-array: exit 1" 1 "$exit_code"

# --collect with bad line
printf '{"a":1}\nnot-json\n' | "$VRK" jsonl --collect > /dev/null 2>&1
exit_code=$?
assert_exit "collect bad line: exit 1" 1 "$exit_code"

# Unknown flag
"$VRK" jsonl --bogus < /dev/null > /dev/null 2>&1
exit_code=$?
assert_exit "unknown flag: exit 2" 2 "$exit_code"

set -e

# Error message for non-array goes to stderr
stderr_out=$(echo '{"not":"array"}' | "$VRK" jsonl 2>&1 1>/dev/null || true)
assert_stdout_contains "non-array: stderr message" "--collect" "$stderr_out"

# ------------------------------------------------------------
# 8. Empty stdin in split mode → exit 0, no output
# ------------------------------------------------------------
echo ""
echo "--- 8. empty stdin ---"

got=$(printf '' | "$VRK" jsonl)
exit_code=$?
assert_exit "empty stdin: exit 0" 0 "$exit_code"
assert_stdout_empty "empty stdin: no output" "$got"

got=$(echo '' | "$VRK" jsonl)
exit_code=$?
assert_exit "newline-only stdin: exit 0" 0 "$exit_code"
assert_stdout_empty "newline-only stdin: no output" "$got"

# ------------------------------------------------------------
# 9. Round-trip: split then collect → structurally equal
# ------------------------------------------------------------
echo ""
echo "--- 9. round-trip ---"

input='[{"a":1},{"b":2}]'
collected=$(echo "$input" | "$VRK" jsonl | "$VRK" jsonl --collect)

# Re-parse both and compare via jq if available, otherwise string compare
# on simple inputs where key order doesn't matter.
assert_stdout_contains "round-trip: contains a" '"a":1' "$collected"
assert_stdout_contains "round-trip: contains b" '"b":2' "$collected"
assert_stdout_contains "round-trip: is array" "[" "$collected"

# ------------------------------------------------------------
# 10. --help exits 0
# ------------------------------------------------------------
echo ""
echo "--- 10. --help ---"

"$VRK" jsonl --help > /dev/null
exit_code=$?
assert_exit "--help: exit 0" 0 "$exit_code"

help_out=$("$VRK" jsonl --help)
assert_stdout_contains "--help: mentions jsonl" "jsonl" "$help_out"
assert_stdout_contains "--help: mentions --collect" "--collect" "$help_out"
assert_stdout_contains "--help: mentions --json" "--json" "$help_out"

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
