#!/usr/bin/env bash
# testdata/kv/smoke.sh
#
# End-to-end smoke tests for vrk kv.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/kv/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/kv/smoke.sh   # explicit binary path
#
# Exit 0 if all pass. Exit 1 on first failure.

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0

# Isolated database — every run starts clean, cleaned up on exit.
DB=$(mktemp /tmp/vrk-kv-smoke-XXXXXX.db)
export VRK_KV_PATH="$DB"
trap 'rm -f "$DB"' EXIT

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

assert_stdout_contains() {
  local desc=$1 pattern=$2 actual=$3
  if echo "$actual" | grep -q "$pattern"; then
    ok "$desc (stdout contains '$pattern')"
  else
    fail "$desc" "stdout did not contain '$pattern'. got: $actual"
  fi
}

assert_line_count() {
  local desc=$1 expected=$2 actual=$3
  local count
  if [ -z "$actual" ]; then
    count=0
  else
    count=$(echo "$actual" | wc -l | tr -d ' ')
  fi
  if [ "$count" -eq "$expected" ]; then
    ok "$desc ($expected lines)"
  else
    fail "$desc" "expected $expected lines, got $count. stdout: $actual"
  fi
}

echo "vrk kv — smoke tests"
echo "binary: $VRK"
echo "db:     $DB"
echo ""

# ------------------------------------------------------------
# 1. Basic set / get / del
# ------------------------------------------------------------
echo "--- basic set/get/del ---"

stdout=$("$VRK" kv set mykey myvalue 2>/dev/null); exit_code=$?
assert_exit        "set: exit 0"          0  "$exit_code"
assert_stdout_empty "set: stdout empty"     "$stdout"
stderr=$(set +e; "$VRK" kv set mykey myvalue 2>&1 >/dev/null; true)
assert_stderr_empty "set: stderr empty"     "$stderr"

stdout=$("$VRK" kv get mykey 2>/dev/null); exit_code=$?
assert_exit        "get: exit 0"          0  "$exit_code"
assert_stdout_equals "get: value"  "myvalue" "$stdout"

stdout=$(set +e; "$VRK" kv get nonexistent 2>/dev/null; true); exit_code=$(set +e; "$VRK" kv get nonexistent >/dev/null 2>&1; echo $?)
assert_exit          "get missing: exit 1"   1  "$exit_code"
assert_stdout_empty  "get missing: stdout empty" "$stdout"
stderr=$(set +e; "$VRK" kv get nonexistent 2>&1 >/dev/null; true)
assert_stdout_contains "get missing: stderr key not found" "key not found" "$stderr"

"$VRK" kv del mykey >/dev/null 2>&1
exit_code=$(set +e; "$VRK" kv get mykey >/dev/null 2>&1; echo $?)
assert_exit "del then get: exit 1" 1 "$exit_code"

# del on missing key — must be silent exit 0
exit_code=$(set +e; "$VRK" kv del nonexistent >/dev/null 2>&1; echo $?)
assert_exit "del missing: exit 0" 0 "$exit_code"

# ------------------------------------------------------------
# 2. Overwrite
# ------------------------------------------------------------
echo ""
echo "--- overwrite ---"

"$VRK" kv set overkey firstvalue >/dev/null 2>&1
"$VRK" kv set overkey secondvalue >/dev/null 2>&1
stdout=$("$VRK" kv get overkey 2>/dev/null)
assert_stdout_equals "overwrite: new value" "secondvalue" "$stdout"

# ------------------------------------------------------------
# 3. list (sorted)
# ------------------------------------------------------------
echo ""
echo "--- list ---"

# Use a separate namespace to avoid noise from other tests.
"$VRK" kv set --ns listtest cherry val >/dev/null 2>&1
"$VRK" kv set --ns listtest apple  val >/dev/null 2>&1
"$VRK" kv set --ns listtest banana val >/dev/null 2>&1

stdout=$("$VRK" kv list --ns listtest 2>/dev/null); exit_code=$?
assert_exit       "list: exit 0"       0  "$exit_code"
assert_line_count "list: three lines"  3  "$stdout"

first=$(echo "$stdout"  | sed -n '1p')
second=$(echo "$stdout" | sed -n '2p')
third=$(echo "$stdout"  | sed -n '3p')
assert_stdout_equals "list: sorted[0]" "apple"  "$first"
assert_stdout_equals "list: sorted[1]" "banana" "$second"
assert_stdout_equals "list: sorted[2]" "cherry" "$third"

# Empty namespace
stdout=$("$VRK" kv list --ns emptyns 2>/dev/null); exit_code=$?
assert_exit        "list empty ns: exit 0"        0  "$exit_code"
assert_stdout_empty "list empty ns: stdout empty"   "$stdout"

