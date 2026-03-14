#!/usr/bin/env bash
# testdata/emit/smoke.sh
#
# End-to-end smoke tests for vrk emit.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/emit/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/emit/smoke.sh   # explicit binary path
#
# NOTE: TTY detection (exit 2 when run interactively with no pipe) cannot be
# automated here — /dev/null is empty non-interactive stdin (exits 0 with no
# output). Verify manually: run `vrk emit` in a terminal with no pipe and
# confirm it exits 2. Covered in unit tests via the isTerminal mock.
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

assert_stdout_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -qF "$pattern"; then
    ok "$desc (stdout contains '$pattern')"
  else
    fail "$desc" "stdout did not contain '$pattern'. got: $actual"
  fi
}

assert_stdout_not_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -qF "$pattern"; then
    fail "$desc" "stdout must NOT contain '$pattern' but did: $actual"
  else
    ok "$desc (stdout does not contain '$pattern')"
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
    count=$(echo "$actual" | wc -l | tr -d ' ')
  fi
  if [ "$count" -eq "$expected" ]; then
    ok "$desc ($expected lines)"
  else
    fail "$desc" "expected $expected lines, got $count. stdout: $actual"
  fi
}

assert_valid_jsonl() {
  local desc=$1 actual=$2
  if command -v jq > /dev/null 2>&1; then
    local ok_count=0 total=0
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      total=$((total + 1))
      if echo "$line" | jq -e . > /dev/null 2>&1; then
        ok_count=$((ok_count + 1))
      fi
    done <<< "$actual"
    if [ "$ok_count" -eq "$total" ]; then
      ok "$desc (all $total lines valid JSON)"
    else
      fail "$desc" "some lines are not valid JSON: $actual"
    fi
  else
    echo "  SKIP  $desc (jq not installed)"
    PASS=$((PASS + 1))
  fi
}

echo "vrk emit — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Basic pipeline
# ------------------------------------------------------------
echo "--- basic pipeline ---"

stdout=$(echo 'Starting job' | "$VRK" emit 2>/dev/null)
stderr=$(set +e; echo 'Starting job' | "$VRK" emit 2>&1 >/dev/null; true)
exit_code=$(set +e; echo 'Starting job' | "$VRK" emit >/dev/null 2>&1; echo $?)

assert_exit         "basic: exit 0"             0         "$exit_code"
assert_stderr_empty "basic: stderr empty"                 "$stderr"
assert_line_count   "basic: one record"          1         "$stdout"
assert_stdout_contains "basic: ts field"         '"ts"'    "$stdout"
assert_stdout_contains "basic: level info"       '"info"'  "$stdout"
assert_stdout_contains "basic: msg"              '"Starting job"' "$stdout"
assert_valid_jsonl  "basic: valid JSONL"                   "$stdout"

# ------------------------------------------------------------
# 2. --level flag
# ------------------------------------------------------------
echo ""
echo "--- --level flag ---"

stdout=$(echo 'msg' | "$VRK" emit --level warn 2>/dev/null)
exit_code=$(set +e; echo 'msg' | "$VRK" emit --level warn >/dev/null 2>&1; echo $?)
assert_exit            "level warn: exit 0"      0       "$exit_code"
assert_stdout_contains "level warn: level=warn"  '"warn"' "$stdout"
assert_valid_jsonl     "level warn: valid JSONL"          "$stdout"

stdout=$(echo 'msg' | "$VRK" emit -l error 2>/dev/null)
exit_code=$(set +e; echo 'msg' | "$VRK" emit -l error >/dev/null 2>&1; echo $?)
assert_exit            "level error short: exit 0"       0        "$exit_code"
assert_stdout_contains "level error short: level=error"  '"error"' "$stdout"

# Invalid level → exit 2
exit_code=$(set +e; echo 'msg' | "$VRK" emit --level bad >/dev/null 2>&1; echo $?)
stdout_bad=$(set +e; echo 'msg' | "$VRK" emit --level bad 2>/dev/null; true)
assert_exit        "level invalid: exit 2"         2 "$exit_code"
assert_stdout_empty "level invalid: stdout empty"     "$stdout_bad"

# ------------------------------------------------------------
# 3. --tag flag
# ------------------------------------------------------------
echo ""
echo "--- --tag flag ---"

stdout=$(echo 'msg' | "$VRK" emit --tag myapp 2>/dev/null)
exit_code=$(set +e; echo 'msg' | "$VRK" emit --tag myapp >/dev/null 2>&1; echo $?)
assert_exit            "tag: exit 0"            0          "$exit_code"
assert_stdout_contains "tag: tag field present" '"myapp"'  "$stdout"

