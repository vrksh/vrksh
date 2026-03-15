#!/usr/bin/env bash
# testdata/slug/smoke.sh
#
# End-to-end smoke tests for vrk slug.
# Run after: make build
#
# Usage:
#   bash testdata/slug/smoke.sh
#   VRK=./vrk bash testdata/slug/smoke.sh

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

# Smoke 1: basic slug
out=$(echo 'Hello World' | "$VRK" slug)
assert_eq "basic slug" "hello-world" "$out"

# Smoke 2: punctuation stripped
out=$(echo 'Hello, World!' | "$VRK" slug)
assert_eq "punctuation stripped" "hello-world" "$out"

# Smoke 3: parens stripped, digits kept
out=$(echo 'Hello World (2026)' | "$VRK" slug)
assert_eq "parens stripped, digits kept" "hello-world-2026" "$out"

# Smoke 4: unicode normalisation
out=$(echo 'Ünïcödé Héró' | "$VRK" slug)
assert_eq "unicode normalisation" "unicode-hero" "$out"

# Smoke 5: consecutive hyphens collapsed
out=$(echo 'hello--world' | "$VRK" slug)
assert_eq "consecutive hyphens collapsed" "hello-world" "$out"

# Smoke 6: leading/trailing spaces stripped
out=$(echo '  hello world  ' | "$VRK" slug)
assert_eq "leading/trailing spaces stripped" "hello-world" "$out"

# Smoke 7: --separator override
out=$(echo 'Hello World' | "$VRK" slug --separator _)
assert_eq "--separator _" "hello_world" "$out"

# Smoke 8: --max word boundary
out=$(echo 'A very long title' | "$VRK" slug --max 12)
assert_eq "--max word boundary" "a-very-long" "$out"

# Smoke 9: empty input exits 0 with no output
out=$(printf '' | "$VRK" slug)
exit_code=$?
assert_exit "empty input exits 0" 0 "$exit_code"
assert_eq "empty input produces no output" "" "$out"

# Smoke 10: all-punctuation input exits 0 with no output
out=$(echo '!!!' | "$VRK" slug)
exit_code=$?
assert_exit "all-punctuation exits 0" 0 "$exit_code"
assert_eq "all-punctuation produces no output" "" "$out"

# Smoke 11: multiline batch
out=$(printf 'Hello World\nFoo Bar\n' | "$VRK" slug)
line1=$(echo "$out" | sed -n '1p')
line2=$(echo "$out" | sed -n '2p')
assert_eq "multiline batch line 1" "hello-world" "$line1"
assert_eq "multiline batch line 2" "foo-bar" "$line2"

# Smoke 12: --json single line
out=$(echo 'Hello World' | "$VRK" slug --json)
assert_contains "--json contains input field" '"input":"Hello World"' "$out"
assert_contains "--json contains output field" '"output":"hello-world"' "$out"

# Smoke 13: --json multiline → two JSON objects
out=$(printf 'Hello World\nFoo Bar\n' | "$VRK" slug --json)
line_count=$(echo "$out" | grep -c '"input"' || true)
assert_eq "--json multiline: two JSON objects" "2" "$line_count"

# Smoke 14: positional arg form
out=$("$VRK" slug 'Hello World')
assert_eq "positional arg form" "hello-world" "$out"

# Smoke 15: unknown flag exits 2
"$VRK" slug --bogus </dev/null 2>/dev/null || exit_code=$?
assert_exit "unknown flag exits 2" 2 "${exit_code:-0}"
unset exit_code

# Smoke 16: --help exits 0
"$VRK" slug --help >/dev/null
assert_exit "--help exits 0" 0 "$?"

# Smoke 17: --max with no word boundary → empty output, exit 0
out=$(echo 'abcdefghij' | "$VRK" slug --max 3)
assert_exit "--max no boundary exits 0" 0 "$?"
assert_eq "--max no boundary: empty output" "" "$out"

# --- summary ---
echo ""
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
