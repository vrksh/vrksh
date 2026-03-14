#!/usr/bin/env bash
# testdata/validate/smoke.sh
#
# End-to-end smoke tests for vrk validate.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/validate/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/validate/smoke.sh   # explicit binary path
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
    ok "$desc"
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

assert_stderr_empty() {
  local desc=$1 stderr=$2
  if [ -z "$stderr" ]; then
    ok "$desc (stderr empty)"
  else
    fail "$desc" "expected empty stderr, got: $stderr"
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

assert_stdout_not_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -q "$pattern"; then
    fail "$desc" "stdout must NOT contain '$pattern' but did: $actual"
  else
    ok "$desc (does not contain '$pattern')"
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
    ok "$desc ($expected line(s))"
  else
    fail "$desc" "expected $expected line(s), got $count. stdout: $actual"
  fi
}

assert_valid_json() {
  local desc=$1 actual=$2
  if command -v python3 > /dev/null 2>&1; then
    if echo "$actual" | python3 -c 'import sys,json; json.load(sys.stdin)' > /dev/null 2>&1; then
      ok "$desc (valid JSON)"
    else
      fail "$desc" "not valid JSON: $actual"
    fi
  else
    echo "  SKIP  $desc (python3 not available)"
    PASS=$((PASS + 1))
  fi
}

echo "vrk validate — smoke tests"
echo "binary: $VRK"
echo ""

SCHEMA='{"name":"string","age":"number"}'

# ------------------------------------------------------------
# 1. Valid record passes through to stdout, exit 0
# ------------------------------------------------------------
echo "--- 1. valid record ---"

stdout=$(echo '{"name":"alice","age":30}' | "$VRK" validate --schema "$SCHEMA" 2>/dev/null)
exit_code=$(set +e; echo '{"name":"alice","age":30}' | "$VRK" validate --schema "$SCHEMA" >/dev/null 2>&1; echo $?)
stderr=$(set +e; echo '{"name":"alice","age":30}' | "$VRK" validate --schema "$SCHEMA" 2>&1 >/dev/null; true)

assert_exit          "valid: exit 0"                   0                          "$exit_code"
assert_stderr_empty  "valid: stderr empty"                                         "$stderr"
assert_stdout_equals "valid: record on stdout"          '{"name":"alice","age":30}' "$stdout"

# ------------------------------------------------------------
# 2. Invalid record: empty stdout, warning on stderr, exit 0
# ------------------------------------------------------------
echo ""
echo "--- 2. invalid record ---"

stdout=$(set +e; echo '{"name":"alice","age":"wrong"}' | "$VRK" validate --schema "$SCHEMA" 2>/dev/null; true)
exit_code=$(set +e; echo '{"name":"alice","age":"wrong"}' | "$VRK" validate --schema "$SCHEMA" >/dev/null 2>&1; echo $?)
stderr=$(set +e; echo '{"name":"alice","age":"wrong"}' | "$VRK" validate --schema "$SCHEMA" 2>&1 >/dev/null; true)

assert_exit         "invalid: exit 0"          0  "$exit_code"
assert_stdout_empty "invalid: stdout empty"        "$stdout"
assert_stdout_contains "invalid: stderr warns about age" "age" "$stderr"

# ------------------------------------------------------------
# 3. --strict with invalid record: exit 1
# ------------------------------------------------------------
echo ""
echo "--- 3. --strict ---"

exit_code=$(set +e; echo '{"name":"alice","age":"wrong"}' | "$VRK" validate --schema "$SCHEMA" --strict >/dev/null 2>&1; echo $?)
assert_exit "strict: exit 1 on invalid" 1 "$exit_code"

# --strict with all valid records: exit 0
exit_code=$(set +e; echo '{"name":"alice","age":30}' | "$VRK" validate --schema "$SCHEMA" --strict >/dev/null 2>&1; echo $?)
assert_exit "strict: exit 0 on all valid" 0 "$exit_code"

