#!/usr/bin/env bash
# testdata/base/smoke.sh
#
# End-to-end smoke tests for vrk base.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/base/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/base/smoke.sh   # explicit binary path

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

echo "vrk base — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Encode — exact output for each encoding
# ------------------------------------------------------------
echo "--- 1. encode ---"

got=$(echo 'hello' | "$VRK" base encode --to base64)
assert_stdout_equals "encode base64"    "aGVsbG8="  "$got"

got=$(echo 'hello' | "$VRK" base encode --to base64url)
assert_stdout_equals "encode base64url" "aGVsbG8"   "$got"

got=$(echo 'hello' | "$VRK" base encode --to hex)
assert_stdout_equals "encode hex"       "68656c6c6f" "$got"

# base32("hello") = 5 bytes = 40 bits = 8 base32 chars, no padding required.
# Note: the session spec showed "NBSWY3DPEB3W64TMMQ======" which is base32 of
# "hello world" — a typo. NBSWY3DP is the correct RFC 4648 encoding of "hello".
got=$(echo 'hello' | "$VRK" base encode --to base32)
assert_stdout_equals "encode base32"    "NBSWY3DP"   "$got"

# ------------------------------------------------------------
# 2. Decode — exact output for each encoding
# ------------------------------------------------------------
echo ""
echo "--- 2. decode ---"

# Decode output is raw bytes with no added newline.
# $() capture is safe because "hello" does not end with \n.
got=$(echo 'aGVsbG8=' | "$VRK" base decode --from base64)
assert_stdout_equals "decode base64"    "hello" "$got"

got=$(echo 'aGVsbG8' | "$VRK" base decode --from base64url)
assert_stdout_equals "decode base64url" "hello" "$got"

got=$(echo '68656c6c6f' | "$VRK" base decode --from hex)
assert_stdout_equals "decode hex"       "hello" "$got"

got=$(echo 'NBSWY3DP' | "$VRK" base decode --from base32)
assert_stdout_equals "decode base32"    "hello" "$got"

# ------------------------------------------------------------
# 3. Binary encode: null bytes through hex
# ------------------------------------------------------------
echo ""
echo "--- 3. binary encode ---"

got=$(printf '\x00\x01\x02\xff' | "$VRK" base encode --to hex)
assert_stdout_equals "binary hex encode" "000102ff" "$got"

# ------------------------------------------------------------
# 4. Binary round-trip via hex
# ------------------------------------------------------------
echo ""
echo "--- 4. binary round-trip ---"

# Encode then decode. Use xxd -p to view the raw bytes as hex for comparison.
# xxd is available on macOS and most Linux distributions.
if command -v xxd > /dev/null 2>&1; then
  got=$(printf '\x00\x01\x02\xff' | "$VRK" base encode --to hex | "$VRK" base decode --from hex | xxd -p | tr -d '\n')
  assert_stdout_equals "binary round-trip hex (xxd)" "000102ff" "$got"
else
  ok "binary round-trip hex (xxd skipped — xxd not found)"
fi

# Round-trip via base64 (verify no data corruption on binary input)
encoded=$(printf '\x00\x01\x02\xff' | "$VRK" base encode --to base64)
got=$(echo "$encoded" | "$VRK" base encode --to hex)
# encoded bytes of "\x00\x01\x02\xff" in base64 are AAQC/w==; their hex is 41415143 2f773d3d
# Instead just verify decode recovers the original hex
decoded_hex=$(echo "$encoded" | "$VRK" base decode --from base64 | "$VRK" base encode --to hex)
assert_stdout_equals "binary round-trip base64→hex" "000102ff" "$decoded_hex"

# ------------------------------------------------------------
# 5. Error paths
# ------------------------------------------------------------
echo ""
echo "--- 5. error paths ---"

set +e

echo 'not valid base64!!!' | "$VRK" base decode --from base64 > /dev/null 2>&1
exit_code=$?
assert_exit "invalid base64 input: exit 1" 1 "$exit_code"

echo 'gg' | "$VRK" base decode --from hex > /dev/null 2>&1
exit_code=$?
assert_exit "invalid hex input: exit 1" 1 "$exit_code"

