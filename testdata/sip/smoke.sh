#!/usr/bin/env bash
# testdata/sip/smoke.sh
#
# End-to-end smoke tests for vrk sip.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, determinism, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/sip/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/sip/smoke.sh   # explicit binary path

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

assert_line_count() {
  local desc=$1 expected=$2 actual=$3
  local count
  if [ -z "$actual" ]; then
    count=0
  else
    count=$(printf '%s\n' "$actual" | wc -l | tr -d ' ')
  fi
  if [ "$count" -eq "$expected" ]; then
    ok "$desc ($expected lines)"
  else
    fail "$desc" "expected $expected lines, got $count"
  fi
}

assert_contains() {
  local desc=$1 pattern=$2 actual=$3
  if printf '%s\n' "$actual" | grep -qF "$pattern"; then
    ok "$desc (contains '$pattern')"
  else
    fail "$desc" "stdout did not contain '$pattern'. got: $actual"
  fi
}

assert_empty() {
  local desc=$1 actual=$2
  if [ -z "$actual" ]; then
    ok "$desc (empty)"
  else
    fail "$desc" "expected empty, got: $actual"
  fi
}

echo "vrk sip — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. --first
# ------------------------------------------------------------
echo "--- 1. --first ---"

got=$(seq 100 | "$VRK" sip --first 10)
assert_line_count "--first 10 on seq 100: 10 lines"  10 "$got"
first_line=$(printf '%s\n' "$got" | head -1)
last_line=$(printf '%s\n' "$got" | tail -1)
if [ "$first_line" = "1" ]; then ok "--first 10: first line is '1'"; else fail "--first 10: first line" "expected '1', got '$first_line'"; fi
if [ "$last_line" = "10" ]; then ok "--first 10: last line is '10'"; else fail "--first 10: last line" "expected '10', got '$last_line'"; fi

got=$(seq 5 | "$VRK" sip --first 10)
exit_code=$?
assert_exit "--first 10 on seq 5: exit 0"  0  "$exit_code"
assert_line_count "--first 10 on seq 5: 5 lines"  5  "$got"

set +e
"$VRK" sip --first 0 < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--first 0: exit 2"  2  "$exit_code"

# ------------------------------------------------------------
# 2. --every
# ------------------------------------------------------------
echo ""
echo "--- 2. --every ---"

got=$(seq 100 | "$VRK" sip --every 10)
assert_line_count "--every 10 on seq 100: 10 lines"  10  "$got"
first_line=$(printf '%s\n' "$got" | head -1)
last_line=$(printf '%s\n' "$got" | tail -1)
if [ "$first_line" = "10" ]; then ok "--every 10: first line is '10'"; else fail "--every 10: first line" "expected '10', got '$first_line'"; fi
if [ "$last_line" = "100" ]; then ok "--every 10: last line is '100'"; else fail "--every 10: last line" "expected '100', got '$last_line'"; fi

got=$(seq 7 | "$VRK" sip --every 3)
assert_line_count "--every 3 on seq 7: 2 lines"  2  "$got"
first_line=$(printf '%s\n' "$got" | head -1)
last_line=$(printf '%s\n' "$got" | tail -1)
if [ "$first_line" = "3" ]; then ok "--every 3 on seq 7: first line is '3'"; else fail "--every 3 on seq 7: first line" "expected '3', got '$first_line'"; fi
if [ "$last_line" = "6" ]; then ok "--every 3 on seq 7: last line is '6'"; else fail "--every 3 on seq 7: last line" "expected '6', got '$last_line'"; fi

set +e
"$VRK" sip --every 0 < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--every 0: exit 2"  2  "$exit_code"

# ------------------------------------------------------------
# 3. --count / -n (reservoir)
# ------------------------------------------------------------
echo ""
echo "--- 3. --count / -n (reservoir) ---"

got=$(seq 1000 | "$VRK" sip --count 100)
assert_line_count "--count 100 on seq 1000: 100 lines"  100  "$got"

got=$(seq 5 | "$VRK" sip --count 10)
exit_code=$?
assert_exit "--count 10 on seq 5: exit 0"  0  "$exit_code"
assert_line_count "--count 10 on seq 5: 5 lines"  5  "$got"

# -n short form must work.
got=$(seq 1000 | "$VRK" sip -n 100)
assert_line_count "-n 100 on seq 1000: 100 lines"  100  "$got"

set +e
"$VRK" sip --count 0 < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--count 0: exit 2"  2  "$exit_code"

# Seed determinism: same seed → same output.
out1=$(seq 1000 | "$VRK" sip --count 100 --seed 42)
out2=$(seq 1000 | "$VRK" sip --count 100 --seed 42)
if [ "$out1" = "$out2" ]; then
  ok "--count --seed 42 twice: identical output"
else
  fail "--count --seed 42 twice" "outputs differ — seed is not deterministic"
fi

# Different seeds → different output (overwhelmingly likely).
out42=$(seq 1000 | "$VRK" sip --count 100 --seed 42)
out99=$(seq 1000 | "$VRK" sip --count 100 --seed 99)
if [ "$out42" != "$out99" ]; then
  ok "--seed 42 vs --seed 99: different output"
