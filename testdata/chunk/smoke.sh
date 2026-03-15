#!/usr/bin/env bash
# testdata/chunk/smoke.sh
#
# End-to-end smoke tests for vrk chunk.
# Run after make build to verify real process behaviour: exit codes, stdout/stderr
# separation, and pipeline composition that unit tests cannot exercise.
#
# Usage:
#   ./testdata/chunk/smoke.sh              # run against ./vrk in repo root
#   VRK=./vrk ./testdata/chunk/smoke.sh   # explicit binary path

set -uo pipefail

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
  local desc=$1 expected=$2
  shift 2
  local actual=0
  "$@" >/dev/null 2>&1 || actual=$?
  if [ "$actual" -eq "$expected" ]; then
    ok "$desc (exit $expected)"
  else
    fail "$desc" "expected exit $expected, got exit $actual"
  fi
}

echo "vrk chunk — smoke tests"
echo "binary: $VRK"
echo ""

# ------------------------------------------------------------
# 1. Empty input
# ------------------------------------------------------------
echo "--- 1. empty input ---"

assert_exit "empty input: exit 0" 0 sh -c "printf '' | $VRK chunk --size 1000"

out=$(printf '' | "$VRK" chunk --size 1000 2>/dev/null) || true
if [ -z "$out" ]; then
  ok "empty input: no output"
else
  fail "empty input: no output" "got: $out"
fi

# ------------------------------------------------------------
# 2. Usage errors
# ------------------------------------------------------------
echo ""
echo "--- 2. usage errors ---"

assert_exit "no --size: exit 2"              2 sh -c "echo hello | $VRK chunk"
assert_exit "--size 0: exit 2"               2 sh -c "echo hello | $VRK chunk --size 0"
assert_exit "--overlap >= --size: exit 2"    2 sh -c "echo hello | $VRK chunk --size 10 --overlap 10"
assert_exit "unknown --by mode: exit 2"      2 sh -c "echo hello | $VRK chunk --size 100 --by sentence"
assert_exit "unknown flag: exit 2"           2 sh -c "echo hello | $VRK chunk --size 100 --bogus"

err_out=$(echo hello | "$VRK" chunk --size 0 2>/dev/null) || true
if [ -z "$err_out" ]; then
  ok "usage error: stdout empty"
else
  fail "usage error: stdout empty" "got: $err_out"
fi

# ------------------------------------------------------------
# 3. Basic split
# ------------------------------------------------------------
echo ""
echo "--- 3. basic split ---"

# "hello" = 1 token, " hello" = 1 token → "hello" + " hello"×2499 = 2500 tokens.
make_input() {
  python3 -c "import sys; sys.stdout.write('hello' + ' hello' * 2499)"
}

chunk_out=$(make_input | "$VRK" chunk --size 1000 2>/dev/null)
chunk_count=$(echo "$chunk_out" | wc -l | tr -d ' ')
if [ "$chunk_count" -eq 3 ]; then
  ok "basic split: 2500 tokens / 1000 = 3 chunks"
else
  fail "basic split" "got $chunk_count chunks, want 3"
fi

