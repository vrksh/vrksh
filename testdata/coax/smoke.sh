#!/usr/bin/env bash
# End-to-end smoke tests for vrk coax.
# Run via: make smoke  (binary available as $VRK)
# Shell scripts passed as a single arg after -- so coax runs: sh -c "<script>"
# rather than the double-wrapping: sh -c "sh -c <script>".
set -uo pipefail

VRK=${VRK:-./vrk}
TMPDIR_COAX=$(mktemp -d)
trap 'rm -rf "$TMPDIR_COAX"' EXIT

PASS=0
FAIL=0

# expect_code <desc> <expected-exit> <cmd...>
expect_code() {
    local desc="$1" expected="$2"
    shift 2
    local actual=0
    "$@" || actual=$?
    if [ "$actual" -eq "$expected" ]; then
        echo "  PASS  $desc"
        PASS=$((PASS+1))
    else
        echo "  FAIL  $desc — expected exit $expected, got $actual"
        FAIL=$((FAIL+1))
    fi
}

expect_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [ "$actual" = "$expected" ]; then
        echo "  PASS  $desc"
        PASS=$((PASS+1))
    else
        echo "  FAIL  $desc — expected '$expected', got '$actual'"
        FAIL=$((FAIL+1))
    fi
}

echo "--- success ---"
expect_code "success immediate" 0 \
    "$VRK" coax -- exit 0

echo ""
echo "--- retry exhaustion ---"
COUNTER="$TMPDIR_COAX/count"
expect_code "retry exhausted exits 1" 1 \
    "$VRK" coax -- "printf x >> $COUNTER; exit 1"
ATTEMPTS=$(wc -c < "$COUNTER" | tr -d ' ')
expect_eq "retry exhausted — 4 total attempts" "4" "$ATTEMPTS"

echo ""
echo "--- --times ---"
COUNTER2="$TMPDIR_COAX/count2"
expect_code "--times 2 exits 1" 1 \
    "$VRK" coax --times 2 -- "printf x >> $COUNTER2; exit 1"
ATTEMPTS2=$(wc -c < "$COUNTER2" | tr -d ' ')
expect_eq "--times 2 — 3 total attempts" "3" "$ATTEMPTS2"

echo ""
echo "--- --on ---"
expect_code "--on 42 with exit 42 exits 42" 42 \
    "$VRK" coax --on 42 -- exit 42

COUNTER3="$TMPDIR_COAX/count3"
expect_code "--on 42 with exit 1 exits 1 immediately" 1 \
    "$VRK" coax --on 42 -- "printf x >> $COUNTER3; exit 1"
ATTEMPTS3=$(wc -c < "$COUNTER3" | tr -d ' ')
expect_eq "--on no-match — exactly 1 attempt" "1" "$ATTEMPTS3"

echo ""
echo "--- exit code passthrough ---"
expect_code "exit code passthrough (7)" 7 \
    "$VRK" coax --times 1 -- exit 7

echo ""
echo "--- backoff ---"
expect_code "--backoff 100ms exits 1" 1 \
    "$VRK" coax --times 2 --backoff 100ms -- exit 1
expect_code "--backoff exp:50ms exits 1" 1 \
    "$VRK" coax --times 2 --backoff exp:50ms -- exit 1
expect_code "--backoff exp:50ms --backoff-max 60ms exits 1" 1 \
    "$VRK" coax --times 2 --backoff exp:50ms --backoff-max 60ms -- exit 1

echo ""
echo "--- stdin re-pipe ---"
STDIN_OUT=$(printf 'hello\n' | "$VRK" coax --times 3 -- cat)
expect_eq "stdin re-pipe" "hello" "$STDIN_OUT"

echo ""
echo "--- --until ---"
DONEFILE="$TMPDIR_COAX/done"
expect_code "--until condition satisfied" 0 \
    "$VRK" coax --times 5 --until "test -f $DONEFILE" -- touch "$DONEFILE"

echo ""
echo "--- --quiet ---"
QUIET_STDERR=$("$VRK" coax --quiet --times 2 -- exit 1 2>&1 || true)
if echo "$QUIET_STDERR" | grep -q "coax:"; then
    echo "  FAIL  --quiet: coax: lines still present in stderr"
    FAIL=$((FAIL+1))
else
    echo "  PASS  --quiet suppresses coax: lines"
    PASS=$((PASS+1))
fi

SUB_STDERR=$("$VRK" coax --quiet --times 1 -- "printf 'from_sub\n' >&2; exit 1" 2>&1 || true)
if echo "$SUB_STDERR" | grep -q "from_sub"; then
    echo "  PASS  --quiet: subprocess stderr passes through"
    PASS=$((PASS+1))
else
    echo "  FAIL  --quiet: subprocess stderr missing — got '$SUB_STDERR'"
    FAIL=$((FAIL+1))
fi

echo ""
echo "--- usage errors ---"
expect_code "--times 0 exits 2 (usage error)" 2 \
    "$VRK" coax --times 0 -- exit 1
expect_code "no command exits 2 (usage error)" 2 \
    "$VRK" coax

echo ""
echo "--- --help ---"
expect_code "--help exits 0" 0 \
    "$VRK" coax --help

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