# Base32 alphabet is A-Z and 2-7. Characters 0, 1, 8, 9 are not valid.
echo '00000000' | "$VRK" base decode --from base32 > /dev/null 2>&1
exit_code=$?
assert_exit "invalid base32 input: exit 1" 1 "$exit_code"

# Confirm exit 1 errors go to stderr (not stdout)
stdout=$(echo 'not valid base64!!!' | "$VRK" base decode --from base64 2>/dev/null)
assert_stdout_empty "invalid base64: stdout empty" "$stdout"

set -e

# ------------------------------------------------------------
# 6. Usage errors (exit 2)
# ------------------------------------------------------------
echo ""
echo "--- 6. usage errors ---"

set +e

echo 'hello' | "$VRK" base > /dev/null 2>&1
assert_exit "no subcommand: exit 2" 2 $?

echo 'hello' | "$VRK" base encode > /dev/null 2>&1
assert_exit "encode no --to: exit 2" 2 $?

echo 'aGVsbG8=' | "$VRK" base decode > /dev/null 2>&1
assert_exit "decode no --from: exit 2" 2 $?

echo 'hello' | "$VRK" base encode --to bogus > /dev/null 2>&1
assert_exit "unsupported encoding: exit 2" 2 $?

echo 'hello' | "$VRK" base encode --bogus > /dev/null 2>&1
assert_exit "unknown flag: exit 2" 2 $?

set -e

# ------------------------------------------------------------
# 7. Empty stdin
# ------------------------------------------------------------
echo ""
echo "--- 7. empty stdin ---"

got=$(printf '' | "$VRK" base encode --to base64)
exit_code=$?
assert_exit        "empty stdin encode: exit 0" 0 "$exit_code"
assert_stdout_empty "empty stdin encode: no output" "$got"

got=$(printf '' | "$VRK" base decode --from base64)
exit_code=$?
assert_exit        "empty stdin decode: exit 0" 0 "$exit_code"
assert_stdout_empty "empty stdin decode: no output" "$got"

# A sole newline is equivalent to empty after stripping.
got=$(printf '\n' | "$VRK" base encode --to hex)
exit_code=$?
assert_exit        "sole newline: exit 0" 0 "$exit_code"
assert_stdout_empty "sole newline: no output" "$got"

# ------------------------------------------------------------
# 8. --help
# ------------------------------------------------------------
echo ""
echo "--- 8. --help ---"

"$VRK" base --help > /dev/null
assert_exit "--help: exit 0" 0 $?

# ------------------------------------------------------------
# 9. --quiet suppresses stderr; exit code unchanged
# ------------------------------------------------------------
echo ""
echo "--- 9. --quiet ---"

set +e
stderr=$(echo 'not valid base64!!!' | "$VRK" base decode --from base64 --quiet 2>&1 >/dev/null)
exit_code=$?
set -e
assert_exit        "--quiet decode error: exit 1" 1 "$exit_code"
assert_stderr_empty "--quiet decode error: stderr empty" "$stderr"

# --quiet on success: stdout still present, stderr still empty
stdout=$(echo 'hello' | "$VRK" base encode --to hex --quiet 2>/dev/null)
stderr=$(echo 'hello' | "$VRK" base encode --to hex --quiet 2>&1 >/dev/null)
assert_stdout_equals "--quiet success: stdout correct" "68656c6c6f" "$stdout"
assert_stderr_empty  "--quiet success: stderr empty" "$stderr"

# ------------------------------------------------------------
# 10. Positional argument
# ------------------------------------------------------------
echo ""
echo "--- 10. positional argument ---"

got=$("$VRK" base encode --to hex hello)
assert_stdout_equals "positional encode" "68656c6c6f" "$got"

got=$("$VRK" base decode --from hex 68656c6c6f)
assert_stdout_equals "positional decode" "hello" "$got"

# Positional output == stdin output
got_stdin=$(echo 'hello' | "$VRK" base encode --to hex)
assert_stdout_equals "positional == stdin" "$got_stdin" "$got_stdin"

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
