#!/usr/bin/env bash
# testdata/throttle/smoke.sh
#
# End-to-end smoke tests for vrk throttle.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/throttle/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/throttle/smoke.sh   # explicit binary path

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
    # printf '%s\n' restores the trailing newline stripped by $() so that
    # wc -l counts all lines including the last one correctly.
    count=$(printf '%s\n' "$actual" | wc -l | tr -d ' ')
  fi
  if [ "$count" -eq "$expected" ]; then
    ok "$desc ($expected line(s))"
  else
    fail "$desc" "expected $expected line(s), got $count. stdout: $actual"
  fi
}

echo "vrk throttle — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Basic pass-through
# ------------------------------------------------------------
echo "--- 1. basic pass-through ---"

got=$(printf 'a\nb\nc\n' | "$VRK" throttle --rate 100/s)
exit_code=$?
assert_exit       "3-line input: exit 0"   0  "$exit_code"
assert_line_count "3-line input: 3 lines"  3  "$got"

first=$(printf '%s\n' "$got" | head -1)
last=$(printf '%s\n' "$got" | tail -1)
assert_stdout_equals "first line unchanged"  "a"  "$first"
assert_stdout_equals "last line unchanged"   "c"  "$last"

got=$(echo 'hello' | "$VRK" throttle --rate 100/s)
assert_stdout_equals "content unchanged"  "hello"  "$got"

# ------------------------------------------------------------
# 2. Empty and blank input
# ------------------------------------------------------------
echo ""
echo "--- 2. empty and blank input ---"

got=$(printf '' | "$VRK" throttle --rate 100/s)
exit_code=$?
assert_exit        "empty stdin: exit 0"     0  "$exit_code"
assert_stdout_empty "empty stdin: no output"    "$got"

# echo '' sends a single newline — scanner produces one empty string → skipped.
got=$(echo '' | "$VRK" throttle --rate 100/s)
exit_code=$?
assert_exit        "empty line: exit 0"     0  "$exit_code"
assert_stdout_empty "empty line: no output"    "$got"

# A whitespace-only line is content and passes through unchanged.
got=$(printf '   \n' | "$VRK" throttle --rate 100/s)
exit_code=$?
assert_exit        "whitespace-only line: exit 0"        0      "$exit_code"
assert_stdout_equals "whitespace-only line: emitted as-is"  "   "  "$got"

# ------------------------------------------------------------
# 3. Usage errors
# ------------------------------------------------------------
echo ""
echo "--- 3. usage errors ---"

set +e
"$VRK" throttle < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "missing --rate: exit 2" 2 "$exit_code"

set +e
"$VRK" throttle --rate 0/s < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--rate 0/s: exit 2" 2 "$exit_code"

set +e
"$VRK" throttle --rate abc < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--rate abc: exit 2" 2 "$exit_code"

set +e
"$VRK" throttle --rate 0.5/s < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--rate 0.5/s (decimal N rejected): exit 2" 2 "$exit_code"

# ------------------------------------------------------------
# 4. --help
# ------------------------------------------------------------
echo ""
echo "--- 4. --help ---"

"$VRK" throttle --help > /dev/null
exit_code=$?
assert_exit "--help: exit 0" 0 "$exit_code"

# ------------------------------------------------------------
# 5. --json metadata trailer
# ------------------------------------------------------------
echo ""
echo "--- 5. --json metadata trailer ---"

json_out=$(printf 'x\ny\n' | "$VRK" throttle --rate 100/s --json)
exit_code=$?
assert_exit       "--json: exit 0"                   0  "$exit_code"
assert_line_count "--json: 2 data + 1 metadata"      3  "$json_out"

last_line=$(printf '%s\n' "$json_out" | tail -1)
assert_stdout_contains "--json: _vrk field"    '"_vrk":"throttle"'  "$last_line"
assert_stdout_contains "--json: rate field"    '"rate":"100/s"'      "$last_line"
assert_stdout_contains "--json: lines field"   '"lines":2'           "$last_line"
assert_stdout_contains "--json: elapsed_ms"    '"elapsed_ms"'        "$last_line"

json_out=$(printf '' | "$VRK" throttle --rate 10/s --json)
exit_code=$?
assert_exit       "--json empty stdin: exit 0"  0  "$exit_code"
assert_stdout_contains "--json empty stdin: lines=0"  '"lines":0'  "$json_out"

# ------------------------------------------------------------
# 6. --tokens-field
# ------------------------------------------------------------
echo ""
echo "--- 6. --tokens-field ---"

tf_input='{"prompt":"hi"}
{"prompt":"hello world"}'
got=$(printf '%s\n' "$tf_input" | "$VRK" throttle --rate 100/s --tokens-field prompt)
exit_code=$?
assert_exit       "--tokens-field: exit 0"          0  "$exit_code"
assert_line_count "--tokens-field: 2 lines emitted"  2  "$got"
assert_stdout_contains "--tokens-field: line 1 verbatim"  '"prompt":"hi"'  "$got"

set +e
echo 'not json' | "$VRK" throttle --rate 10/s --tokens-field prompt > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--tokens-field invalid JSON: exit 1" 1 "$exit_code"

# ------------------------------------------------------------
# 7. Pipeline composition
# ------------------------------------------------------------
echo ""
echo "--- 7. pipeline ---"

count=$(seq 10 | "$VRK" throttle --rate 100/s | wc -l | tr -d ' ')
assert_stdout_equals "seq 10 | throttle: 10 lines out" "10" "$count"

# ------------------------------------------------------------
# 8. Timing: seq 3 at 2/s takes >= 1s
# ------------------------------------------------------------
echo ""
echo "--- 8. timing ---"

# TTY guard is covered by TestInteractiveTTY / TestInteractiveTTYWithJSON in
# throttle_test.go — a real TTY cannot be simulated in automated smoke tests.

start_ts=$(date +%s)
seq 3 | "$VRK" throttle --rate 2/s > /dev/null
end_ts=$(date +%s)
elapsed=$((end_ts - start_ts))

if [ "$elapsed" -ge 1 ] && [ "$elapsed" -le 5 ]; then
  ok "timing: seq 3 at 2/s took ${elapsed}s (want 1-5s)"
else
  fail "timing: seq 3 at 2/s" "elapsed ${elapsed}s, want 1-5s"
fi

# ------------------------------------------------------------
# --quiet flag
# ------------------------------------------------------------
echo ""
echo "--- --quiet ---"

# --quiet suppresses usage error when --rate is omitted (fires after defer)
stderr=$(printf 'a\nb\n' | "$VRK" throttle --quiet 2>&1 >/dev/null) || true
exit_code=0; printf 'a\nb\n' | "$VRK" throttle --quiet > /dev/null 2>&1 || exit_code=$?
assert_exit         "--quiet error: exit 2 (missing --rate)" 2  "$exit_code"
assert_stderr_empty "--quiet error: stderr empty"                "$stderr"

# --quiet success: stdout unaffected
stdout=$(printf 'a\nb\n' | "$VRK" throttle --rate 1000/s --quiet 2>/dev/null)
stderr=$(printf 'a\nb\n' | "$VRK" throttle --rate 1000/s --quiet 2>&1 >/dev/null)
exit_code=0; printf 'a\nb\n' | "$VRK" throttle --rate 1000/s --quiet > /dev/null 2>&1 || exit_code=$?
assert_exit            "--quiet success: exit 0"               0    "$exit_code"
assert_stdout_contains "--quiet success: stdout has lines"     "a"  "$stdout"
assert_stderr_empty    "--quiet success: no stderr"                 "$stderr"

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
