#!/usr/bin/env bash
# testdata/pct/smoke.sh
#
# End-to-end smoke tests for vrk pct.
# Run after: make build
#
# Usage:
#   bash testdata/pct/smoke.sh
#   VRK=./vrk bash testdata/pct/smoke.sh

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
  if echo "$haystack" | grep -qF -- "$needle"; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (expected to contain: $needle)"
    echo "  got: $haystack"
    FAIL=$((FAIL + 1))
  fi
}

# --- Basic encode/decode ---

result=$(echo 'hello world & more' | "$VRK" pct --encode)
assert_eq "basic encode" "hello%20world%20%26%20more" "$result"

result=$(echo 'hello%20world%20%26%20more' | "$VRK" pct --decode)
assert_eq "basic decode" "hello world & more" "$result"

# --- Round-trip ---

result=$(echo 'hello world & more' | "$VRK" pct --encode | "$VRK" pct --decode)
assert_eq "round-trip encode|decode" "hello world & more" "$result"

# --- Form encoding ---

result=$(echo 'hello world' | "$VRK" pct --encode --form)
assert_eq "form encode spaces to +" "hello+world" "$result"

result=$(echo 'hello+world' | "$VRK" pct --decode --form)
assert_eq "form decode + to space" "hello world" "$result"

# --- Double-encode (documented gotcha) ---

result=$(echo 'hello%20world' | "$VRK" pct --encode)
assert_eq "double-encode % becomes %25" "hello%2520world" "$result"

# --- + literal in non-form decode ---

result=$(echo 'hello+world' | "$VRK" pct --decode)
assert_eq "+ is literal in non-form decode" "hello+world" "$result"

# --- %2B decodes to + ---

result=$(echo 'hello%2Bworld' | "$VRK" pct --decode)
assert_eq "%2B decodes to +" "hello+world" "$result"

# --- Positional argument ---

result=$("$VRK" pct --encode 'hello world')
assert_eq "positional arg encode" "hello%20world" "$result"

result=$("$VRK" pct --decode 'hello%20world')
assert_eq "positional arg decode" "hello world" "$result"

# --- Empty input ---

result=$(printf '' | "$VRK" pct --encode)
assert_eq "empty input produces no output" "" "$result"

"$VRK" pct --encode < /dev/null && code=0 || code=$?
assert_exit "empty input exits 0" 0 $code

# --- Multiline batch ---

result=$(printf 'a b\nc d\n' | "$VRK" pct --encode)
line1=$(echo "$result" | sed -n '1p')
line2=$(echo "$result" | sed -n '2p')
assert_eq "multiline encode line 1" "a%20b" "$line1"
assert_eq "multiline encode line 2" "c%20d" "$line2"

result=$(printf '%s\n' 'a%20b' 'c%20d' | "$VRK" pct --decode)
line1=$(echo "$result" | sed -n '1p')
line2=$(echo "$result" | sed -n '2p')
assert_eq "multiline decode line 1" "a b" "$line1"
assert_eq "multiline decode line 2" "c d" "$line2"

# --- JSON output shape ---

result=$(echo 'hello world' | "$VRK" pct --encode --json)
assert_contains "json encode has input field"  '"input":"hello world"'  "$result"
assert_contains "json encode has output field" '"output":"hello%20world"' "$result"
assert_contains "json encode op=encode"        '"op":"encode"'           "$result"
assert_contains "json encode mode=percent"     '"mode":"percent"'        "$result"

result=$(echo 'hello+world' | "$VRK" pct --decode --form --json)
assert_contains "json form decode mode=form" '"mode":"form"' "$result"
assert_contains "json form decode op=decode" '"op":"decode"' "$result"

# --- Multiline JSON (JSONL) ---

result=$(printf 'a b\nc d\n' | "$VRK" pct --encode --json)
line_count=$(echo "$result" | grep -c '"input"')
assert_eq "multiline --json emits one object per line" "2" "$line_count"

# --- Unicode round-trip ---

result=$(printf 'é 你好\n' | "$VRK" pct --encode | "$VRK" pct --decode)
assert_eq "unicode round-trip" "é 你好" "$result"

# --- Special chars round-trip ---

result=$(printf '%s\n' '& = ? # / + %' | "$VRK" pct --encode | "$VRK" pct --decode)
assert_eq "special chars round-trip" "& = ? # / + %" "$result"

# --- Usage errors (exit 2) ---

"$VRK" pct 2>/dev/null && code=0 || code=$?
assert_exit "no mode flag exits 2" 2 $code

"$VRK" pct --encode --decode 2>/dev/null && code=0 || code=$?
assert_exit "both --encode and --decode exits 2" 2 $code

"$VRK" pct --encode --bogus 2>/dev/null && code=0 || code=$?
assert_exit "unknown flag exits 2" 2 $code

# --- Runtime errors (exit 1) ---

echo '%ZZ' | "$VRK" pct --decode 2>/dev/null && code=0 || code=$?
assert_exit "invalid percent sequence exits 1" 1 $code

echo '%' | "$VRK" pct --decode 2>/dev/null && code=0 || code=$?
assert_exit "truncated percent sequence exits 1" 1 $code

# --- --help exits 0 ---

"$VRK" pct --help > /dev/null && code=0 || code=$?
assert_exit "--help exits 0" 0 $code

result=$("$VRK" pct --help)
assert_contains "--help mentions pct" "pct" "$result"
assert_contains "--help mentions --encode" "--encode" "$result"
assert_contains "--help mentions --decode" "--decode" "$result"
assert_contains "--help mentions --form" "--form" "$result"

# --- Stderr is empty on success ---

stderr=$(echo 'hello world' | "$VRK" pct --encode 2>&1 1>/dev/null)
assert_eq "no stderr on success" "" "$stderr"

# --- Summary ---

echo ""
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
