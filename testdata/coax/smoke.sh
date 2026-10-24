#!/usr/bin/env bash
# End-to-end smoke tests for vrk coax.
# Run via: make smoke  (binary available as $VRK)
# coax runs the command as argv, not through a shell, so shell snippets are
# passed as: -- sh -c '<script>'
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
        echo "  FAIL  $desc - expected exit $expected, got $actual"
        FAIL=$((FAIL+1))
    fi
}

expect_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [ "$actual" = "$expected" ]; then
        echo "  PASS  $desc"
        PASS=$((PASS+1))
    else
        echo "  FAIL  $desc - expected '$expected', got '$actual'"
        FAIL=$((FAIL+1))
    fi
}

echo "--- success ---"
expect_code "success immediate" 0 \
    "$VRK" coax -- true

echo ""
echo "--- argv execution (no shell) ---"
OUT=$("$VRK" coax --times 1 -- printf '%s\n' 'a b')
expect_eq "quoting preserved" "a b" "$OUT"
OUT=$("$VRK" coax --times 1 -- echo 'x; echo INJECTED')
expect_eq "no shell injection" "x; echo INJECTED" "$OUT"
expect_code "sh -c exit code passes through" 3 \
    "$VRK" coax --times 1 --backoff 1ms -- sh -c 'exit 3'
expect_code "missing command exits 1" 1 \
    "$VRK" coax --times 2 -- vrk-no-such-cmd-xyz
ERR=$("$VRK" coax --times 2 -- vrk-no-such-cmd-xyz 2>&1 >/dev/null)
expect_eq "missing command message" "coax: command not found: vrk-no-such-cmd-xyz" "$ERR"

echo ""
echo "--- retry exhaustion ---"
COUNTER="$TMPDIR_COAX/count"
expect_code "retry exhausted exits 1" 1 \
    "$VRK" coax -- sh -c "printf x >> $COUNTER; exit 1"
ATTEMPTS=$(wc -c < "$COUNTER" | tr -d ' ')
expect_eq "retry exhausted - 4 total attempts" "4" "$ATTEMPTS"
LAST=$("$VRK" coax --times 1 -- false 2>&1 | tail -n 1)
expect_eq "final exhaustion line" "coax: all 2 attempts failed (last exit 1)" "$LAST"

echo ""
echo "--- --times ---"
COUNTER2="$TMPDIR_COAX/count2"
expect_code "--times 2 exits 1" 1 \
    "$VRK" coax --times 2 -- sh -c "printf x >> $COUNTER2; exit 1"
ATTEMPTS2=$(wc -c < "$COUNTER2" | tr -d ' ')
expect_eq "--times 2 - 3 total attempts" "3" "$ATTEMPTS2"

echo ""
echo "--- --on ---"
expect_code "--on 42 with exit 42 exits 42" 42 \
    "$VRK" coax --on 42 -- sh -c 'exit 42'

COUNTER3="$TMPDIR_COAX/count3"
expect_code "--on 42 with exit 1 exits 1 immediately" 1 \
    "$VRK" coax --on 42 -- sh -c "printf x >> $COUNTER3; exit 1"
ATTEMPTS3=$(wc -c < "$COUNTER3" | tr -d ' ')
expect_eq "--on no-match - exactly 1 attempt" "1" "$ATTEMPTS3"

echo ""
echo "--- exit code passthrough ---"
expect_code "exit code passthrough (7)" 7 \
    "$VRK" coax --times 1 -- sh -c 'exit 7'
expect_code "signal exit code is 128+N" 143 \
    "$VRK" coax --times 1 --on 1 -- sh -c 'kill -TERM $$'

echo ""
echo "--- backoff ---"
expect_code "--backoff 100ms exits 1" 1 \
    "$VRK" coax --times 2 --backoff 100ms -- false
expect_code "--backoff exp:50ms exits 1" 1 \
    "$VRK" coax --times 2 --backoff exp:50ms -- false
expect_code "--backoff exp:50ms --backoff-max 60ms exits 1" 1 \
    "$VRK" coax --times 2 --backoff exp:50ms --backoff-max 60ms -- false

echo ""
echo "--- stdin ---"
STDIN_OUT=$(printf 'hello\n' | "$VRK" coax --times 3 -- cat)
expect_eq "stdin re-pipe" "hello" "$STDIN_OUT"
REPLAY=$(printf abc | "$VRK" coax --times 2 --backoff 1ms -- sh -c 'cat; exit 1' 2>/dev/null)
expect_eq "stdin replayed on every attempt" "abcabcabc" "$REPLAY"
# An open stdin pipe must not block a command that ignores stdin. Bash waits
# for sleep either way, so time coax itself on the right side of the pipe.
RESULT=$(sleep 5 | { s=$(date +%s); "$VRK" coax --times 1 -- true; c=$?; echo "$c $(( $(date +%s) - s ))"; })
expect_eq "open stdin pipe: exit 0" "0" "${RESULT% *}"
if [ "${RESULT#* }" -lt 3 ]; then
    echo "  PASS  open stdin pipe returns at once (${RESULT#* }s)"
    PASS=$((PASS+1))
else
    echo "  FAIL  open stdin pipe took ${RESULT#* }s"
    FAIL=$((FAIL+1))
fi

echo ""
echo "--- --until ---"
DONEFILE="$TMPDIR_COAX/done"
expect_code "--until condition satisfied" 0 \
    "$VRK" coax --times 5 --until "test -f $DONEFILE" -- touch "$DONEFILE"

echo ""
echo "--- --quiet ---"
QUIET_STDERR=$("$VRK" coax --quiet --times 2 -- false 2>&1 || true)
if echo "$QUIET_STDERR" | grep -q "coax:"; then
    echo "  FAIL  --quiet: coax: lines still present in stderr"
    FAIL=$((FAIL+1))
else
    echo "  PASS  --quiet suppresses coax: lines"
    PASS=$((PASS+1))
fi

SUB_STDERR=$("$VRK" coax --quiet --times 1 -- sh -c "printf 'from_sub\n' >&2; exit 1" 2>&1 || true)
if echo "$SUB_STDERR" | grep -q "from_sub"; then
    echo "  PASS  --quiet: subprocess stderr passes through"
    PASS=$((PASS+1))
else
    echo "  FAIL  --quiet: subprocess stderr missing - got '$SUB_STDERR'"
    FAIL=$((FAIL+1))
fi

echo ""
echo "--- usage errors ---"
expect_code "--times 0 exits 2 (usage error)" 2 \
    "$VRK" coax --times 0 -- false
expect_code "no command exits 2 (usage error)" 2 \
    "$VRK" coax

echo ""
echo "--- --help ---"
expect_code "--help exits 0" 0 \
    "$VRK" coax --help

echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