# ------------------------------------------------------------
# 4. Empty string value
# ------------------------------------------------------------
echo ""
echo "--- empty string value ---"

"$VRK" kv set emptykey "" >/dev/null 2>&1
stdout=$("$VRK" kv get emptykey 2>/dev/null); exit_code=$?
assert_exit          "empty value: exit 0"         0  "$exit_code"
assert_stdout_equals "empty value: stdout empty line" "" "$stdout"

# ------------------------------------------------------------
# 5. Stdin value for set
# ------------------------------------------------------------
echo ""
echo "--- stdin value for set ---"

# echo appends \n; kv set strips exactly one trailing \n.
echo '{"status":"done"}' | "$VRK" kv set stdinkey >/dev/null 2>&1
stdout=$("$VRK" kv get stdinkey 2>/dev/null)
assert_stdout_equals "stdin set: value" '{"status":"done"}' "$stdout"

# ------------------------------------------------------------
# 6. incr / decr / --by
# ------------------------------------------------------------
echo ""
echo "--- incr / decr / --by ---"

stdout=$("$VRK" kv incr counter 2>/dev/null); exit_code=$?
assert_exit          "incr 1st: exit 0" 0  "$exit_code"
assert_stdout_equals "incr 1st: value"  "1" "$stdout"

stdout=$("$VRK" kv incr counter 2>/dev/null)
assert_stdout_equals "incr 2nd: value" "2" "$stdout"

stdout=$("$VRK" kv incr counter 2>/dev/null)
assert_stdout_equals "incr 3rd: value" "3" "$stdout"

stdout=$("$VRK" kv incr counter --by 5 2>/dev/null); exit_code=$?
assert_exit          "incr --by 5: exit 0" 0  "$exit_code"
assert_stdout_equals "incr --by 5: value"  "8" "$stdout"

stdout=$("$VRK" kv decr counter 2>/dev/null); exit_code=$?
assert_exit          "decr: exit 0" 0  "$exit_code"
assert_stdout_equals "decr: value"  "7" "$stdout"

# incr / decr on missing key start from 0
stdout=$("$VRK" kv incr --ns fresh freshkey 2>/dev/null)
assert_stdout_equals "incr missing: value 1"  "1" "$stdout"
stdout=$("$VRK" kv decr --ns fresh freshkey2 2>/dev/null)
assert_stdout_equals "decr missing: value -1" "-1" "$stdout"

# ------------------------------------------------------------
# 7. incr on non-numeric value
# ------------------------------------------------------------
echo ""
echo "--- incr non-numeric ---"

"$VRK" kv set badnum notanumber >/dev/null 2>&1
stdout=$(set +e; "$VRK" kv incr badnum 2>/dev/null; true)
exit_code=$(set +e; "$VRK" kv incr badnum >/dev/null 2>&1; echo $?)
stderr=$(set +e; "$VRK" kv incr badnum 2>&1 >/dev/null; true)
assert_exit        "incr non-numeric: exit 1"                  1              "$exit_code"
assert_stdout_empty "incr non-numeric: stdout empty"              "$stdout"
assert_stdout_contains "incr non-numeric: stderr message" "value is not a number" "$stderr"

# ------------------------------------------------------------
# 8. incr preserves TTL
# ------------------------------------------------------------
echo ""
echo "--- incr preserves TTL ---"

"$VRK" kv set ttlctr 5 --ttl 60s >/dev/null 2>&1

# incr should succeed and value should be 6.
stdout=$("$VRK" kv incr ttlctr 2>/dev/null); exit_code=$?
assert_exit          "incr on TTL key: exit 0"  0  "$exit_code"
assert_stdout_equals "incr on TTL key: value 6" "6" "$stdout"

# Key must still be readable (TTL was not wiped).
stdout=$("$VRK" kv get ttlctr 2>/dev/null); exit_code=$?
assert_exit          "incr TTL key still live: exit 0"  0  "$exit_code"
assert_stdout_equals "incr TTL key still live: value 6" "6" "$stdout"

# ------------------------------------------------------------
# 9. Namespace isolation
# ------------------------------------------------------------
echo ""
echo "--- namespace isolation ---"

"$VRK" kv set --ns myjob step 3 >/dev/null 2>&1

stdout=$("$VRK" kv get --ns myjob step 2>/dev/null); exit_code=$?
assert_exit          "ns get myjob: exit 0" 0  "$exit_code"
assert_stdout_equals "ns get myjob: value"  "3" "$stdout"

exit_code=$(set +e; "$VRK" kv get step >/dev/null 2>&1; echo $?)
assert_exit "ns get default: exit 1 (isolated)" 1 "$exit_code"

# ------------------------------------------------------------
# 10. TTL expiry
# ------------------------------------------------------------
echo ""
echo "--- TTL expiry ---"