invariant_ok=$(echo "$chunk_out" | python3 -c "
import sys, json
size = 1000
bad = [r for r in (json.loads(l) for l in sys.stdin if l.strip()) if r['tokens'] > size]
print('ok' if not bad else 'fail')
" 2>/dev/null) || true
if [ "$invariant_ok" = "ok" ]; then
  ok "invariant: all chunks <= 1000 tokens"
else
  fail "invariant: tokens" "some chunk exceeds 1000 tokens"
fi

index_ok=$(echo "$chunk_out" | python3 -c "
import sys, json
recs = [json.loads(l) for l in sys.stdin if l.strip()]
ok = all(r['index'] == i for i, r in enumerate(recs))
print('ok' if ok else 'fail')
" 2>/dev/null) || true
if [ "$index_ok" = "ok" ]; then
  ok "index is 0-based and sequential"
else
  fail "index" "index is not sequential"
fi

# ------------------------------------------------------------
# 4. Overlap
# ------------------------------------------------------------
echo ""
echo "--- 4. overlap ---"

overlap_out=$(make_input | "$VRK" chunk --size 1000 --overlap 100 2>/dev/null)
overlap_ok=$(echo "$overlap_out" | python3 -c "
import sys, json
recs = [json.loads(l) for l in sys.stdin if l.strip()]
ok = len(recs) >= 3 and all(r['text'] for r in recs)
print('ok' if ok else 'fail')
" 2>/dev/null) || true
if [ "$overlap_ok" = "ok" ]; then
  ok "overlap: produces expected number of chunks"
else
  fail "overlap" "check failed"
fi

# ------------------------------------------------------------
# 5. --by paragraph
# ------------------------------------------------------------
echo ""
echo "--- 5. --by paragraph ---"

P1="The quick brown fox jumps over the lazy dog."
P2="Pack my box with five dozen liquor jugs."
P3="How vexingly quick daft zebras jump."

para_out=$(printf '%s\n\n%s\n\n%s' "$P1" "$P2" "$P3" | "$VRK" chunk --size 500 --by paragraph 2>/dev/null)

for para in "$P1" "$P2" "$P3"; do
  if echo "$para_out" | grep -qF "$para"; then
    ok "--by paragraph: keeps whole: ${para:0:30}..."
  else
    fail "--by paragraph: keeps whole" "paragraph was split: ${para:0:30}..."
  fi
done

para_inv=$(echo "$para_out" | python3 -c "
import sys, json
size = 500
bad = [r for r in (json.loads(l) for l in sys.stdin if l.strip()) if r['tokens'] > size]
print('ok' if not bad else 'fail')
" 2>/dev/null) || true
if [ "$para_inv" = "ok" ]; then
  ok "--by paragraph: invariant holds (<= 500 tokens per chunk)"
else
  fail "--by paragraph: invariant" "some chunk exceeds 500 tokens"
fi

# ------------------------------------------------------------
# 6. vrk --manifest lists chunk
# ------------------------------------------------------------
echo ""
echo "--- 6. manifest ---"

if "$VRK" --manifest 2>/dev/null | python3 -c "
import sys, json
d = json.load(sys.stdin)
names = [t['name'] for t in d['tools']]
sys.exit(0 if 'chunk' in names else 1)
" 2>/dev/null; then
  ok "vrk --manifest lists chunk"
else
  fail "vrk --manifest lists chunk" "--manifest does not include chunk"
fi

# ------------------------------------------------------------
# --quiet flag
# ------------------------------------------------------------
echo ""
echo "--- --quiet ---"

# --quiet suppresses usage error when --size is omitted (fires after defer)
stderr=$(printf 'hello' | "$VRK" chunk --quiet 2>&1 >/dev/null) || true
exit_code=0; printf 'hello' | "$VRK" chunk --quiet > /dev/null 2>&1 || exit_code=$?
if [ "$exit_code" -eq 2 ]; then
  ok "--quiet error: exit 2 (missing --size)"
else
  fail "--quiet error: exit 2 (missing --size)" "expected exit 2, got $exit_code"
fi
if [ -z "$stderr" ]; then
  ok "--quiet error: stderr empty"
else
  fail "--quiet error: stderr empty" "got: $stderr"
fi

# --quiet success: stdout unaffected
stdout=$(printf 'hello world' | "$VRK" chunk --size 100 --quiet 2>/dev/null) || true
exit_code=0; printf 'hello world' | "$VRK" chunk --size 100 --quiet > /dev/null 2>&1 || exit_code=$?
if [ "$exit_code" -eq 0 ]; then
  ok "--quiet success: exit 0"
else
  fail "--quiet success: exit 0" "expected exit 0, got $exit_code"
fi
if echo "$stdout" | grep -q '"text"'; then
  ok "--quiet success: stdout has chunk"
else
  fail "--quiet success: stdout has chunk" "stdout did not contain '\"text\"'. got: $stdout"
fi

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
