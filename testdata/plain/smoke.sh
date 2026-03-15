#!/usr/bin/env bash
# testdata/plain/smoke.sh
#
# End-to-end smoke tests for vrk plain.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/plain/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/plain/smoke.sh   # explicit binary path

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

echo "vrk plain — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Basic markdown stripping
# ------------------------------------------------------------
echo "--- 1. basic stripping ---"

got=$(echo '**hello** _world_' | "$VRK" plain)
assert_stdout_equals "bold and italic stripped"   "hello world"                   "$got"

got=$(echo '# Heading' | "$VRK" plain)
assert_stdout_equals "heading stripped"           "Heading"                       "$got"

got=$(printf -- '- item one\n- item two' | "$VRK" plain)
assert_stdout_equals "list items stripped"        "$(printf 'item one\nitem two')" "$got"

got=$(echo '[link text](https://example.com)' | "$VRK" plain)
assert_stdout_equals "link: text kept, URL dropped"  "link text"                 "$got"

got=$(echo '`code snippet`' | "$VRK" plain)
assert_stdout_equals "inline code stripped"       "code snippet"                  "$got"

got=$(echo '> blockquote text' | "$VRK" plain)
assert_stdout_equals "blockquote stripped"        "blockquote text"               "$got"

# ------------------------------------------------------------
# 2. Empty stdin
# ------------------------------------------------------------
echo ""
echo "--- 2. empty stdin ---"

got=$(printf '' | "$VRK" plain)
exit_code=$?
assert_exit        "empty stdin: exit 0"    0  "$exit_code"
assert_stdout_empty "empty stdin: no output"   "$got"

# ------------------------------------------------------------
# 3. --json output
# ------------------------------------------------------------
echo ""
echo "--- 3. --json output ---"

json_out=$(echo '**hello**' | "$VRK" plain --json)

text_val=$(printf '%s' "$json_out" | grep -o '"text":"[^"]*"' | sed 's/"text":"//;s/"//')
assert_stdout_equals "json: text field value"  "hello"  "$text_val"

has_input=$(printf '%s' "$json_out" | grep -c '"input_bytes"' || true)
assert_stdout_equals "json: input_bytes present"  "1"  "$has_input"

has_output=$(printf '%s' "$json_out" | grep -c '"output_bytes"' || true)
assert_stdout_equals "json: output_bytes present"  "1"  "$has_output"

echo '**hello**' | "$VRK" plain --json > /dev/null
exit_code=$?
assert_exit "json: exit 0" 0 "$exit_code"

# ------------------------------------------------------------
# 4. --help and usage errors
# ------------------------------------------------------------
echo ""
echo "--- 4. --help and usage errors ---"

"$VRK" plain --help > /dev/null
exit_code=$?
assert_exit "--help: exit 0" 0 "$exit_code"

set +e
"$VRK" plain --bogus < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "unknown flag: exit 2" 2 "$exit_code"

# ------------------------------------------------------------
# 5. Positional argument
# ------------------------------------------------------------
echo ""
echo "--- 5. positional argument ---"

# Basic: positional arg exits 0 and output equals stripped text.
got=$("$VRK" plain '**hello** world')
exit_code=$?
assert_exit        "positional: exit 0"    0             "$exit_code"
assert_stdout_equals "positional: stripped" "hello world" "$got"

# Positional output == stdin output.
# echo appends a newline; the tool strips exactly one trailing newline from stdin.
# The positional path receives no trailing newline from the shell, so outputs match.
got_stdin=$(echo '**hello** world' | "$VRK" plain)
assert_stdout_equals "positional == stdin" "$got_stdin" "$got"

# --json combined with positional arg.
got=$("$VRK" plain '**hello**' --json)
exit_code=$?
assert_exit            "positional --json: exit 0"     0        "$exit_code"
assert_stdout_contains "positional --json: text field" '"text":"hello"' "$got"

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
