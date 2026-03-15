#!/usr/bin/env bash
# testdata/moniker/smoke.sh
#
# End-to-end smoke tests for vrk moniker.
# Run after: make build
#
# Usage:
#   bash testdata/moniker/smoke.sh
#   VRK=./vrk bash testdata/moniker/smoke.sh

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc"
    echo "  expected: $expected"
    echo "  got:      $actual"
    FAIL=$((FAIL + 1))
  fi
}

assert_exit() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$actual" -eq "$expected" ]; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (expected exit $expected, got $actual)"
    FAIL=$((FAIL + 1))
  fi
}

assert_contains() {
  local desc="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (expected to contain: $needle)"
    echo "  got: $haystack"
    FAIL=$((FAIL + 1))
  fi
}

assert_match() {
  local desc="$1" pattern="$2" actual="$3"
  if echo "$actual" | grep -qE "$pattern"; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (expected to match: $pattern)"
    echo "  got: $actual"
    FAIL=$((FAIL + 1))
  fi
}

# Smoke 1: default output — one line, lowercase, two words, hyphen-separated
out=$("$VRK" moniker)
assert_match "default output matches adjective-noun pattern" '^[a-z]+-[a-z]+$' "$out"

# Smoke 2: --count 5 produces exactly 5 lines
line_count=$("$VRK" moniker --count 5 | wc -l | tr -d ' ')
assert_eq "--count 5 produces 5 lines" "5" "$line_count"

# Smoke 3: --count 0 exits 2
"$VRK" moniker --count 0 2>/dev/null || exit_code=$?
assert_exit "--count 0 exits 2" 2 "${exit_code:-0}"
unset exit_code

# Smoke 4: --separator _ uses underscore
out=$("$VRK" moniker --separator _)
assert_match "--separator _ output matches adj_noun pattern" '^[a-z]+_[a-z]+$' "$out"

# Smoke 5: --words 3 produces exactly 2 hyphens (3 words)
out=$("$VRK" moniker --words 3)
hyphen_count=$(echo "$out" | tr -cd '-' | wc -c | tr -d ' ')
assert_eq "--words 3 has 2 hyphens" "2" "$hyphen_count"

# Smoke 6: --seed 42 produces identical output on two consecutive runs
out1=$("$VRK" moniker --seed 42)
out2=$("$VRK" moniker --seed 42)
assert_eq "--seed 42 is deterministic" "$out1" "$out2"

# Smoke 7: --seed 42 and --seed 99 produce different output
out42=$("$VRK" moniker --seed 42)
out99=$("$VRK" moniker --seed 99)
if [ "$out42" != "$out99" ]; then
  echo "PASS: --seed 42 and --seed 99 produce different output"
  PASS=$((PASS + 1))
else
  echo "FAIL: --seed 42 and --seed 99 produced identical output: $out42"
  FAIL=$((FAIL + 1))
fi

# Smoke 8: --json emits valid JSON with name and words fields (consistent for all --words values)
out=$("$VRK" moniker --json)
assert_contains "--json contains name field" '"name"' "$out"
assert_contains "--json contains words field" '"words"' "$out"

# Smoke 9: --json --count 3 emits 3 lines each parseable as JSON
json_out=$("$VRK" moniker --json --count 3)
json_line_count=$(echo "$json_out" | python3 -c '
import sys, json
lines = [l for l in sys.stdin if l.strip()]
for l in lines:
    json.loads(l)  # raises if invalid
print(len(lines))
')
assert_eq "--json --count 3 produces 3 JSON lines" "3" "$json_line_count"

# Smoke 10: --count 1000 produces exactly 1000 lines, all unique
unique_count=$("$VRK" moniker --count 1000 --seed 42 | sort -u | wc -l | tr -d ' ')
assert_eq "--count 1000 produces 1000 unique names" "1000" "$unique_count"

# Smoke 11: stdin is ignored — piping input does not affect output format
out=$(echo "this must be ignored" | "$VRK" moniker --seed 42)
assert_match "stdin ignored: output still matches adj-noun" '^[a-z]+-[a-z]+$' "$out"

# Smoke 12: --help exits 0
"$VRK" moniker --help >/dev/null
assert_exit "--help exits 0" 0 "$?"

# Smoke 13: unknown flag exits 2
"$VRK" moniker --unknown 2>/dev/null || exit_code=$?
assert_exit "unknown flag exits 2" 2 "${exit_code:-0}"
unset exit_code

# Smoke 14: --seed 0 is a valid seed (not treated as "no seed")
out_s0a=$("$VRK" moniker --seed 0)
out_s0b=$("$VRK" moniker --seed 0)
assert_eq "--seed 0 is deterministic (not random)" "$out_s0a" "$out_s0b"

# --- summary ---
echo ""
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
