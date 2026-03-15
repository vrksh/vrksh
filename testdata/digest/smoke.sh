#!/usr/bin/env bash
# testdata/digest/smoke.sh — end-to-end smoke tests for vrk digest.
# Run after: make build
# Usage: bash testdata/digest/smoke.sh
#        VRK=./vrk bash testdata/digest/smoke.sh

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

pass() { echo "PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    pass "$desc"
  else
    fail "$desc — expected: $expected | got: $actual"
  fi
}

assert_exit() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$actual" -eq "$expected" ]; then
    pass "$desc (exit $expected)"
  else
    fail "$desc — expected exit $expected, got $actual"
  fi
}

assert_contains() {
  local desc="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF -- "$needle"; then
    pass "$desc"
  else
    fail "$desc — expected to contain: $needle | got: $haystack"
  fi
}

# ---------------------------------------------------------------------------
# Section 1 — SHA-256 (default)
# ---------------------------------------------------------------------------
echo "--- Section 1: SHA-256 default ---"

out=$(echo 'hello' | $VRK digest)
assert_eq "sha256 of hello (with newline)" \
  "sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03" \
  "$out"

# ---------------------------------------------------------------------------
# Section 2 — algorithms
# ---------------------------------------------------------------------------
echo "--- Section 2: algorithms ---"

out=$(echo 'hello' | $VRK digest --algo md5)
assert_eq "md5 of hello (with newline)" "md5:b1946ac92492d2347c6235b4d2611184" "$out"

out=$(echo 'hello' | $VRK digest --algo sha512)
assert_contains "sha512 has correct prefix" "sha512:" "$out"
hexpart="${out#sha512:}"
assert_eq "sha512 hex is 128 chars" "128" "${#hexpart}"

ec=$(set +e; echo 'x' | $VRK digest --algo crc32 >/dev/null 2>&1; echo $?)
assert_exit "--algo unknown exits 2" 2 "$ec"

# ---------------------------------------------------------------------------
# Section 3 — --bare
# ---------------------------------------------------------------------------
echo "--- Section 3: --bare ---"

out=$(echo 'hello' | $VRK digest --bare)
assert_eq "--bare sha256 of hello (with newline)" \
  "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03" \
  "$out"
# No colon in bare output
if echo "$out" | grep -q ':'; then
  fail "--bare output must not contain colon"
else
  pass "--bare output has no colon"
fi

# ---------------------------------------------------------------------------
# Section 4 — --json
# ---------------------------------------------------------------------------
echo "--- Section 4: --json ---"

out=$(echo 'hello' | $VRK digest --json)
assert_contains "--json has input_bytes:6"  '"input_bytes":6'  "$out"
assert_contains "--json has algo sha256"    '"algo":"sha256"'  "$out"
assert_contains "--json has correct hash"  '"hash":"5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"' "$out"

# --json is valid JSON
echo "$out" | python3 -c 'import sys,json; json.load(sys.stdin)' 2>/dev/null \
  && pass "--json output is valid JSON" \
  || fail "--json output is not valid JSON"

# ---------------------------------------------------------------------------
# Section 5 — empty stdin
# ---------------------------------------------------------------------------
echo "--- Section 5: empty stdin ---"

out=$(printf '' | $VRK digest)
assert_eq "empty stdin (printf '')" \
  "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" \
  "$out"

out=$(echo '' | $VRK digest)
assert_eq "echo '' pipes \\n → sha256 of newline" \
  "sha256:01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b" \
  "$out"

# ---------------------------------------------------------------------------
# Section 6 — positional arg
# ---------------------------------------------------------------------------
echo "--- Section 6: positional arg ---"

# Positional arg hashes the literal string without a newline.
out=$($VRK digest 'hello')
assert_eq "positional arg sha256 of hello" \
  "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" \
  "$out"

# stdin hashes the literal pipe bytes — echo adds \n so the hash differs.
out=$(echo 'hello' | $VRK digest)
assert_eq "stdin sha256 of hello-newline" \
  "sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03" \
  "$out"

# printf omits the trailing newline → same hash as positional arg.
printf_out=$(printf 'hello' | $VRK digest)
assert_eq "printf stdin matches positional (no newline)" \
  "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" \
  "$printf_out"

# ---------------------------------------------------------------------------
# Section 7 — --file
# ---------------------------------------------------------------------------
echo "--- Section 7: --file ---"

f1="$TMPDIR/hello.txt"
printf 'hello' > "$f1"
out=$($VRK digest --file "$f1")
assert_eq "--file sha256 of hello" \
  "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" \
  "$out"

ec=$(set +e; $VRK digest --file "$TMPDIR/no-such-file" >/dev/null 2>&1; echo $?)
assert_exit "--file not found exits 1" 1 "$ec"

# stdin (printf, no newline) and --file must produce identical hashes.
stdin_hash=$(printf 'hello' | $VRK digest)
assert_eq "printf stdin matches --file (consistent streaming)" \
  "$($VRK digest --file "$f1")" \
  "$stdin_hash"

# Large-stream: 10 MB of zero bytes must hash without OOM.
large_hash=$(dd if=/dev/zero bs=1048576 count=10 2>/dev/null | $VRK digest)
assert_eq "large stream (10 MB zeros) sha256" \
  "sha256:e5b844cc57f57094ea4585e235f36c78c1cd222262bb89d53c94dcb4d6b3e55d" \
  "$large_hash"

# --file JSON shape
out=$($VRK digest --file "$f1" --json)
assert_contains "--file --json has file key"  '"file":'   "$out"
assert_contains "--file --json has algo key"  '"algo":'   "$out"
assert_contains "--file --json has hash key"  '"hash":'   "$out"