# Verify tag appears between level and msg in the raw output line.
# Format: {"ts":"...","level":"...","tag":"myapp","msg":"..."}
tag_idx=$(echo "$stdout" | grep -bo '"tag"' | head -1 | cut -d: -f1 || echo 0)
msg_idx=$(echo "$stdout" | grep -bo '"msg"' | head -1 | cut -d: -f1 || echo 0)
level_idx=$(echo "$stdout" | grep -bo '"level"' | head -1 | cut -d: -f1 || echo 0)
if [ "$level_idx" -lt "$tag_idx" ] && [ "$tag_idx" -lt "$msg_idx" ]; then
  ok "tag: field order level < tag < msg"
else
  fail "tag: field order wrong" "level=$level_idx tag=$tag_idx msg=$msg_idx"
fi

# --tag "" omits the field.
stdout=$(echo 'msg' | "$VRK" emit --tag '' 2>/dev/null)
assert_stdout_not_contains "tag empty: no tag field" '"tag"' "$stdout"

# ------------------------------------------------------------
# 4. --msg flag with JSON stdin merge
# ------------------------------------------------------------
echo ""
echo "--- --msg flag ---"

stdout=$(echo '{"job_id":"abc"}' | "$VRK" emit --level error --msg "Job failed" 2>/dev/null)
exit_code=$(set +e; echo '{"job_id":"abc"}' | "$VRK" emit --level error --msg "Job failed" >/dev/null 2>&1; echo $?)
assert_exit            "msg merge: exit 0"            0             "$exit_code"
assert_stdout_contains "msg merge: msg overridden"    '"Job failed"' "$stdout"
assert_stdout_contains "msg merge: level=error"       '"error"'      "$stdout"
assert_stdout_contains "msg merge: job_id present"    '"job_id"'     "$stdout"
assert_stdout_contains "msg merge: job_id value"      '"abc"'        "$stdout"
assert_valid_jsonl     "msg merge: valid JSONL"                       "$stdout"

# Core fields from stdin JSON must not override flag values.
stdout=$(echo '{"msg":"overridden","level":"debug"}' | "$VRK" emit --msg "mine" --level info 2>/dev/null)
assert_stdout_contains     "msg core fields win: msg=mine"  '"mine"'  "$stdout"
assert_stdout_not_contains "msg core fields win: no debug"  '"debug"' "$stdout"

# Plain-text stdin with --msg: no extra fields.
stdout=$(echo 'plain text' | "$VRK" emit --msg "override" 2>/dev/null)
assert_stdout_contains "msg plain stdin: msg=override" '"override"' "$stdout"
assert_stdout_not_contains "msg plain stdin: no plain" '"plain"' "$stdout"

# ------------------------------------------------------------
# 5. --parse-level
# ------------------------------------------------------------
echo ""
echo "--- --parse-level ---"

# Known prefixes are detected and stripped.
stdout=$(printf 'ERROR: disk full\nWARN: low memory\nINFO: ok\nDEBUG: verbose\n' | "$VRK" emit --parse-level 2>/dev/null)
exit_code=$(set +e; printf 'ERROR: disk full\nWARN: low memory\nINFO: ok\nDEBUG: verbose\n' | "$VRK" emit --parse-level >/dev/null 2>&1; echo $?)
assert_exit       "parse-level: exit 0"    0  "$exit_code"
assert_line_count "parse-level: 4 records" 4  "$stdout"
assert_stdout_contains "parse-level: error detected"  '"error"'   "$stdout"
assert_stdout_contains "parse-level: warn detected"   '"warn"'    "$stdout"
assert_stdout_contains "parse-level: disk full"       '"disk full"' "$stdout"
assert_stdout_contains "parse-level: low memory"      '"low memory"' "$stdout"
assert_valid_jsonl "parse-level: valid JSONL" "$stdout"

# WARNING maps to warn.
stdout=$(echo 'WARNING: out of space' | "$VRK" emit --parse-level 2>/dev/null)
assert_stdout_contains     "parse-level WARNING→warn: warn"      '"warn"'         "$stdout"
assert_stdout_not_contains "parse-level WARNING→warn: no warning" '"warning"'     "$stdout"
assert_stdout_contains     "parse-level WARNING→warn: msg"       '"out of space"' "$stdout"

# Unknown prefix falls back to --level value (not hardcoded "info").
stdout=$(echo '[ERROR] crash' | "$VRK" emit --parse-level --level warn 2>/dev/null)
assert_stdout_contains     "parse-level unknown: falls back to --level" '"warn"'       "$stdout"
assert_stdout_contains     "parse-level unknown: msg unchanged"         '"[ERROR] crash"' "$stdout"

