#!/usr/bin/env bash
# testdata/recase/smoke.sh
#
# End-to-end smoke tests for vrk recase.
# Run after: make build
#
# Usage:
#   bash testdata/recase/smoke.sh
#   VRK=./vrk bash testdata/recase/smoke.sh
#
# Exit 0 if all pass, exit 1 if any fail.

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

# ---------------------------------------------------------------------------
# Core convention conversions
# ---------------------------------------------------------------------------

got=$(echo 'hello_world' | $VRK recase --to camel)
assert_eq "snake → camel" "helloWorld" "$got"

got=$(echo 'hello_world' | $VRK recase --to pascal)
assert_eq "snake → pascal" "HelloWorld" "$got"

got=$(echo 'hello_world' | $VRK recase --to kebab)
assert_eq "snake → kebab" "hello-world" "$got"

got=$(echo 'hello_world' | $VRK recase --to screaming)
assert_eq "snake → screaming" "HELLO_WORLD" "$got"

got=$(echo 'hello_world' | $VRK recase --to title)
assert_eq "snake → title" "Hello World" "$got"

got=$(echo 'hello_world' | $VRK recase --to lower)
assert_eq "snake → lower" "hello world" "$got"

got=$(echo 'hello_world' | $VRK recase --to upper)
assert_eq "snake → upper" "HELLO WORLD" "$got"

got=$(echo 'helloWorld' | $VRK recase --to snake)
assert_eq "camel → snake" "hello_world" "$got"

got=$(echo 'helloWorld' | $VRK recase --to kebab)
assert_eq "camel → kebab" "hello-world" "$got"

got=$(echo 'HelloWorld' | $VRK recase --to snake)
assert_eq "pascal → snake" "hello_world" "$got"

got=$(echo 'hello-world' | $VRK recase --to camel)
assert_eq "kebab → camel" "helloWorld" "$got"

got=$(echo 'Hello World' | $VRK recase --to snake)
assert_eq "title → snake" "hello_world" "$got"

got=$(echo 'HELLO_WORLD' | $VRK recase --to camel)
assert_eq "screaming → camel" "helloWorld" "$got"

# ---------------------------------------------------------------------------
# Acronym handling
# ---------------------------------------------------------------------------

got=$(echo 'userID' | $VRK recase --to snake)
assert_eq "acronym: userID → snake" "user_id" "$got"

got=$(echo 'parseHTML' | $VRK recase --to snake)
assert_eq "acronym: parseHTML → snake" "parse_html" "$got"

# Documented limitation: two consecutive acronyms merge into one word.
got=$(echo 'getHTTPSURL' | $VRK recase --to snake)
assert_eq "consecutive acronyms (limitation): getHTTPSURL → snake" "get_httpsurl" "$got"

# ---------------------------------------------------------------------------
# Multiline batch mode
# ---------------------------------------------------------------------------

got=$(printf 'hello_world\nfoo_bar\n' | $VRK recase --to camel)
assert_eq "multiline batch" "$(printf 'helloWorld\nfooBar')" "$got"

# ---------------------------------------------------------------------------
# Edge cases
# ---------------------------------------------------------------------------

# Empty line is preserved — blank line in, blank line out.
line_count=$(printf '\n' | $VRK recase --to camel | wc -l | tr -d ' ')
assert_eq "empty line preserved (line count)" "1" "$line_count"

# Empty stdin: exit 0, no output.
out=$(printf '' | $VRK recase --to camel)
exit_code=0
printf '' | $VRK recase --to camel > /dev/null || exit_code=$?
assert_exit "empty stdin exit 0" "0" "$exit_code"
assert_eq "empty stdin no output" "" "$out"

# ---------------------------------------------------------------------------
# Error handling
# ---------------------------------------------------------------------------

# Missing --to → exit 2.
exit_code=0
echo 'x' | $VRK recase > /dev/null 2>&1 || exit_code=$?
assert_exit "missing --to → exit 2" "2" "$exit_code"

# Unknown --to value → exit 2.
exit_code=0
echo 'x' | $VRK recase --to bogus > /dev/null 2>&1 || exit_code=$?
assert_exit "unknown --to → exit 2" "2" "$exit_code"

# --quiet + missing --to → exit 2, stderr empty.
stderr_out=$(echo 'x' | $VRK recase --quiet 2>&1 >/dev/null || true)
exit_code=0
echo 'x' | $VRK recase --quiet > /dev/null 2>/dev/null || exit_code=$?
assert_exit "--quiet + missing --to → exit 2" "2" "$exit_code"
assert_eq "--quiet stderr empty" "" "$stderr_out"

# ---------------------------------------------------------------------------
# --json flag
# ---------------------------------------------------------------------------

got=$(echo 'hello_world' | $VRK recase --to camel --json)
assert_contains "--json has input field" '"input":"hello_world"' "$got"
assert_contains "--json has output field" '"output":"helloWorld"' "$got"
assert_contains "--json has from field" '"from":"snake"' "$got"
assert_contains "--json has to field" '"to":"camel"' "$got"

# --json multiline: two records, one per line.
line_count=$(printf 'hello_world\nfoo_bar\n' | $VRK recase --to camel --json | wc -l | tr -d ' ')
assert_eq "--json multiline: 2 records" "2" "$line_count"

# ---------------------------------------------------------------------------
# --help
# ---------------------------------------------------------------------------

got=$($VRK recase --help 2>/dev/null || true)
assert_contains "--help mentions recase" "recase" "$got"
assert_contains "--help mentions --to" "target naming convention" "$got"

# ---------------------------------------------------------------------------
# Results
# ---------------------------------------------------------------------------
echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