# ---------------------------------------------------------------------------
# Section 8 — --compare
# ---------------------------------------------------------------------------
echo "--- Section 8: --compare ---"

f2="$TMPDIR/hello2.txt"
f3="$TMPDIR/world.txt"
printf 'hello' > "$f2"
printf 'world' > "$f3"

out=$($VRK digest --file "$f1" --file "$f2" --compare)
assert_eq "--compare match" "match: true" "$out"
ec=$(set +e; $VRK digest --file "$f1" --file "$f2" --compare >/dev/null 2>&1; echo $?)
assert_exit "--compare match exits 0" 0 "$ec"

out=$($VRK digest --file "$f1" --file "$f3" --compare)
assert_eq "--compare mismatch" "match: false" "$out"
ec=$(set +e; $VRK digest --file "$f1" --file "$f3" --compare >/dev/null 2>&1; echo $?)
assert_exit "--compare mismatch exits 0" 0 "$ec"

# --compare JSON shape
out=$($VRK digest --file "$f1" --file "$f2" --compare --json)
assert_contains "--compare --json has match:true"  '"match":true'  "$out"
assert_contains "--compare --json has files array" '"files":'      "$out"
assert_contains "--compare --json has hashes array" '"hashes":'    "$out"
assert_contains "--compare --json has algo"        '"algo":'       "$out"
echo "$out" | python3 -c 'import sys,json; json.load(sys.stdin)' 2>/dev/null \
  && pass "--compare --json is valid JSON" \
  || fail "--compare --json is not valid JSON"

# --compare with only 1 file → exit 2
ec=$(set +e; $VRK digest --file "$f1" --compare >/dev/null 2>&1; echo $?)
assert_exit "--compare with 1 file exits 2" 2 "$ec"

# --compare with no files → exit 2
ec=$(set +e; echo 'x' | $VRK digest --compare >/dev/null 2>&1; echo $?)
assert_exit "--compare with no files exits 2" 2 "$ec"

# ---------------------------------------------------------------------------
# Section 9 — HMAC
# ---------------------------------------------------------------------------
echo "--- Section 9: HMAC ---"

out=$(echo 'payload' | $VRK digest --hmac --key mysecret)
assert_contains "HMAC has sha256: prefix" "sha256:" "$out"
hexpart="${out#sha256:}"
assert_eq "HMAC sha256 hex is 64 chars" "64" "${#hexpart}"

# HMAC --json shape
out=$(echo 'payload' | $VRK digest --hmac --key mysecret --json)
assert_contains "HMAC --json has hmac key" '"hmac":' "$out"
assert_contains "HMAC --json has algo"     '"algo":' "$out"
echo "$out" | python3 -c 'import sys,json; json.load(sys.stdin)' 2>/dev/null \
  && pass "HMAC --json is valid JSON" \
  || fail "HMAC --json is not valid JSON"

# --hmac without --key → exit 2
ec=$(set +e; echo 'x' | $VRK digest --hmac >/dev/null 2>&1; echo $?)
assert_exit "--hmac without --key exits 2" 2 "$ec"

# ---------------------------------------------------------------------------
# Section 10 — --verify
# ---------------------------------------------------------------------------
echo "--- Section 10: --verify ---"

# Round-trip: produce HMAC, verify it → exit 0
HMAC_HEX=$(echo 'payload' | $VRK digest --hmac --key mysecret --bare)
ec=$(set +e; echo 'payload' | $VRK digest --hmac --key mysecret --verify "$HMAC_HEX" >/dev/null 2>&1; echo $?)
assert_exit "--verify round-trip exits 0" 0 "$ec"

# Wrong HMAC → exit 1
ec=$(set +e; echo 'payload' | $VRK digest --hmac --key mysecret --verify "$(printf '0%.0s' {1..64})" >/dev/null 2>&1; echo $?)
assert_exit "--verify mismatch exits 1" 1 "$ec"

# Different key → mismatch, exit 1
ec=$(set +e; echo 'payload' | $VRK digest --hmac --key wrongkey --verify "$HMAC_HEX" >/dev/null 2>&1; echo $?)
assert_exit "--verify different key exits 1" 1 "$ec"

# --verify without --hmac → exit 2
ec=$(set +e; echo 'x' | $VRK digest --verify "$HMAC_HEX" >/dev/null 2>&1; echo $?)
assert_exit "--verify without --hmac exits 2" 2 "$ec"

# ---------------------------------------------------------------------------
# Section 11 — mutual exclusion and usage errors
# ---------------------------------------------------------------------------
echo "--- Section 11: usage errors ---"

ec=$(set +e; echo 'hello' | $VRK digest --bare --json >/dev/null 2>&1; echo $?)
assert_exit "--bare --json exits 2" 2 "$ec"

ec=$(set +e; $VRK digest --notaflag >/dev/null 2>&1; echo $?)
assert_exit "unknown flag exits 2" 2 "$ec"

# ---------------------------------------------------------------------------
# Section 12 — --help
# ---------------------------------------------------------------------------
echo "--- Section 12: --help ---"

out=$($VRK digest --help 2>&1 || true)
ec=$(set +e; $VRK digest --help >/dev/null 2>&1; echo $?)
assert_exit "--help exits 0" 0 "$ec"
assert_contains "--help contains 'digest'" "digest" "$out"
assert_contains "--help lists --algo"  "--algo"  "$out"
assert_contains "--help lists --bare"  "--bare"  "$out"
assert_contains "--help lists --hmac"  "--hmac"  "$out"
assert_contains "--help lists --verify" "--verify" "$out"

# ---------------------------------------------------------------------------
# Results
# ---------------------------------------------------------------------------
echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
