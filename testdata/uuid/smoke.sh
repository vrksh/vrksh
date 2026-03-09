#!/usr/bin/env bash
# testdata/uuid/smoke.sh
#
# End-to-end smoke tests for vrk uuid.
# Run after make build to verify behaviour against the real binary.
#
# Usage:
#   ./testdata/uuid/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/uuid/smoke.sh   # explicit binary path
#
# Exit 0 if all pass. Exit 1 on first failure.

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0

UUID_RE='^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'

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

assert_stdout_matches() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -qE "$pattern"; then
    ok "$desc (matches $pattern)"
  else
    fail "$desc" "expected match for '$pattern', got: '$actual'"
  fi
}

assert_stdout_not_empty() {
  local desc=$1 stdout=$2
  if [ -n "$stdout" ]; then
    ok "$desc (stdout not empty)"
  else
    fail "$desc" "expected non-empty stdout, got empty"
  fi
}

assert_stdout_empty() {
  local desc=$1 stdout=$2
  if [ -z "$stdout" ]; then
    ok "$desc (stdout empty)"
  else
    fail "$desc" "expected empty stdout, got: $stdout"
  fi
}

assert_stderr_not_empty() {
  local desc=$1 stderr=$2
  if [ -n "$stderr" ]; then
    ok "$desc (stderr not empty)"
  else
    fail "$desc" "expected non-empty stderr, got empty"
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
  if [ "$actual" -eq "$expected" ]; then
    ok "$desc ($expected lines)"
  else
    fail "$desc" "expected $expected lines, got $actual"
  fi
}

echo "vrk uuid — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Default v4 UUID
# ------------------------------------------------------------
echo "--- default v4 ---"

stdout=$("$VRK" uuid 2>/dev/null) || true
exit_code=$(set +e; "$VRK" uuid > /dev/null 2>&1; echo $?)
assert_exit "default: exit 0" 0 "$exit_code"
assert_stdout_matches "default: UUID format" "$UUID_RE" "$stdout"

# Version nibble (char 15, 1-indexed = position 14 0-indexed) must be '4'
version_nibble="${stdout:14:1}"
if [ "$version_nibble" = "4" ]; then
  ok "default: version nibble = 4"
else
  fail "default: version nibble" "expected '4', got '$version_nibble' in UUID '$stdout'"
fi

# Two runs produce different values
uuid1=$("$VRK" uuid 2>/dev/null) || true
uuid2=$("$VRK" uuid 2>/dev/null) || true
if [ "$uuid1" != "$uuid2" ]; then
  ok "default: two calls produce different UUIDs"
else
  fail "default: uniqueness" "two calls produced the same UUID: $uuid1"
fi

# ------------------------------------------------------------
# 2. v7 UUID
# ------------------------------------------------------------
echo ""
echo "--- --v7 ---"

stdout=$("$VRK" uuid --v7 2>/dev/null) || true
exit_code=$(set +e; "$VRK" uuid --v7 > /dev/null 2>&1; echo $?)
assert_exit "--v7: exit 0" 0 "$exit_code"
assert_stdout_matches "--v7: UUID format" "$UUID_RE" "$stdout"

version_nibble="${stdout:14:1}"
if [ "$version_nibble" = "7" ]; then
  ok "--v7: version nibble = 7"
else
  fail "--v7: version nibble" "expected '7', got '$version_nibble' in UUID '$stdout'"
fi

# ------------------------------------------------------------
# 3. --count
# ------------------------------------------------------------
echo ""
echo "--- --count ---"

stdout=$("$VRK" uuid --count 5 2>/dev/null) || true
exit_code=$(set +e; "$VRK" uuid --count 5 > /dev/null 2>&1; echo $?)
assert_exit "--count 5: exit 0" 0 "$exit_code"
line_count=$(echo "$stdout" | wc -l | tr -d ' ')
assert_line_count "--count 5: line count" 5 "$line_count"

# All 5 must match UUID regex
while IFS= read -r line; do
  if ! echo "$line" | grep -qE "$UUID_RE"; then
    fail "--count 5: line format" "line '$line' does not match UUID regex"
  fi
done <<< "$stdout"
ok "--count 5: all lines are valid UUIDs"

# Short flag -n works
stdout=$("$VRK" uuid -n 3 2>/dev/null) || true
exit_code=$(set +e; "$VRK" uuid -n 3 > /dev/null 2>&1; echo $?)
assert_exit "-n 3: exit 0" 0 "$exit_code"
line_count=$(echo "$stdout" | wc -l | tr -d ' ')
assert_line_count "-n 3: line count" 3 "$line_count"

# v7 + count: lexicographically ordered
lines=$("$VRK" uuid --v7 --count 5 2>/dev/null) || true
prev=""
ordered=true
while IFS= read -r line; do
  if [ -n "$prev" ] && [ "$line" \< "$prev" ]; then
    ordered=false
    fail "--v7 --count 5: ordering" "UUID '$line' < '$prev' (not ordered)"
    break
  fi
  prev="$line"
done <<< "$lines"
if $ordered; then
  ok "--v7 --count 5: lexicographically ordered"
fi

# ------------------------------------------------------------
# 4. --json output
# ------------------------------------------------------------
echo ""
echo "--- --json ---"

