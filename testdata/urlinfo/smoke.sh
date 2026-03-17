#!/usr/bin/env bash
# testdata/urlinfo/smoke.sh
#
# End-to-end smoke tests for vrk urlinfo.
# Run against the real binary — no mocks, no stubs.
#
# Usage:
#   make smoke                                           # via Makefile
#   make build && bash testdata/urlinfo/smoke.sh         # direct
#   VRK=./vrk bash testdata/urlinfo/smoke.sh

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0

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
    echo "FAIL: $desc (expected output to contain: $needle)"
    echo "  got: $haystack"
    FAIL=$((FAIL + 1))
  fi
}

assert_not_contains() {
  local desc="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    echo "FAIL: $desc (expected output NOT to contain: $needle)"
    echo "  got: $haystack"
    FAIL=$((FAIL + 1))
  else
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  fi
}

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

assert_valid_json() {
  local desc="$1" actual="$2"
  if echo "$actual" | python3 -c 'import sys,json; json.load(sys.stdin)' 2>/dev/null; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (not valid JSON)"
    echo "  got: $actual"
    FAIL=$((FAIL + 1))
  fi
}

assert_line_count() {
  local desc="$1" expected="$2" actual="$3"
  local count
  count=$(echo "$actual" | grep -c .)
  if [ "$count" -eq "$expected" ]; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (expected $expected lines, got $count)"
    echo "  got: $actual"
    FAIL=$((FAIL + 1))
  fi
}

# ---------------------------------------------------------------------------
# Basic parse — all fields present
# ---------------------------------------------------------------------------

out=$($VRK urlinfo 'https://api.example.com/v1/users?page=2&limit=10')
e=$?
assert_exit "basic parse exits 0" 0 "$e"
assert_contains "basic parse: scheme=https" '"scheme":"https"' "$out"
assert_contains "basic parse: host=api.example.com" '"host":"api.example.com"' "$out"
assert_contains "basic parse: path=/v1/users" '"path":"/v1/users"' "$out"
assert_contains "basic parse: port=0" '"port":0' "$out"
assert_contains "basic parse: query.page=2" '"page":"2"' "$out"
assert_contains "basic parse: query.limit=10" '"limit":"10"' "$out"
assert_valid_json "basic parse output is valid JSON" "$out"

# ---------------------------------------------------------------------------
# Credentials — password omitted, user and port present
# ---------------------------------------------------------------------------

creds_out=$($VRK urlinfo 'https://user:pass@host:8080/path#anchor')
assert_exit "credentials exits 0" 0 "$?"
assert_contains "credentials: user=user" '"user":"user"' "$creds_out"
assert_contains "credentials: port=8080" '"port":8080' "$creds_out"
assert_contains "credentials: fragment=anchor" '"fragment":"anchor"' "$creds_out"
assert_not_contains "credentials: password not in output" '"pass"' "$creds_out"
assert_not_contains "credentials: password field absent" '"password"' "$creds_out"

# ---------------------------------------------------------------------------
# --field extractions
# ---------------------------------------------------------------------------

f=$($VRK urlinfo --field host 'https://api.example.com/path')
assert_eq "--field host" "api.example.com" "$f"

f=$($VRK urlinfo --field port 'https://example.com')
assert_eq "--field port absent → empty" "" "$f"

f=$($VRK urlinfo --field port 'https://example.com:8080')
assert_eq "--field port present" "8080" "$f"

f=$($VRK urlinfo --field 'query.page' 'https://example.com?page=2')
assert_eq "--field query.page" "2" "$f"

f_exit=0
f=$($VRK urlinfo --field 'query.missing' 'https://example.com') || f_exit=$?
assert_exit "--field query.missing exits 0" 0 "$f_exit"
assert_eq "--field query.missing → empty" "" "$f"

f=$($VRK urlinfo --field scheme 'https://example.com')
assert_eq "--field scheme" "https" "$f"

f=$($VRK urlinfo --field path 'https://example.com/a/b')
assert_eq "--field path" "/a/b" "$f"

f=$($VRK urlinfo --field fragment 'https://example.com#sec')
assert_eq "--field fragment" "sec" "$f"

f=$($VRK urlinfo --field user 'https://alice@example.com')
assert_eq "--field user" "alice" "$f"

