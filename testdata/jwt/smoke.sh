#!/usr/bin/env bash
# testdata/jwt/smoke.sh
#
# Manual smoke tests for vrk jwt.
# Run after make build to verify end-to-end behaviour.
#
# Usage:
#   ./testdata/jwt/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/jwt/smoke.sh   # explicit binary path
#
# Exit 0 if all pass. Exit 1 on first failure.

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0

# Token constants
# valid:   exp=2524608000 (2050-01-01), sub=1234567890, name=John Doe, admin=true
# expired: exp=1772983763 (2026-03-08 15:29 UTC — already past), same other claims
VALID_JWT="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImV4cCI6MjUyNDYwODAwMCwiaWF0IjoxNTE2MjM5MDIyfQ.1n2qLms2Fy9TOojNHoEplIoS0Oyu4PKT3wYwRv5_0Ok"
EXPIRED_JWT="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImV4cCI6MTc3Mjk4Mzc2MywiaWF0IjoxNTE2MjM5MDIyfQ.Ox-nWmGb-ehO0U38wefNLdP18uC6-HjGum6pcNXVVM4"

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

# assert_exit <description> <expected_exit> <actual_exit>
assert_exit() {
  local desc=$1 expected=$2 actual=$3
  if [ "$actual" -eq "$expected" ]; then
    ok "$desc (exit $expected)"
  else
    fail "$desc" "expected exit $expected, got exit $actual"
  fi
}

# assert_stdout_contains <description> <pattern> <stdout>
assert_stdout_contains() {
  local desc=$1 pattern=$2 stdout=$3
  if echo "$stdout" | grep -q "$pattern"; then
    ok "$desc (stdout contains '$pattern')"
  else
    fail "$desc" "stdout did not contain '$pattern'. got: $stdout"
  fi
}

# assert_stdout_empty <description> <stdout>
assert_stdout_empty() {
  local desc=$1 stdout=$2
  if [ -z "$stdout" ]; then
    ok "$desc (stdout empty)"
  else
    fail "$desc" "expected empty stdout, got: $stdout"
  fi
}

# assert_stderr_contains <description> <pattern> <stderr>
assert_stderr_contains() {
  local desc=$1 pattern=$2 stderr=$3
  if echo "$stderr" | grep -q "$pattern"; then
    ok "$desc (stderr contains '$pattern')"
  else
    fail "$desc" "stderr did not contain '$pattern'. got: $stderr"
  fi
}

# assert_stdout_equals <description> <expected> <actual>
assert_stdout_equals() {
  local desc=$1 expected=$2 actual=$3
  if [ "$actual" = "$expected" ]; then
    ok "$desc (stdout = '$expected')"
  else
    fail "$desc" "expected '$expected', got '$actual'"
  fi
}

echo "vrk jwt — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Default output — positional arg
# ------------------------------------------------------------
echo "--- default output ---"

stdout=$("$VRK" jwt "$VALID_JWT" 2>/dev/null) || true
exit_code=$("$VRK" jwt "$VALID_JWT" > /dev/null 2>&1; echo $?) || true
exit_code=$(set +e; "$VRK" jwt "$VALID_JWT" > /dev/null 2>&1; echo $?)

assert_stdout_contains "default: contains sub"    '"sub"'       "$stdout"
assert_stdout_contains "default: contains name"   '"name"'      "$stdout"
assert_stdout_contains "default: contains admin"  '"admin"'     "$stdout"
assert_stdout_contains "default: contains exp"    '"exp"'       "$stdout"
assert_stdout_contains "default: sub value"       '1234567890'  "$stdout"
assert_stdout_contains "default: name value"      'John Doe'    "$stdout"

exit_code=$(set +e; "$VRK" jwt "$VALID_JWT" > /dev/null 2>&1; echo $?)
assert_exit "default: exit 0" 0 "$exit_code"

# ------------------------------------------------------------
# 2. Default output — stdin
# ------------------------------------------------------------
echo ""
echo "--- stdin ---"

stdout=$(echo "$VALID_JWT" | "$VRK" jwt 2>/dev/null) || true
exit_code=$(set +e; echo "$VALID_JWT" | "$VRK" jwt > /dev/null 2>&1; echo $?)

assert_stdout_contains "stdin: contains sub"   '"sub"'      "$stdout"
assert_stdout_contains "stdin: contains exp"   '"exp"'      "$stdout"
assert_exit            "stdin: exit 0"         0            "$exit_code"

