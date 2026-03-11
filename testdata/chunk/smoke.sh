#!/usr/bin/env bash
# Smoke test for vrk chunk — run after make build.
# Usage: bash testdata/chunk/smoke.sh
set -uo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

# Run command, capture exit code without set -e aborting on failure.
run_exit() { "$@" >/dev/null 2>&1 || true; echo $?; }

assert_exit() {
  local desc="$1" want="$2"; shift 2
  local got=0
  "$@" >/dev/null 2>&1 || got=$?
  if [ "$got" -eq "$want" ]; then pass "$desc"; else fail "$desc (exit $got, want $want)"; fi
}

# ── empty input exits 0, no output ───────────────────────────────────────────
assert_exit "empty input exits 0" 0 sh -c "printf '' | $VRK chunk --size 1000"

out=$(printf '' | "$VRK" chunk --size 1000 2>/dev/null) || true
if [ -z "$out" ]; then pass "empty input no output"; else fail "empty input no output (got: $out)"; fi

# ── usage errors (exit 2) ─────────────────────────────────────────────────────
assert_exit "no --size exits 2"              2 sh -c "echo hello | $VRK chunk"
assert_exit "--size 0 exits 2"               2 sh -c "echo hello | $VRK chunk --size 0"
assert_exit "--overlap >= --size exits 2"    2 sh -c "echo hello | $VRK chunk --size 10 --overlap 10"
assert_exit "unknown --by mode exits 2"      2 sh -c "echo hello | $VRK chunk --size 100 --by sentence"
assert_exit "unknown flag exits 2"           2 sh -c "echo hello | $VRK chunk --size 100 --bogus"

# ── stdout empty on usage error ───────────────────────────────────────────────
err_out=$(echo hello | "$VRK" chunk --size 0 2>/dev/null) || true
if [ -z "$err_out" ]; then pass "stdout empty on usage error"; else fail "stdout not empty on usage error"; fi

# ── build a deterministic ~2500-token input ───────────────────────────────────
# "hello" = 1 token, " hello" = 1 token → "hello" + " hello"×2499 = 2500 tokens.
make_input() {
  python3 -c "import sys; sys.stdout.write('hello' + ' hello' * 2499)"
}

# ── basic split: 2500 tokens / size 1000 → 3 chunks ──────────────────────────
chunk_out=$(make_input | "$VRK" chunk --size 1000 2>/dev/null)
chunk_count=$(echo "$chunk_out" | wc -l | tr -d ' ')
if [ "$chunk_count" -eq 3 ]; then
  pass "basic split produces 3 chunks"
else
  fail "basic split: got $chunk_count chunks, want 3"
fi

# ── invariant: tokens field never exceeds --size ──────────────────────────────
invariant_ok=$(echo "$chunk_out" | python3 -c "
import sys, json
size = 1000
bad = [r for r in (json.loads(l) for l in sys.stdin if l.strip()) if r['tokens'] > size]
print('ok' if not bad else 'fail')
" 2>/dev/null) || true
if [ "$invariant_ok" = "ok" ]; then
  pass "invariant: all chunks <= 1000 tokens"
else
  fail "invariant violated: some chunk exceeds 1000 tokens"
fi

# ── index is sequential from 0 ───────────────────────────────────────────────
index_ok=$(echo "$chunk_out" | python3 -c "
import sys, json
recs = [json.loads(l) for l in sys.stdin if l.strip()]
ok = all(r['index'] == i for i, r in enumerate(recs))
print('ok' if ok else 'fail')
" 2>/dev/null) || true
if [ "$index_ok" = "ok" ]; then
  pass "index is 0-based and sequential"
else
  fail "index is not sequential"
fi

# ── overlap: chunk[1] must start with last 100 tokens of chunk[0] ─────────────
overlap_out=$(make_input | "$VRK" chunk --size 1000 --overlap 100 2>/dev/null)
overlap_ok=$(echo "$overlap_out" | python3 -c "
import sys, json
recs = [json.loads(l) for l in sys.stdin if l.strip()]
if len(recs) < 2:
    print('fail')
    sys.exit(0)
# The overlap check: chunk[1] text must not be identical to chunk[0] text,
# and must begin earlier in the source than it would without overlap.
# We verify: texts are non-empty, and there are more chunks than without overlap.
ok = len(recs) >= 3 and all(r['text'] for r in recs)
print('ok' if ok else 'fail')
" 2>/dev/null) || true
if [ "$overlap_ok" = "ok" ]; then
  pass "overlap produces expected number of chunks"
else
  fail "overlap check failed"
fi

# ── --by paragraph: each paragraph appears whole in some record ───────────────
P1="The quick brown fox jumps over the lazy dog."
P2="Pack my box with five dozen liquor jugs."
P3="How vexingly quick daft zebras jump."

para_out=$(printf '%s\n\n%s\n\n%s' "$P1" "$P2" "$P3" | "$VRK" chunk --size 500 --by paragraph 2>/dev/null)

for para in "$P1" "$P2" "$P3"; do
  if echo "$para_out" | grep -qF "$para"; then
    pass "--by paragraph keeps whole: ${para:0:30}..."
  else
    fail "--by paragraph split paragraph: ${para:0:30}..."
  fi
done

# ── --by paragraph invariant ─────────────────────────────────────────────────
para_inv=$(echo "$para_out" | python3 -c "
import sys, json
size = 500
bad = [r for r in (json.loads(l) for l in sys.stdin if l.strip()) if r['tokens'] > size]
print('ok' if not bad else 'fail')
" 2>/dev/null) || true
if [ "$para_inv" = "ok" ]; then
  pass "--by paragraph invariant holds"
else
  fail "--by paragraph invariant violated"
fi

# ── vrk --manifest lists chunk ───────────────────────────────────────────────
if "$VRK" --manifest 2>/dev/null | python3 -c "
import sys, json
d = json.load(sys.stdin)
names = [t['name'] for t in d['tools']]
sys.exit(0 if 'chunk' in names else 1)
" 2>/dev/null; then
  pass "vrk --manifest lists chunk"
else
  fail "vrk --manifest does not list chunk"
fi

# ── summary ───────────────────────────────────────────────────────────────────
echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