# ---------------------------------------------------------------------------
# Stdin equivalence — positional arg and piped stdin produce identical output
# ---------------------------------------------------------------------------

out_arg=$($VRK urlinfo 'https://example.com/path?x=1')
out_stdin=$(echo 'https://example.com/path?x=1' | $VRK urlinfo)
assert_eq "stdin equivalence" "$out_arg" "$out_stdin"

# ---------------------------------------------------------------------------
# Multiline batch — one JSON record per line
# ---------------------------------------------------------------------------

batch_out=$(printf 'https://example.com\nhttps://api.example.com\n' | $VRK urlinfo)
assert_exit "batch exits 0" 0 "$?"
assert_line_count "batch produces 2 lines" 2 "$batch_out"
assert_contains "batch line 1: host=example.com" '"host":"example.com"' "$(echo "$batch_out" | head -1)"
assert_contains "batch line 2: host=api.example.com" '"host":"api.example.com"' "$(echo "$batch_out" | tail -1)"

# ---------------------------------------------------------------------------
# Invalid URL — both scheme and host empty → exit 1
# ---------------------------------------------------------------------------

inv_exit=0
$VRK urlinfo 'not a url' 2>/dev/null || inv_exit=$?
assert_exit "invalid URL exits 1" 1 "$inv_exit"

# stderr contains "invalid URL"
inv_stderr=$($VRK urlinfo 'not a url' 2>&1 >/dev/null || true)
assert_contains "invalid URL: stderr message" "invalid URL" "$inv_stderr"

# ---------------------------------------------------------------------------
# Scheme-relative URL — host non-empty → exit 0
# ---------------------------------------------------------------------------

sr_out=$($VRK urlinfo '//example.com/path')
assert_exit "scheme-relative exits 0" 0 "$?"
assert_contains "scheme-relative: host=example.com" '"host":"example.com"' "$sr_out"

# ---------------------------------------------------------------------------
# Empty stdin — exit 0, no output
# ---------------------------------------------------------------------------

empty_out=$(printf '' | $VRK urlinfo)
assert_exit "empty stdin exits 0" 0 "$?"
assert_eq "empty stdin: no output" "" "$empty_out"

# ---------------------------------------------------------------------------
# --help — exit 0, stdout contains "urlinfo"
# ---------------------------------------------------------------------------

help_out=$($VRK urlinfo --help)
assert_exit "--help exits 0" 0 "$?"
assert_contains "--help contains 'urlinfo'" "urlinfo" "$help_out"

# ---------------------------------------------------------------------------
# --json metadata trailer
# ---------------------------------------------------------------------------

json_out=$(echo 'https://example.com' | $VRK urlinfo --json)
assert_exit "--json exits 0" 0 "$?"
assert_line_count "--json single URL: 2 lines (record + trailer)" 2 "$json_out"
trailer=$(echo "$json_out" | tail -1)
assert_contains "--json trailer: _vrk=urlinfo" '"_vrk":"urlinfo"' "$trailer"
assert_contains "--json trailer: count=1" '"count":1' "$trailer"

# ---------------------------------------------------------------------------
# --json batch: trailer count matches record count
# ---------------------------------------------------------------------------

batch_json=$(printf 'https://a.com\nhttps://b.com\n' | $VRK urlinfo --json)
assert_exit "--json batch exits 0" 0 "$?"
assert_line_count "--json batch: 3 lines (2 records + trailer)" 3 "$batch_json"
batch_trailer=$(echo "$batch_json" | tail -1)
assert_contains "--json batch trailer: count=2" '"count":2' "$batch_trailer"

# ---------------------------------------------------------------------------
# Unknown flag — exit 2
# ---------------------------------------------------------------------------

unk_exit=0
$VRK urlinfo --bogus 2>/dev/null || unk_exit=$?
assert_exit "unknown flag exits 2" 2 "$unk_exit"

# ---------------------------------------------------------------------------
# Output shape stable — all fields present including zero values
# ---------------------------------------------------------------------------

shape_out=$($VRK urlinfo 'https://example.com/path')
for field in scheme host port path query fragment user; do
  assert_contains "stable shape: field $field present" "\"$field\"" "$shape_out"
done

# ---------------------------------------------------------------------------
# Results
# ---------------------------------------------------------------------------

echo ""
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