stdout=$("$VRK" uuid --json 2>/dev/null) || true
exit_code=$(set +e; "$VRK" uuid --json > /dev/null 2>&1; echo $?)
assert_exit "--json: exit 0" 0 "$exit_code"

if command -v jq > /dev/null 2>&1; then
  # Parse and validate JSON fields
  uuid_val=$(echo "$stdout" | jq -r '.uuid' 2>/dev/null) || true
  version_val=$(echo "$stdout" | jq -r '.version' 2>/dev/null) || true
  gen_val=$(echo "$stdout" | jq -r '.generated_at' 2>/dev/null) || true

  assert_stdout_matches "--json: uuid field format" "$UUID_RE" "$uuid_val"

  if [ "$version_val" = "4" ]; then
    ok "--json: version = 4"
  else
    fail "--json: version field" "expected 4, got '$version_val'"
  fi

  if [ -n "$gen_val" ] && [ "$gen_val" != "null" ] && [ "$gen_val" -gt 0 ] 2>/dev/null; then
    ok "--json: generated_at is a positive integer"
  else
    fail "--json: generated_at field" "expected positive integer, got '$gen_val'"
  fi

  # --v7 --json: version = 7
  stdout=$("$VRK" uuid --v7 --json 2>/dev/null) || true
  version_val=$(echo "$stdout" | jq -r '.version' 2>/dev/null) || true
  if [ "$version_val" = "7" ]; then
    ok "--v7 --json: version = 7"
  else
    fail "--v7 --json: version field" "expected 7, got '$version_val'"
  fi

  # --count 5 --json: 5 JSONL lines, each valid JSON with uuid/version/generated_at
  stdout=$("$VRK" uuid --count 5 --json 2>/dev/null) || true
  line_count=$(echo "$stdout" | wc -l | tr -d ' ')
  assert_line_count "--count 5 --json: line count" 5 "$line_count"

  line_num=0
  while IFS= read -r line; do
    line_num=$((line_num + 1))
    if echo "$line" | jq -e '.uuid and .version and .generated_at' > /dev/null 2>&1; then
      true  # valid
    else
      fail "--count 5 --json: line $line_num" "missing required fields in: $line"
    fi
  done <<< "$stdout"
  ok "--count 5 --json: all lines have uuid/version/generated_at"
else
  echo "  SKIP  --json field validation (jq not installed)"
fi

# ------------------------------------------------------------
# 5. Error cases
# ------------------------------------------------------------
echo ""
echo "--- error cases ---"

# --count 0 → exit 2, stderr non-empty, stdout empty
stdout=$("$VRK" uuid --count 0 2>/dev/null) || true
stderr=$(set +e; "$VRK" uuid --count 0 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" uuid --count 0 > /dev/null 2>&1; echo $?)
assert_exit "--count 0: exit 2" 2 "$exit_code"
assert_stdout_empty "--count 0: stdout empty" "$stdout"
assert_stderr_not_empty "--count 0: stderr not empty" "$stderr"

# Unknown flag → exit 2
exit_code=$(set +e; "$VRK" uuid --bogus > /dev/null 2>&1; echo $?)
assert_exit "--bogus: exit 2" 2 "$exit_code"

# ------------------------------------------------------------
# 6. Stdin is ignored
# ------------------------------------------------------------
echo ""
echo "--- stdin ignored ---"

stdout=$(echo "this should be ignored" | "$VRK" uuid 2>/dev/null) || true
exit_code=$(set +e; echo "ignored" | "$VRK" uuid > /dev/null 2>&1; echo $?)
assert_exit "stdin ignored: exit 0" 0 "$exit_code"
assert_stdout_matches "stdin ignored: still produces UUID" "$UUID_RE" "$stdout"

# ------------------------------------------------------------
# 7. --help
# ------------------------------------------------------------
echo ""
echo "--- --help ---"

stdout=$("$VRK" uuid --help 2>/dev/null) || true
exit_code=$(set +e; "$VRK" uuid --help > /dev/null 2>&1; echo $?)
assert_exit "--help: exit 0" 0 "$exit_code"
assert_stdout_not_empty "--help: stdout not empty" "$stdout"

# ------------------------------------------------------------
# 8. --quiet
# ------------------------------------------------------------
echo ""
echo "--- --quiet ---"

# Error case (--count 0) with quiet should exit 2 and have no stderr
stdout=$("$VRK" uuid --quiet --count 0 2>/dev/null) || true
stderr=$(set +e; "$VRK" uuid --quiet --count 0 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" uuid --quiet --count 0 > /dev/null 2>&1; echo $?)
assert_exit            "--quiet error: exit 2"          2        "$exit_code"
assert_stdout_empty    "--quiet error: stdout empty"             "$stdout"
assert_stderr_empty    "--quiet error: stderr empty"             "$stderr"

# Valid generation with quiet should still produce stdout
stdout=$("$VRK" uuid --quiet 2>/dev/null) || true
exit_code=$(set +e; "$VRK" uuid --quiet > /dev/null 2>&1; echo $?)
assert_exit            "--quiet success: exit 0"        0        "$exit_code"
assert_stdout_matches  "--quiet success: value" "$UUID_RE"       "$stdout"

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