# ------------------------------------------------------------
# 3. --claim
# ------------------------------------------------------------
echo ""
echo "--- --claim ---"

stdout=$("$VRK" jwt --claim sub "$VALID_JWT" 2>/dev/null) || true
assert_stdout_equals "claim sub: value" "1234567890" "$stdout"
exit_code=$(set +e; "$VRK" jwt --claim sub "$VALID_JWT" > /dev/null 2>&1; echo $?)
assert_exit "claim sub: exit 0" 0 "$exit_code"

stdout=$("$VRK" jwt --claim name "$VALID_JWT" 2>/dev/null) || true
assert_stdout_equals "claim name: value" "John Doe" "$stdout"

stdout=$("$VRK" jwt --claim admin "$VALID_JWT" 2>/dev/null) || true
assert_stdout_equals "claim admin: value" "true" "$stdout"

stdout=$("$VRK" jwt --claim exp "$VALID_JWT" 2>/dev/null) || true
assert_stdout_equals "claim exp: value" "2524608000" "$stdout"

# missing claim
stdout=$("$VRK" jwt --claim does_not_exist "$VALID_JWT" 2>/dev/null) || true
stderr=$(set +e; "$VRK" jwt --claim does_not_exist "$VALID_JWT" 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" jwt --claim does_not_exist "$VALID_JWT" > /dev/null 2>&1; echo $?)
assert_exit            "claim missing: exit 1"          1         "$exit_code"
assert_stdout_empty    "claim missing: stdout empty"               "$stdout"
assert_stderr_contains "claim missing: stderr not found" "not found" "$stderr"

# ------------------------------------------------------------
# 4. --json
# ------------------------------------------------------------
echo ""
echo "--- --json ---"

stdout=$("$VRK" jwt --json "$VALID_JWT" 2>/dev/null) || true
exit_code=$(set +e; "$VRK" jwt --json "$VALID_JWT" > /dev/null 2>&1; echo $?)
assert_exit            "--json valid: exit 0"              0            "$exit_code"
assert_stdout_contains "--json valid: has header"          '"header"'   "$stdout"
assert_stdout_contains "--json valid: has payload"         '"payload"'  "$stdout"
assert_stdout_contains "--json valid: has expires_in"      '"expires_in"' "$stdout"
assert_stdout_contains "--json valid: alg in header"       '"alg"'      "$stdout"
assert_stdout_contains "--json valid: expires_in not expired" "20"      "$stdout"  # 2050 = "~24 years..."

# --json on expired token: exit 0, expires_in = "expired"
stdout=$("$VRK" jwt --json "$EXPIRED_JWT" 2>/dev/null) || true
exit_code=$(set +e; "$VRK" jwt --json "$EXPIRED_JWT" > /dev/null 2>&1; echo $?)
assert_exit            "--json expired: exit 0"               0          "$exit_code"
assert_stdout_contains "--json expired: expires_in=expired"  'expired'   "$stdout"

# ------------------------------------------------------------
# 5. --expired
# ------------------------------------------------------------
echo ""
echo "--- --expired ---"

# valid token + --expired: exit 0
exit_code=$(set +e; "$VRK" jwt --expired "$VALID_JWT" > /dev/null 2>&1; echo $?)
assert_exit "--expired valid token: exit 0" 0 "$exit_code"

# valid token + --expired: stdout has payload
stdout=$("$VRK" jwt --expired "$VALID_JWT" 2>/dev/null) || true
assert_stdout_contains "--expired valid: stdout has payload" '"sub"' "$stdout"

# expired token + --expired: exit 1
stdout=$("$VRK" jwt --expired "$EXPIRED_JWT" 2>/dev/null) || true
stderr=$(set +e; "$VRK" jwt --expired "$EXPIRED_JWT" 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" jwt --expired "$EXPIRED_JWT" > /dev/null 2>&1; echo $?)
assert_exit            "--expired expired token: exit 1"              1               "$exit_code"
assert_stdout_empty    "--expired expired token: stdout empty"                        "$stdout"
assert_stderr_contains "--expired expired token: stderr token expired" "token expired" "$stderr"

# expired token, no flags: exit 0 (default does not check expiry)
exit_code=$(set +e; "$VRK" jwt "$EXPIRED_JWT" > /dev/null 2>&1; echo $?)
assert_exit "expired token no flags: exit 0" 0 "$exit_code"

stdout=$("$VRK" jwt "$EXPIRED_JWT" 2>/dev/null) || true
assert_stdout_contains "expired token no flags: stdout has payload" '"sub"' "$stdout"

# ------------------------------------------------------------
# 6. Invalid tokens
# ------------------------------------------------------------
echo ""
echo "--- invalid tokens ---"

# not a JWT
stderr=$(set +e; "$VRK" jwt "notajwt" 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" jwt "notajwt" > /dev/null 2>&1; echo $?)
assert_exit            "invalid notajwt: exit 1"                1             "$exit_code"
assert_stderr_contains "invalid notajwt: stderr invalid JWT"    "invalid JWT"  "$stderr"

# two parts only
stderr=$(set +e; "$VRK" jwt "header.payload" 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" jwt "header.payload" > /dev/null 2>&1; echo $?)
assert_exit            "invalid two parts: exit 1"              1             "$exit_code"
assert_stderr_contains "invalid two parts: stderr invalid JWT"  "invalid JWT"  "$stderr"

# bad base64 in payload
stderr=$(set +e; "$VRK" jwt "eyJhbGciOiJIUzI1NiJ9.!!!invalid!!!.sig" 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" jwt "eyJhbGciOiJIUzI1NiJ9.!!!invalid!!!.sig" > /dev/null 2>&1; echo $?)
assert_exit            "invalid bad base64: exit 1"             1             "$exit_code"
assert_stderr_contains "invalid bad base64: stderr invalid JWT" "invalid JWT"  "$stderr"

# valid base64 but not JSON in payload
# "aGVsbG8=" = base64("hello") — valid base64, not JSON
stderr=$(set +e; "$VRK" jwt "eyJhbGciOiJIUzI1NiJ9.aGVsbG8.sig" 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" jwt "eyJhbGciOiJIUzI1NiJ9.aGVsbG8.sig" > /dev/null 2>&1; echo $?)
assert_exit            "invalid not JSON: exit 1"               1             "$exit_code"
assert_stderr_contains "invalid not JSON: stderr invalid JWT"   "invalid JWT"  "$stderr"

# empty string
stderr=$(set +e; "$VRK" jwt "" 2>&1 >/dev/null; true)
exit_code=$(set +e; "$VRK" jwt "" > /dev/null 2>&1; echo $?)
assert_exit            "empty string: exit 1 or 2"  1 "$exit_code" 2>/dev/null || \
assert_exit            "empty string: exit 1 or 2"  2 "$exit_code" 2>/dev/null || true

# ------------------------------------------------------------
# 7. Usage errors
# ------------------------------------------------------------
echo ""
echo "--- usage errors ---"

# no input (simulate no stdin in non-TTY by closing stdin)
exit_code=$(set +e; "$VRK" jwt < /dev/null > /dev/null 2>&1; echo $?)
assert_exit "no input: exit 2" 2 "$exit_code"

stderr=$(set +e; "$VRK" jwt < /dev/null 2>&1 >/dev/null; true)
assert_stderr_contains "no input: stderr usage hint" "usage" "$stderr"

# unknown flag
exit_code=$(set +e; "$VRK" jwt --unknown-flag "$VALID_JWT" > /dev/null 2>&1; echo $?)
assert_exit "unknown flag: exit 2" 2 "$exit_code"

# ------------------------------------------------------------
# 8. --help
# ------------------------------------------------------------
echo ""
echo "--- --help ---"

stdout=$("$VRK" jwt --help 2>/dev/null) || true
exit_code=$(set +e; "$VRK" jwt --help > /dev/null 2>&1; echo $?)
assert_exit            "--help: exit 0"            0       "$exit_code"
assert_stdout_contains "--help: contains usage"    "usage"  "$stdout"
assert_stdout_contains "--help: mentions --claim"  "claim"  "$stdout"
assert_stdout_contains "--help: mentions --expired" "expired" "$stdout"
assert_stdout_contains "--help: mentions --json"   "json"   "$stdout"

# ------------------------------------------------------------
# 9. Pipeline usage
# ------------------------------------------------------------
echo ""
echo "--- pipeline ---"

# vrk jwt | jq
if command -v jq > /dev/null 2>&1; then
  sub=$(echo "$VALID_JWT" | "$VRK" jwt | jq -r '.sub' 2>/dev/null) || true
  assert_stdout_equals "pipeline: jwt | jq .sub" "1234567890" "$sub"
else
  echo "  SKIP  pipeline jq test (jq not installed)"
fi

# claim extraction in pipeline
sub=$("$VRK" jwt --claim sub "$VALID_JWT" | tr -d '\n') || true
assert_stdout_equals "pipeline: --claim sub piped" "1234567890" "$sub"

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
