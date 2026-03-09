#!/usr/bin/env bash
# testdata/epoch/smoke.sh
#
# Manual smoke tests for vrk epoch.
# Run after make build to verify end-to-end behaviour.
#
# Usage:
#   ./testdata/epoch/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/epoch/smoke.sh   # explicit binary path
#
# Exit 0 if all pass. Exit 1 on first failure.

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0

# Anchor timestamp: 2025-02-20T00:00:00Z = 1740009600
ANCHOR=1740009600

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

assert_stdout_contains() {
  local desc=$1 pattern=$2 stdout=$3
  if echo "$stdout" | grep -qF -- "$pattern"; then
    ok "$desc (stdout contains '$pattern')"
  else
    fail "$desc" "stdout did not contain '$pattern'. got: $stdout"
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

assert_stderr_contains() {
  local desc=$1 pattern=$2 stderr=$3
  if echo "$stderr" | grep -qF -- "$pattern"; then
    ok "$desc (stderr contains '$pattern')"
  else
    fail "$desc" "stderr did not contain '$pattern'. got: $stderr"
  fi
}

echo "vrk epoch — smoke tests"
echo "binary: $VRK"
echo "anchor: $ANCHOR = 2025-02-20T00:00:00Z"
echo ""

# ------------------------------------------------------------
# 1. --now bare: print current timestamp
# ------------------------------------------------------------
echo "--- --now bare ---"

stdout=$("$VRK" epoch --now 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch --now > /dev/null 2>&1; echo $?)
assert_exit "--now bare: exit 0" 0 "$exit_code"
assert_stdout_not_empty "--now bare: stdout not empty" "$stdout"
# Sanity: result should be a multi-digit integer (at least 10 digits for 2001+)
digit_count=${#stdout}
if [ "$digit_count" -ge 10 ]; then
  ok "--now bare: result looks like a unix timestamp ($stdout)"
else
  fail "--now bare: result too short" "got '$stdout', want 10+ digit unix timestamp"
fi

# ------------------------------------------------------------
# 2. Unix passthrough
# ------------------------------------------------------------
echo ""
echo "--- Unix passthrough ---"

# Positional arg
stdout=$("$VRK" epoch "$ANCHOR" 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch "$ANCHOR" > /dev/null 2>&1; echo $?)
assert_exit "passthrough arg: exit 0" 0 "$exit_code"
assert_stdout_equals "passthrough arg: value" "$ANCHOR" "$stdout"

# Stdin
stdout=$(echo "$ANCHOR" | "$VRK" epoch 2>/dev/null) || true
exit_code=$(set +e; echo "$ANCHOR" | "$VRK" epoch > /dev/null 2>&1; echo $?)
assert_exit "passthrough stdin: exit 0" 0 "$exit_code"
assert_stdout_equals "passthrough stdin: value" "$ANCHOR" "$stdout"

# Negative unix (pre-epoch): stdin form
stdout=$(echo "-1000" | "$VRK" epoch 2>/dev/null) || true
exit_code=$(set +e; echo "-1000" | "$VRK" epoch > /dev/null 2>&1; echo $?)
assert_exit "negative unix stdin: exit 0" 0 "$exit_code"
assert_stdout_equals "negative unix stdin: value" "-1000" "$stdout"

# ------------------------------------------------------------
# 3. ISO date → unix
# ------------------------------------------------------------
echo ""
echo "--- ISO date → unix ---"

stdout=$("$VRK" epoch 2025-02-20 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch 2025-02-20 > /dev/null 2>&1; echo $?)
assert_exit "ISO date arg: exit 0" 0 "$exit_code"
assert_stdout_equals "ISO date arg: value" "$ANCHOR" "$stdout"

# ISO datetime
stdout=$("$VRK" epoch 2025-02-20T10:00:00Z 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch 2025-02-20T10:00:00Z > /dev/null 2>&1; echo $?)
assert_exit "ISO datetime arg: exit 0" 0 "$exit_code"
assert_stdout_equals "ISO datetime arg: value" "1740045600" "$stdout"

# ------------------------------------------------------------
# 4. Relative times
# ------------------------------------------------------------
echo ""
echo "--- relative times ---"

# +3d via arg
stdout=$("$VRK" epoch +3d --at "$ANCHOR" 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch +3d --at "$ANCHOR" > /dev/null 2>&1; echo $?)
assert_exit "+3d arg: exit 0" 0 "$exit_code"
assert_stdout_equals "+3d arg: value" "1740268800" "$stdout"

# +3d via stdin
stdout=$(echo '+3d' | "$VRK" epoch --at "$ANCHOR" 2>/dev/null) || true
exit_code=$(set +e; echo '+3d' | "$VRK" epoch --at "$ANCHOR" > /dev/null 2>&1; echo $?)
assert_exit "+3d stdin: exit 0" 0 "$exit_code"
assert_stdout_equals "+3d stdin: value" "1740268800" "$stdout"

# -3d via positional arg (pre-pass extracts it before pflag)
stdout=$("$VRK" epoch -3d --at "$ANCHOR" 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch -3d --at "$ANCHOR" > /dev/null 2>&1; echo $?)
assert_exit "-3d arg: exit 0" 0 "$exit_code"
assert_stdout_equals "-3d arg: value" "1739750400" "$stdout"

# +6h via arg
stdout=$("$VRK" epoch +6h --at "$ANCHOR" 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch +6h --at "$ANCHOR" > /dev/null 2>&1; echo $?)
assert_exit "+6h arg: exit 0" 0 "$exit_code"
assert_stdout_equals "+6h arg: value" "1740031200" "$stdout"

# Bare unsigned (no sign) → exit 2
stdout=$(echo '3d' | "$VRK" epoch 2>/dev/null) || true
stderr=$(set +e; echo '3d' | "$VRK" epoch 2>&1 >/dev/null; true)
exit_code=$(set +e; echo '3d' | "$VRK" epoch > /dev/null 2>&1; echo $?)
assert_exit "unsigned 3d: exit 2" 2 "$exit_code"
assert_stdout_empty "unsigned 3d: stdout empty" "$stdout"
assert_stderr_contains "unsigned 3d: stderr sign required" "sign required" "$stderr"

# ------------------------------------------------------------
# 5. --iso output
# ------------------------------------------------------------
echo ""
echo "--- --iso output ---"

stdout=$("$VRK" epoch "$ANCHOR" --iso 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch "$ANCHOR" --iso > /dev/null 2>&1; echo $?)
assert_exit "--iso: exit 0" 0 "$exit_code"
assert_stdout_equals "--iso: value" "2025-02-20T00:00:00Z" "$stdout"

# --iso with numeric timezone offset
stdout=$("$VRK" epoch "$ANCHOR" --iso --tz +05:30 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch "$ANCHOR" --iso --tz +05:30 > /dev/null 2>&1; echo $?)
assert_exit "--iso --tz +05:30: exit 0" 0 "$exit_code"
assert_stdout_equals "--iso --tz +05:30: value" "2025-02-20T05:30:00+05:30" "$stdout"

# --iso with IANA timezone
stdout=$("$VRK" epoch "$ANCHOR" --iso --tz America/New_York 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch "$ANCHOR" --iso --tz America/New_York > /dev/null 2>&1; echo $?)
assert_exit "--iso --tz IANA: exit 0" 0 "$exit_code"
assert_stdout_equals "--iso --tz IANA: value (EST)" "2025-02-19T19:00:00-05:00" "$stdout"

# --iso relative time
stdout=$(echo '+3d' | "$VRK" epoch --iso --at "$ANCHOR" 2>/dev/null) || true
exit_code=$(set +e; echo '+3d' | "$VRK" epoch --iso --at "$ANCHOR" > /dev/null 2>&1; echo $?)
assert_exit "--iso +3d: exit 0" 0 "$exit_code"
assert_stdout_equals "--iso +3d: value" "2025-02-23T00:00:00Z" "$stdout"

# ------------------------------------------------------------
# 6. Error cases
# ------------------------------------------------------------
echo ""
echo "--- error cases ---"

# Ambiguous timezone abbreviation
stdout=$("$VRK" epoch "$ANCHOR" --iso --tz IST 2>/dev/null) || true
stderr=$(set +e; "$VRK" epoch "$ANCHOR" --iso --tz IST 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" epoch "$ANCHOR" --iso --tz IST > /dev/null 2>&1; echo $?)
assert_exit "--tz IST ambiguous: exit 2" 2 "$exit_code"
assert_stdout_empty "--tz IST ambiguous: stdout empty" "$stdout"
assert_stderr_contains "--tz IST ambiguous: stderr contains 'ambiguous'" "ambiguous" "$stderr"

# --tz without --iso
stdout=$("$VRK" epoch "$ANCHOR" --tz America/New_York 2>/dev/null) || true
stderr=$(set +e; "$VRK" epoch "$ANCHOR" --tz America/New_York 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" epoch "$ANCHOR" --tz America/New_York > /dev/null 2>&1; echo $?)
assert_exit "--tz without --iso: exit 2" 2 "$exit_code"
assert_stdout_empty "--tz without --iso: stdout empty" "$stdout"
assert_stderr_contains "--tz without --iso: stderr mentions --iso" "--tz requires --iso" "$stderr"

# Natural language
stdout=$(echo 'next tuesday' | "$VRK" epoch 2>/dev/null) || true
stderr=$(set +e; echo 'next tuesday' | "$VRK" epoch 2>&1 >/dev/null; true)
exit_code=$(set +e; echo 'next tuesday' | "$VRK" epoch > /dev/null 2>&1; echo $?)
assert_exit "natural language: exit 2" 2 "$exit_code"
assert_stdout_empty "natural language: stdout empty" "$stdout"
assert_stderr_contains "natural language: stderr mentions 'natural language'" "natural language" "$stderr"

# No input
exit_code=$(set +e; "$VRK" epoch < /dev/null > /dev/null 2>&1; echo $?)
assert_exit "no input: exit 2" 2 "$exit_code"

# --at with no input
exit_code=$(set +e; "$VRK" epoch --at "$ANCHOR" < /dev/null > /dev/null 2>&1; echo $?)
assert_exit "--at no input: exit 2" 2 "$exit_code"

# Unknown flag
exit_code=$(set +e; "$VRK" epoch --not-a-flag "$ANCHOR" > /dev/null 2>&1; echo $?)
assert_exit "unknown flag: exit 2" 2 "$exit_code"

# ------------------------------------------------------------
# 7. --help
# ------------------------------------------------------------
echo ""
echo "--- --help ---"

stdout=$("$VRK" epoch --help 2>/dev/null) || true
exit_code=$(set +e; "$VRK" epoch --help > /dev/null 2>&1; echo $?)
assert_exit "--help: exit 0" 0 "$exit_code"
assert_stdout_contains "--help: mentions usage" "usage" "$stdout"
assert_stdout_contains "--help: mentions --iso" "iso" "$stdout"
assert_stdout_contains "--help: mentions --tz" "tz" "$stdout"
assert_stdout_contains "--help: mentions --now" "now" "$stdout"

# ------------------------------------------------------------
# 8. Pipeline usage
# ------------------------------------------------------------
echo ""
echo "--- pipeline ---"

# epoch | jq (if available)
if command -v jq > /dev/null 2>&1; then
  ts=$(echo '2025-02-20' | "$VRK" epoch 2>/dev/null) || true
  assert_stdout_equals "pipeline: ISO date to unix" "$ANCHOR" "$ts"
else
  echo "  SKIP  pipeline jq test (jq not installed)"
fi

# Deterministic: same --at always gives same result
ts1=$(echo '+3d' | "$VRK" epoch --at "$ANCHOR" 2>/dev/null) || true
ts2=$(echo '+3d' | "$VRK" epoch --at "$ANCHOR" 2>/dev/null) || true
if [ "$ts1" = "$ts2" ] && [ "$ts1" = "1740268800" ]; then
  ok "pipeline: deterministic --at (got $ts1)"
else
  fail "pipeline: deterministic --at" "expected 1740268800, got ts1=$ts1 ts2=$ts2"
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