# --strict + --json: must emit metadata record even when exiting 1
stdout=$(set +e; echo '{"name":"alice","age":"wrong"}' | "$VRK" validate --schema "$SCHEMA" --strict --json 2>/dev/null; true)
exit_code=$(set +e; echo '{"name":"alice","age":"wrong"}' | "$VRK" validate --schema "$SCHEMA" --strict --json >/dev/null 2>&1; echo $?)
assert_exit "strict+json: exit 1" 1 "$exit_code"
assert_stdout_contains "strict+json: metadata on stdout"       '"_vrk":"validate"' "$stdout"
assert_stdout_contains "strict+json: metadata has failed=1"    '"failed":1'         "$stdout"

# ------------------------------------------------------------
# 4. Empty stdin: exit 0, no output
# ------------------------------------------------------------
echo ""
echo "--- 4. empty stdin ---"

stdout=$(printf '' | "$VRK" validate --schema "$SCHEMA" 2>/dev/null)
exit_code=$(set +e; printf '' | "$VRK" validate --schema "$SCHEMA" >/dev/null 2>&1; echo $?)

assert_exit         "empty stdin: exit 0"        0  "$exit_code"
assert_stdout_empty "empty stdin: stdout empty"      "$stdout"

# ------------------------------------------------------------
# 5. Empty stdin + --json: metadata record with all zeros
# ------------------------------------------------------------
echo ""
echo "--- 5. empty stdin + --json ---"

stdout=$(printf '' | "$VRK" validate --schema "$SCHEMA" --json 2>/dev/null)
exit_code=$(set +e; printf '' | "$VRK" validate --schema "$SCHEMA" --json >/dev/null 2>&1; echo $?)

assert_exit "empty + --json: exit 0" 0 "$exit_code"
assert_line_count "empty + --json: one line (metadata)" 1 "$stdout"
assert_stdout_contains "empty + --json: _vrk field" '"_vrk":"validate"' "$stdout"
assert_stdout_contains "empty + --json: total=0"    '"total":0'          "$stdout"
assert_stdout_contains "empty + --json: passed=0"   '"passed":0'         "$stdout"
assert_stdout_contains "empty + --json: failed=0"   '"failed":0'         "$stdout"
assert_valid_json      "empty + --json: valid JSON"                       "$stdout"

# ------------------------------------------------------------
# 6. No --schema: exit 2
# ------------------------------------------------------------
echo ""
echo "--- 6. no --schema ---"

exit_code=$(set +e; printf '' | "$VRK" validate >/dev/null 2>&1; echo $?)
stdout=$(set +e; printf '' | "$VRK" validate 2>/dev/null; true)

assert_exit         "no schema: exit 2"        2  "$exit_code"
assert_stdout_empty "no schema: stdout empty"      "$stdout"

# ------------------------------------------------------------
# 7. Bad schema JSON: exit 2
# ------------------------------------------------------------
echo ""
echo "--- 7. bad schema JSON ---"

exit_code=$(set +e; printf '' | "$VRK" validate --schema '{"bad json' >/dev/null 2>&1; echo $?)
stdout=$(set +e; printf '' | "$VRK" validate --schema '{"bad json' 2>/dev/null; true)

assert_exit         "bad schema: exit 2"        2  "$exit_code"
assert_stdout_empty "bad schema: stdout empty"      "$stdout"

# ------------------------------------------------------------
# 8. File-based schema
# ------------------------------------------------------------
echo ""
echo "--- 8. file-based schema ---"

TMPSCHEMA=$(mktemp /tmp/vrk-validate-smoke-XXXXXX.json)
echo '{"name":"string","age":"number"}' > "$TMPSCHEMA"

stdout=$(echo '{"name":"alice","age":30}' | "$VRK" validate --schema "$TMPSCHEMA" 2>/dev/null)
exit_code=$(set +e; echo '{"name":"alice","age":30}' | "$VRK" validate --schema "$TMPSCHEMA" >/dev/null 2>&1; echo $?)

rm -f "$TMPSCHEMA"

assert_exit          "file schema: exit 0"              0                          "$exit_code"
assert_stdout_equals "file schema: record on stdout"    '{"name":"alice","age":30}' "$stdout"

# ------------------------------------------------------------
# 9. --json with mixed stream: metadata is last, totals add up
# ------------------------------------------------------------
echo ""
echo "--- 9. --json mixed stream ---"