"$VRK" kv set expiring value --ttl 1s >/dev/null 2>&1

# Key is live immediately.
exit_code=$(set +e; "$VRK" kv get expiring >/dev/null 2>&1; echo $?)
assert_exit "ttl: live before expiry" 0 "$exit_code"

sleep 2

exit_code=$(set +e; "$VRK" kv get expiring >/dev/null 2>&1; echo $?)
assert_exit "ttl: expired after sleep" 1 "$exit_code"
stderr=$(set +e; "$VRK" kv get expiring 2>&1 >/dev/null; true)
assert_stdout_contains "ttl: stderr key not found" "key not found" "$stderr"

# ------------------------------------------------------------
# 11. --dry-run
# ------------------------------------------------------------
echo ""
echo "--- --dry-run ---"

stdout=$("$VRK" kv set drykey dryval --dry-run 2>/dev/null); exit_code=$?
assert_exit          "--dry-run: exit 0"          0                       "$exit_code"
assert_stdout_contains "--dry-run: stdout intent" "would set drykey = dryval" "$stdout"

# Nothing should have been written.
exit_code=$(set +e; "$VRK" kv get drykey >/dev/null 2>&1; echo $?)
assert_exit "--dry-run: key not written" 1 "$exit_code"

# ------------------------------------------------------------
# 12. stdout / stderr separation
# ------------------------------------------------------------
echo ""
echo "--- stdout/stderr separation ---"

# Successful set: stdout empty, stderr empty.
stderr=$(set +e; "$VRK" kv set sepkey sepval 2>&1 >/dev/null; true)
assert_stderr_empty "set success: stderr empty" "$stderr"

# Successful get: stderr empty.
stderr=$(set +e; "$VRK" kv get sepkey 2>&1 >/dev/null; true)
assert_stderr_empty "get success: stderr empty" "$stderr"

# Error: stdout empty.
stdout=$(set +e; "$VRK" kv get nonexistent 2>/dev/null; true)
assert_stdout_empty "get error: stdout empty" "$stdout"

# ------------------------------------------------------------
# 13. Usage errors → exit 2, stdout empty
# ------------------------------------------------------------
echo ""
echo "--- usage errors ---"

exit_code=$(set +e; "$VRK" kv >/dev/null 2>&1; echo $?)
assert_exit "no subcommand: exit 2" 2 "$exit_code"
stdout=$(set +e; "$VRK" kv 2>/dev/null; true)
assert_stdout_empty "no subcommand: stdout empty" "$stdout"

exit_code=$(set +e; "$VRK" kv bogus >/dev/null 2>&1; echo $?)
assert_exit "unknown subcommand: exit 2" 2 "$exit_code"
stdout=$(set +e; "$VRK" kv bogus 2>/dev/null; true)
assert_stdout_empty "unknown subcommand: stdout empty" "$stdout"

exit_code=$(set +e; "$VRK" kv get --bogus-flag mykey >/dev/null 2>&1; echo $?)
assert_exit "unknown flag: exit 2" 2 "$exit_code"
stdout=$(set +e; "$VRK" kv get --bogus-flag mykey 2>/dev/null; true)
assert_stdout_empty "unknown flag: stdout empty" "$stdout"

exit_code=$(set +e; "$VRK" kv incr counter --by 0 >/dev/null 2>&1; echo $?)
assert_exit "--by 0: exit 2" 2 "$exit_code"

# ------------------------------------------------------------
# 14. --help → exit 0
# ------------------------------------------------------------
echo ""
echo "--- --help ---"

stdout=$("$VRK" kv --help 2>/dev/null) || true; exit_code=$(set +e; "$VRK" kv --help >/dev/null 2>&1; echo $?)
assert_exit          "kv --help: exit 0"      0    "$exit_code"
assert_stdout_contains "kv --help: mentions set" "set" "$stdout"
assert_stdout_contains "kv --help: mentions get" "get" "$stdout"

stdout=$("$VRK" kv set --help 2>/dev/null) || true; exit_code=$(set +e; "$VRK" kv set --help >/dev/null 2>&1; echo $?)
assert_exit          "kv set --help: exit 0"      0     "$exit_code"
assert_stdout_contains "kv set --help: mentions --ttl" "ttl" "$stdout"

# ------------------------------------------------------------
# 15. Concurrency: 10 parallel incr → final value must be 10
# ------------------------------------------------------------
echo ""
echo "--- concurrency ---"

"$VRK" kv set conckey 0 >/dev/null 2>&1

for i in $(seq 1 10); do
  "$VRK" kv incr conckey >/dev/null 2>&1 &
done
wait

stdout=$("$VRK" kv get conckey 2>/dev/null)
assert_stdout_equals "10 parallel incr: final value 10" "10" "$stdout"

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
