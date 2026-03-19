#!/usr/bin/env bash
# testdata/assert/smoke.sh
#
# End-to-end smoke tests for vrk assert.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/assert/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/assert/smoke.sh   # explicit binary path

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
  if echo "$actual" | grep -q "$pattern"; then
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

assert_stderr_contains() {
  local desc=$1 pattern=$2 stderr=$3
  if echo "$stderr" | grep -q "$pattern"; then
    ok "$desc (stderr contains '$pattern')"
  else
    fail "$desc" "stderr did not contain '$pattern'. got: $stderr"
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

assert_line_count() {
  local desc=$1 expected=$2 actual=$3
  local count
  if [ -z "$actual" ]; then
    count=0
  else
    count=$(echo "$actual" | wc -l | tr -d ' ')
  fi
  if [ "$count" -eq "$expected" ]; then
    ok "$desc ($count lines)"
  else
    fail "$desc" "expected $expected lines, got $count"
  fi
}

echo "vrk assert — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. JSON condition pass
# ------------------------------------------------------------
echo "--- 1. JSON condition pass ---"

got=$(echo '{"status":"ok"}' | "$VRK" assert '.status == "ok"')
rc=$?
assert_exit "json condition pass" 0 "$rc"
assert_stdout_equals "json condition passthrough" '{"status":"ok"}' "$got"

# ------------------------------------------------------------
# 2. JSON condition fail
# ------------------------------------------------------------
echo ""
echo "--- 2. JSON condition fail ---"

got=$(echo '{"status":"fail"}' | "$VRK" assert '.status == "ok"' 2>/dev/null || true)
rc=0
echo '{"status":"fail"}' | "$VRK" assert '.status == "ok"' 2>/dev/null || rc=$?
assert_exit "json condition fail" 1 "$rc"
assert_stdout_empty "json condition fail stdout" "$got"

# ------------------------------------------------------------
# 3. Multiple conditions
# ------------------------------------------------------------
echo ""
echo "--- 3. Multiple conditions ---"

got=$(echo '{"count":5,"errors":[]}' | "$VRK" assert '.count > 0' '.errors == []')
rc=$?
assert_exit "multiple conditions pass" 0 "$rc"
assert_stdout_equals "multiple conditions passthrough" '{"count":5,"errors":[]}' "$got"

rc=0
echo '{"count":0,"errors":[]}' | "$VRK" assert '.count > 0' '.errors == []' 2>/dev/null || rc=$?
assert_exit "multiple conditions fail" 1 "$rc"

# ------------------------------------------------------------
# 4. Numeric comparisons
# ------------------------------------------------------------
echo ""
echo "--- 4. Numeric comparisons ---"

echo '{"score":0.91}' | "$VRK" assert '.score > 0.8' >/dev/null
assert_exit "numeric > pass" 0 $?

rc=0
echo '{"score":0.71}' | "$VRK" assert '.score > 0.8' 2>/dev/null || rc=$?
assert_exit "numeric > fail" 1 "$rc"

# ------------------------------------------------------------
# 5. Null handling
# ------------------------------------------------------------
echo ""
echo "--- 5. Null handling ---"

echo '{}' | "$VRK" assert '.missing == null' >/dev/null
assert_exit "missing == null" 0 $?

rc=0
echo '{}' | "$VRK" assert '.missing != null' 2>/dev/null || rc=$?
assert_exit "missing != null" 1 "$rc"

# ------------------------------------------------------------
# 6. Array length
# ------------------------------------------------------------
echo ""
echo "--- 6. Array length ---"

echo '{"items":[1,2,3]}' | "$VRK" assert '.items | length > 0' >/dev/null
assert_exit "array length > 0 pass" 0 $?

rc=0
echo '{"items":[]}' | "$VRK" assert '.items | length > 0' 2>/dev/null || rc=$?
assert_exit "array length > 0 fail" 1 "$rc"

# ------------------------------------------------------------
# 7. --contains
# ------------------------------------------------------------
echo ""
echo "--- 7. --contains ---"

got=$(echo 'All tests passed' | "$VRK" assert --contains 'passed')
rc=$?
assert_exit "contains pass" 0 "$rc"
assert_stdout_equals "contains passthrough" "All tests passed" "$got"

rc=0
echo 'Some tests failed' | "$VRK" assert --contains 'passed' 2>/dev/null || rc=$?
assert_exit "contains fail" 1 "$rc"

# ------------------------------------------------------------
# 8. --matches
# ------------------------------------------------------------
echo ""
echo "--- 8. --matches ---"

got=$(echo 'OK: all systems nominal' | "$VRK" assert --matches '^OK:')
rc=$?
assert_exit "matches pass" 0 "$rc"
assert_stdout_equals "matches passthrough" "OK: all systems nominal" "$got"

rc=0
echo 'ERROR: disk full' | "$VRK" assert --matches '^OK:' 2>/dev/null || rc=$?
assert_exit "matches fail" 1 "$rc"

# ------------------------------------------------------------
# 9. --contains + --matches combined
# ------------------------------------------------------------
echo ""
echo "--- 9. --contains + --matches combined ---"

got=$(echo 'OK: all tests passed' | "$VRK" assert --contains 'passed' --matches '^OK:')
rc=$?
assert_exit "contains+matches pass" 0 "$rc"
assert_stdout_equals "contains+matches passthrough" "OK: all tests passed" "$got"

rc=0
echo 'OK: some tests failed' | "$VRK" assert --contains 'passed' --matches '^OK:' 2>/dev/null || rc=$?
assert_exit "contains+matches fail" 1 "$rc"

# ------------------------------------------------------------
# 10. --message
# ------------------------------------------------------------
echo ""
echo "--- 10. --message ---"

stderr_got=$(echo '{"x":1}' | "$VRK" assert '.x == 2' --message 'Wrong value' 2>&1 >/dev/null || true)
assert_stderr_contains "message flag" "Wrong value" "$stderr_got"
assert_stderr_contains "message includes condition" '.x == 2' "$stderr_got"

# ------------------------------------------------------------
# 11. --quiet
# ------------------------------------------------------------
echo ""
echo "--- 11. --quiet ---"

stderr_got=$(echo '{"x":1}' | "$VRK" assert '.x == 2' --quiet 2>&1 >/dev/null || true)
assert_stderr_empty "quiet suppresses stderr" "$stderr_got"

# ------------------------------------------------------------
# 12. --json (JSON mode)
# ------------------------------------------------------------
echo ""
echo "--- 12. --json (JSON mode) ---"

got=$(echo '{"status":"ok"}' | "$VRK" assert '.status == "ok"' --json)
assert_stdout_contains "json pass: passed=true" '"passed":true' "$got"
assert_stdout_contains "json pass: condition" '"condition"' "$got"
assert_stdout_contains "json pass: input" '"input"' "$got"

got=$(echo '{"status":"fail"}' | "$VRK" assert '.status == "ok"' --json 2>/dev/null || true)
assert_stdout_contains "json fail: passed=false" '"passed":false' "$got"

# ------------------------------------------------------------
# 13. --json (plain text mode)
# ------------------------------------------------------------
echo ""
echo "--- 13. --json (plain text mode) ---"

got=$(echo 'All tests passed' | "$VRK" assert --contains 'passed' --json)
assert_stdout_contains "json plain pass: passed=true" '"passed":true' "$got"
assert_stdout_contains "json plain pass: condition" '"--contains: passed"' "$got"

got=$(echo 'Some tests failed' | "$VRK" assert --contains 'passed' --json 2>/dev/null || true)
assert_stdout_contains "json plain fail: passed=false" '"passed":false' "$got"
assert_stdout_contains "json plain fail: message" '"message"' "$got"

# ------------------------------------------------------------
# 14. JSONL streaming
# ------------------------------------------------------------
echo ""
echo "--- 14. JSONL streaming ---"

got=$(printf '{"ok":true}\n{"ok":true}\n' | "$VRK" assert '.ok == true')
rc=$?
assert_exit "jsonl all pass" 0 "$rc"
assert_line_count "jsonl all pass lines" 2 "$got"

rc=0
got=$(printf '{"ok":true}\n{"ok":false}\n' | "$VRK" assert '.ok == true' 2>/dev/null || rc=$?)
# rc might not be set properly due to subshell; re-check
rc2=0
printf '{"ok":true}\n{"ok":false}\n' | "$VRK" assert '.ok == true' 2>/dev/null || rc2=$?
assert_exit "jsonl second line fail" 1 "$rc2"

# ------------------------------------------------------------
# 15. --json with JSONL
# ------------------------------------------------------------
echo ""
echo "--- 15. --json with JSONL ---"

got=$(printf '{"ok":true}\n{"ok":false}\n' | "$VRK" assert '.ok == true' --json 2>/dev/null || true)
assert_line_count "json+jsonl output lines" 2 "$got"
first_line=$(echo "$got" | head -1)
assert_stdout_contains "json+jsonl first line pass" '"passed":true' "$first_line"
second_line=$(echo "$got" | tail -1)
assert_stdout_contains "json+jsonl second line fail" '"passed":false' "$second_line"

# ------------------------------------------------------------
# 16. Byte-for-byte transparency
# ------------------------------------------------------------
echo ""
echo "--- 16. Byte-for-byte transparency ---"

input='{"a":1,  "b":2}'
got=$(echo "$input" | "$VRK" assert '.a == 1')
assert_stdout_equals "byte-for-byte passthrough" "$input" "$got"

# ------------------------------------------------------------
# 17. Error: no condition
# ------------------------------------------------------------
echo ""
echo "--- 17. Error: no condition ---"

rc=0
echo '{}' | "$VRK" assert 2>/dev/null || rc=$?
assert_exit "no condition" 2 "$rc"

# ------------------------------------------------------------
# 18. Error: invalid JSON input
# ------------------------------------------------------------
echo ""
echo "--- 18. Error: invalid JSON ---"

rc=0
echo 'not json' | "$VRK" assert '.field == "value"' 2>/dev/null || rc=$?
assert_exit "invalid json" 1 "$rc"

# ------------------------------------------------------------
# 19. Error: mode conflict
# ------------------------------------------------------------
echo ""
echo "--- 19. Error: mode conflict ---"

rc=0
echo 'text' | "$VRK" assert '.field == 1' --contains 'text' 2>/dev/null || rc=$?
assert_exit "mode conflict" 2 "$rc"

# ------------------------------------------------------------
# 20. Pipeline composition
# ------------------------------------------------------------
echo ""
echo "--- 20. Pipeline composition ---"

got=$(echo '{"confidence":0.9}' | "$VRK" assert '.confidence > 0.8' | cat)
rc=$?
assert_exit "pipeline composition" 0 "$rc"
assert_stdout_equals "pipeline data flows" '{"confidence":0.9}' "$got"

# ------------------------------------------------------------
# Summary
# ------------------------------------------------------------
echo ""
echo "================================"
echo "  $PASS passed, $FAIL failed"
echo "================================"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