else
  fail "--seed 42 vs --seed 99" "identical output — different seeds should differ"
fi

# Seed 0 is valid.
got=$(seq 100 | "$VRK" sip --count 10 --seed 0)
exit_code=$?
assert_exit "--count --seed 0: exit 0 (seed 0 is valid)"  0  "$exit_code"
assert_line_count "--count --seed 0: 10 lines"  10  "$got"

# Seed 0 is deterministic (not falling back to random).
out0a=$(seq 1000 | "$VRK" sip --count 100 --seed 0)
out0b=$(seq 1000 | "$VRK" sip --count 100 --seed 0)
if [ "$out0a" = "$out0b" ]; then
  ok "--seed 0 twice: identical output (not random)"
else
  fail "--seed 0 twice" "outputs differ — seed 0 must not fall back to random"
fi

# ------------------------------------------------------------
# 4. --sample
# ------------------------------------------------------------
echo ""
echo "--- 4. --sample ---"

got=$(seq 1000 | "$VRK" sip --sample 100)
exit_code=$?
assert_exit "--sample 100: exit 0"  0  "$exit_code"
assert_line_count "--sample 100 on seq 1000: all 1000 lines"  1000  "$got"

set +e
"$VRK" sip --sample 0 < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--sample 0: exit 2"  2  "$exit_code"

set +e
"$VRK" sip --sample 101 < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--sample 101: exit 2"  2  "$exit_code"

# Seed determinism for --sample.
outs1=$(seq 1000 | "$VRK" sip --sample 10 --seed 42)
outs2=$(seq 1000 | "$VRK" sip --sample 10 --seed 42)
if [ "$outs1" = "$outs2" ]; then
  ok "--sample --seed 42 twice: identical output"
else
  fail "--sample --seed 42 twice" "outputs differ — seed is not deterministic for --sample"
fi

# ------------------------------------------------------------
# 5. Mutual exclusion
# ------------------------------------------------------------
echo ""
echo "--- 5. mutual exclusion ---"

set +e
seq 100 | "$VRK" sip --first 5 --count 10 > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--first + --count: exit 2"  2  "$exit_code"

set +e
seq 100 | "$VRK" sip --every 5 --sample 10 > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "--every + --sample: exit 2"  2  "$exit_code"

# ------------------------------------------------------------
# 6. --json trailer
# ------------------------------------------------------------
echo ""
echo "--- 6. --json trailer ---"

json_out=$(seq 100 | "$VRK" sip --first 5 --json)
data_lines=$(printf '%s\n' "$json_out" | head -5 | wc -l | tr -d ' ')
if [ "$data_lines" -eq 5 ]; then ok "--first 5 --json: 5 data lines"; else fail "--first 5 --json: data lines" "expected 5, got $data_lines"; fi
last_line=$(printf '%s\n' "$json_out" | tail -1)
assert_contains "--first 5 --json: _vrk field"       '"_vrk":"sip"'         "$last_line"
assert_contains "--first 5 --json: strategy field"   '"strategy":"first"'   "$last_line"
assert_contains "--first 5 --json: requested field"  '"requested":5'        "$last_line"
assert_contains "--first 5 --json: returned field"   '"returned":5'         "$last_line"
assert_contains "--first 5 --json: total_seen field" '"total_seen":100'     "$last_line"

json_out=$(seq 5 | "$VRK" sip --count 10 --json)
last_line=$(printf '%s\n' "$json_out" | tail -1)
assert_contains "--count 10 on seq 5 --json: strategy=reservoir"  '"strategy":"reservoir"'  "$last_line"
assert_contains "--count 10 on seq 5 --json: returned=5"          '"returned":5'            "$last_line"
assert_contains "--count 10 on seq 5 --json: total_seen=5"        '"total_seen":5'          "$last_line"

# ------------------------------------------------------------
# 7. Empty stdin
# ------------------------------------------------------------
echo ""
echo "--- 7. empty stdin ---"

got=$(printf '' | "$VRK" sip --count 10)
exit_code=$?
assert_exit "printf '' | sip --count 10: exit 0"  0  "$exit_code"
assert_empty "printf '' | sip --count 10: no output"  "$got"

got=$(echo '' | "$VRK" sip --count 10)
exit_code=$?
assert_exit "echo '' | sip --count 10: exit 0 (empty line skipped)"  0  "$exit_code"
assert_empty "echo '' | sip --count 10: no output (empty line skipped)"  "$got"

got=$("$VRK" sip --count 10 </dev/null)
exit_code=$?
assert_exit "sip --count 10 </dev/null: exit 0"  0  "$exit_code"
assert_empty "sip --count 10 </dev/null: no output"  "$got"

# ------------------------------------------------------------
# 8. --help and usage errors
# ------------------------------------------------------------
echo ""
echo "--- 8. --help and usage errors ---"

"$VRK" sip --help > /dev/null
exit_code=$?
assert_exit "--help: exit 0"  0  "$exit_code"

set +e
"$VRK" sip --bogus < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "unknown flag: exit 2"  2  "$exit_code"

set +e
"$VRK" sip < /dev/null > /dev/null 2>&1
exit_code=$?
set -e
assert_exit "no strategy flag: exit 2"  2  "$exit_code"

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