INPUT=$(printf '{"name":"alice","age":30}\n{"name":"bob","age":"wrong"}\n{"name":"carol","age":25}')
stdout=$(echo "$INPUT" | "$VRK" validate --schema "$SCHEMA" --json 2>/dev/null)
exit_code=$(set +e; echo "$INPUT" | "$VRK" validate --schema "$SCHEMA" --json >/dev/null 2>&1; echo $?)

assert_exit "json mixed: exit 0" 0 "$exit_code"

# Last line is metadata.
last_line=$(echo "$stdout" | tail -1)
assert_stdout_contains "json mixed: _vrk in last line"  '"_vrk":"validate"' "$last_line"
assert_stdout_contains "json mixed: total=3"             '"total":3'         "$last_line"
assert_stdout_contains "json mixed: passed=2"            '"passed":2'        "$last_line"
assert_stdout_contains "json mixed: failed=1"            '"failed":1'        "$last_line"

# ------------------------------------------------------------
# 10. Mixed stream: valid lines on stdout, invalid absent
# ------------------------------------------------------------
echo ""
echo "--- 10. mixed stream stdout/stderr separation ---"

INPUT=$(printf '{"name":"alice","age":30}\n{"name":"bob","age":"wrong"}\n{"name":"carol","age":25}')
stdout=$(echo "$INPUT" | "$VRK" validate --schema "$SCHEMA" 2>/dev/null)
stderr=$(set +e; echo "$INPUT" | "$VRK" validate --schema "$SCHEMA" 2>&1 >/dev/null; true)

assert_stdout_contains     "mixed: alice on stdout"        "alice"         "$stdout"
assert_stdout_contains     "mixed: carol on stdout"        "carol"         "$stdout"
assert_stdout_not_contains "mixed: bob not on stdout"      '"bob"'         "$stdout"
assert_line_count          "mixed: two valid lines"        2               "$stdout"
assert_stdout_contains     "mixed: warning on stderr"      "validation"    "$stderr"

# ------------------------------------------------------------
# 11. Pipeline composition: validate | tok
# ------------------------------------------------------------
echo ""
echo "--- 11. pipeline composition ---"

tok_out=$(echo '{"name":"alice","age":30}' | "$VRK" validate --schema "$SCHEMA" | "$VRK" tok 2>/dev/null)
exit_code=$(set +e; echo '{"name":"alice","age":30}' | "$VRK" validate --schema "$SCHEMA" | "$VRK" tok >/dev/null 2>&1; echo $?)

assert_exit "pipeline: exit 0" 0 "$exit_code"
# tok should output a positive integer.
if echo "$tok_out" | grep -qE '^[1-9][0-9]*$'; then
  ok "pipeline: tok output is a positive number ($tok_out)"
else
  fail "pipeline: tok output is not a positive number" "got: $tok_out"
fi

# ------------------------------------------------------------
# 12. --fix graceful degradation (no API key configured)
# ------------------------------------------------------------
echo ""
echo "--- 12. --fix degradation (no API key) ---"

# When no API key is set, --fix should degrade silently:
# the invalid line stays invalid (nothing on stdout), a warning goes to stderr,
# and the process exits 0 (not 2 — missing key is not a usage error).
# This is always safe to run in CI since no key is expected.
stdout=$(set +e; echo '{"name":"alice","age":"wrong"}' | "$VRK" validate --schema "$SCHEMA" --fix 2>/dev/null; true)
exit_code=$(set +e; echo '{"name":"alice","age":"wrong"}' | "$VRK" validate --schema "$SCHEMA" --fix >/dev/null 2>&1; echo $?)
stderr=$(set +e; echo '{"name":"alice","age":"wrong"}' | "$VRK" validate --schema "$SCHEMA" --fix 2>&1 >/dev/null; true)

assert_exit         "fix degrade: exit 0"        0  "$exit_code"
assert_stdout_empty "fix degrade: stdout empty"      "$stdout"
# stderr must have some output (either the fix warning or validation warning)
if [ -n "$stderr" ]; then
  ok "fix degrade: stderr has warning"
else
  fail "fix degrade: stderr must have a warning" "got empty stderr"
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