# Default fallback to "info" when --level not set.
stdout=$(echo 'nothing to match' | "$VRK" emit --parse-level 2>/dev/null)
assert_stdout_contains "parse-level default: info" '"info"' "$stdout"

# ------------------------------------------------------------
# 6. Empty lines skipped
# ------------------------------------------------------------
echo ""
echo "--- empty line handling ---"

stdout=$(echo '' | "$VRK" emit 2>/dev/null)
exit_code=$(set +e; echo '' | "$VRK" emit >/dev/null 2>&1; echo $?)
assert_exit        "empty line: exit 0"         0  "$exit_code"
assert_stdout_empty "empty line: stdout empty"     "$stdout"

stdout=$(printf '\n\n\n' | "$VRK" emit 2>/dev/null)
exit_code=$(set +e; printf '\n\n\n' | "$VRK" emit >/dev/null 2>&1; echo $?)
assert_exit        "all empty: exit 0"          0  "$exit_code"
assert_stdout_empty "all empty: stdout empty"      "$stdout"

# Mixed empty and non-empty: only real lines produce records.
stdout=$(printf '\nfirst\n\nsecond\n\n' | "$VRK" emit 2>/dev/null)
assert_line_count "mixed empty: 2 records" 2 "$stdout"

# ------------------------------------------------------------
# 7. Positional arg
# ------------------------------------------------------------
echo ""
echo "--- positional arg ---"

stdout=$("$VRK" emit 'Starting job' 2>/dev/null)
exit_code=$(set +e; "$VRK" emit 'Starting job' >/dev/null 2>&1; echo $?)
assert_exit            "positional: exit 0"           0               "$exit_code"
assert_line_count      "positional: one record"        1               "$stdout"
assert_stdout_contains "positional: msg"               '"Starting job"' "$stdout"
assert_valid_jsonl     "positional: valid JSONL"                        "$stdout"

# ------------------------------------------------------------
# 8. stdout/stderr separation
# ------------------------------------------------------------
echo ""
echo "--- stdout/stderr separation ---"

# Normal run: stderr must be empty.
stderr=$(set +e; echo 'hello' | "$VRK" emit 2>&1 >/dev/null; true)
assert_stderr_empty "normal run: stderr empty" "$stderr"

# Unknown flag: stdout must be empty, error goes to stderr.
stdout_bad=$(set +e; echo 'msg' | "$VRK" emit --bogus 2>/dev/null; true)
assert_stdout_empty "unknown flag: stdout empty" "$stdout_bad"

# ------------------------------------------------------------
# 9. --help
# ------------------------------------------------------------
echo ""
echo "--- --help ---"

stdout=$("$VRK" emit --help 2>/dev/null) || true
exit_code=$(set +e; "$VRK" emit --help >/dev/null 2>&1; echo $?)
assert_exit            "help: exit 0"             0        "$exit_code"
assert_stdout_contains "help: mentions emit"      "emit"   "$stdout"
assert_stdout_contains "help: mentions --level"   "level"  "$stdout"
assert_stdout_contains "help: mentions --tag"     "tag"    "$stdout"
assert_stdout_contains "help: mentions --msg"     "msg"    "$stdout"

# ------------------------------------------------------------
# 10. Usage errors
# ------------------------------------------------------------
echo ""
echo "--- usage errors ---"

exit_code=$(set +e; echo 'msg' | "$VRK" emit --bogus-flag >/dev/null 2>&1; echo $?)
assert_exit "unknown flag: exit 2" 2 "$exit_code"

exit_code=$(set +e; echo 'msg' | "$VRK" emit --level bad >/dev/null 2>&1; echo $?)
assert_exit "invalid level: exit 2" 2 "$exit_code"

# ------------------------------------------------------------
# 11. Killer pipeline: parse mixed log output
# ------------------------------------------------------------
echo ""
echo "--- killer pipeline ---"

log_output() {
  printf 'ERROR: connection refused\nWARN: retrying\nINFO: connected\nDEBUG: sent 42 bytes\n'
}

stdout=$(log_output | "$VRK" emit --parse-level --tag myservice 2>/dev/null)
exit_code=$(set +e; log_output | "$VRK" emit --parse-level --tag myservice >/dev/null 2>&1; echo $?)
assert_exit       "pipeline: exit 0"     0  "$exit_code"
assert_line_count "pipeline: 4 records"  4  "$stdout"
assert_stdout_contains "pipeline: tag present"     '"myservice"'         "$stdout"
assert_stdout_contains "pipeline: error level"     '"error"'             "$stdout"
assert_stdout_contains "pipeline: warn level"      '"warn"'              "$stdout"
assert_stdout_contains "pipeline: connection msg"  '"connection refused"' "$stdout"
assert_valid_jsonl "pipeline: all records valid JSONL" "$stdout"

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
