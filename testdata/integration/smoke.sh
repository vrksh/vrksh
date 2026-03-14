#!/usr/bin/env bash
# testdata/integration/smoke.sh
#
# Integration smoke tests: tool composition and real pipelines.
# Each section pipes one tool into another — no mocks, no stubs.
#
# Usage:
#   ./testdata/integration/smoke.sh
#   VRK=./vrk ./testdata/integration/smoke.sh
#
# Exit 0 if all pass, exit 1 if any fail.

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# milliseconds portable across macOS (BSD date has +%s%N on recent versions,
# but fall back to python3 if it returns the literal "%N")
_now_ms() {
  local raw
  raw=$(date +%s%N 2>/dev/null)
  if [[ "$raw" == *N ]]; then
    python3 -c 'import time; print(int(time.time()*1000))'
  else
    echo $(( raw / 1000000 ))
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

assert_stdout() {
  local desc="$1" expected="$2" actual="$3"
  if echo "$actual" | grep -qF "$expected"; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (expected stdout to contain: $expected)"
    echo "  got: $actual"
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

assert_valid_jsonl() {
  local desc="$1" actual="$2"
  local failed=0
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    echo "$line" | python3 -c 'import sys,json; json.load(sys.stdin)' 2>/dev/null || failed=1
  done <<< "$actual"
  if [ "$failed" -eq 0 ]; then
    echo "PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $desc (not valid JSONL)"
    FAIL=$((FAIL + 1))
  fi
}

# Build a minimal JWT (unsigned — jwt tool doesn't verify signatures).
# Usage: make_jwt '{"sub":"x","exp":1234567890}'
make_jwt() {
  python3 -c "
import base64, json, sys
payload = json.loads(sys.argv[1])
h = base64.urlsafe_b64encode(json.dumps({'alg':'HS256','typ':'JWT'}).encode()).rstrip(b'=').decode()
p = base64.urlsafe_b64encode(json.dumps(payload).encode()).rstrip(b'=').decode()
print(f'{h}.{p}.fakesig')
" "$1"
}

# ---------------------------------------------------------------------------
# Section 1 — jwt + epoch
# ---------------------------------------------------------------------------
echo "--- Section 1: jwt + epoch ---"

JWT=$(make_jwt '{"sub":"123","exp":1740009600}')
out=$(echo "$JWT" | $VRK jwt --json | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d["payload"]["exp"])')
result=$(echo "$out" | $VRK epoch --iso)
assert_stdout "jwt payload exp → epoch --iso" "2025-02-20" "$result"

EXPIRED_JWT=$(make_jwt '{"sub":"123","exp":1}')
ec=$(set +e; echo "$EXPIRED_JWT" | $VRK jwt --expired 2>/dev/null; echo $?)
assert_exit "jwt --expired on expired token exits 1" 1 "$ec"

out=$(echo "$JWT" | $VRK jwt --json)
assert_valid_json "jwt --json produces valid JSON" "$out"

# ---------------------------------------------------------------------------
# Section 2 — epoch + kv
# ---------------------------------------------------------------------------
echo "--- Section 2: epoch + kv ---"

VRK_KV_PATH="$TMPDIR/epoch_kv.db"
export VRK_KV_PATH

now=$($VRK epoch --now --quiet)
$VRK kv set last_run "$now"
stored=$($VRK kv get last_run)
iso=$(echo "$stored" | $VRK epoch --iso)
assert_stdout "epoch → kv → epoch roundtrip" "T" "$iso"

$VRK kv set temp_ts "$now" --ttl 1s
sleep 2
ec=$(set +e; $VRK kv get temp_ts 2>/dev/null; echo $?)
assert_exit "epoch value in kv expires via TTL" 1 "$ec"

unset VRK_KV_PATH

# ---------------------------------------------------------------------------
# Section 3 — tok + kv (budget guard pattern)
# ---------------------------------------------------------------------------
echo "--- Section 3: tok + kv ---"

VRK_KV_PATH="$TMPDIR/tok_kv.db"
export VRK_KV_PATH

text="The quick brown fox jumps over the lazy dog"
count=$(echo "$text" | $VRK tok --quiet)
$VRK kv set token_count "$count"
retrieved=$($VRK kv get token_count)
assert_stdout "tok count stored and retrieved via kv" "$count" "$retrieved"

echo "$text" | $VRK tok --budget 100 > /dev/null
assert_exit "tok --budget passes under limit" 0 $?
ec=$(set +e; echo "$text" | $VRK tok --budget 1 2>/dev/null; echo $?)
assert_exit "tok --budget fails over limit" 1 "$ec"

unset VRK_KV_PATH

# ---------------------------------------------------------------------------
# Section 4 — uuid + kv (request-scoped storage)
# ---------------------------------------------------------------------------
echo "--- Section 4: uuid + kv ---"

VRK_KV_PATH="$TMPDIR/uuid_kv.db"
export VRK_KV_PATH

req_id=$($VRK uuid)
$VRK kv set "req:$req_id" '{"status":"pending"}'
result=$($VRK kv get "req:$req_id")
assert_stdout "uuid as kv key stores and retrieves" "pending" "$result"

id1=$($VRK uuid)
id2=$($VRK uuid)
[ "$id1" != "$id2" ] \
  && { echo "PASS: uuid generates unique values"; PASS=$((PASS+1)); } \
  || { echo "FAIL: uuid generated duplicate"; FAIL=$((FAIL+1)); }

unset VRK_KV_PATH

# ---------------------------------------------------------------------------
# Section 5 — sse + tok (stream token counting)
# ---------------------------------------------------------------------------
echo "--- Section 5: sse + tok ---"

SSE_FIXTURE="$TMPDIR/stream.sse"
cat > "$SSE_FIXTURE" << 'SSEEOF'
event: content_block_delta
data: {"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"delta":{"type":"text_delta","text":" world"}}

event: content_block_delta
data: {"delta":{"type":"text_delta","text":"."}}

data: [DONE]

SSEEOF

text=$($VRK sse --field data.delta.text < "$SSE_FIXTURE" | tr -d '\n')
assert_stdout "sse --field extracts text from stream" "Hello world." "$text"

count=$(echo "$text" | $VRK tok --quiet)
[ "$count" -gt 0 ] \
  && { echo "PASS: sse output piped into tok produces count > 0"; PASS=$((PASS+1)); } \
  || { echo "FAIL: tok count was 0"; FAIL=$((FAIL+1)); }

# ---------------------------------------------------------------------------
# Section 6 — sse + kv (stream state persistence)
# ---------------------------------------------------------------------------
echo "--- Section 6: sse + kv ---"

VRK_KV_PATH="$TMPDIR/sse_kv.db"
export VRK_KV_PATH

SSE_FIXTURE2="$TMPDIR/stream2.sse"
cat > "$SSE_FIXTURE2" << 'SSEEOF'
event: content_block_delta
data: {"delta":{"text":"chunk one"}}

event: content_block_delta
data: {"delta":{"text":"chunk two"}}

data: [DONE]

SSEEOF

i=0
while IFS= read -r chunk; do
  [ -z "$chunk" ] && continue
  key=$($VRK uuid)
  $VRK kv set "chunk:$key" "$chunk"
  i=$((i+1))
done < <($VRK sse --field data.delta.text < "$SSE_FIXTURE2")

[ "$i" -eq 2 ] \
  && { echo "PASS: two sse chunks stored in kv"; PASS=$((PASS+1)); } \
  || { echo "FAIL: expected 2 chunks, got $i"; FAIL=$((FAIL+1)); }

unset VRK_KV_PATH

# ---------------------------------------------------------------------------
# Section 7 — coax + kv (retry until condition met)
# ---------------------------------------------------------------------------
echo "--- Section 7: coax + kv ---"

VRK_KV_PATH="$TMPDIR/coax_kv.db"
export VRK_KV_PATH

$VRK kv set attempt_count 0

# Write the flaky logic to a temp script — coax already wraps in sh -c internally,
# so passing "sh -c ..." as the command would double-wrap and break quoting.
FLAKY_SCRIPT="$TMPDIR/flaky.sh"
VRK_ABS="$(cd "$(dirname "$VRK")" && pwd)/$(basename "$VRK")"
cat > "$FLAKY_SCRIPT" << SCRIPTEOF
#!/bin/sh
VRK_KV_PATH="$TMPDIR/coax_kv.db"
export VRK_KV_PATH
"$VRK_ABS" kv incr attempt_count > /dev/null
count=\$("$VRK_ABS" kv get attempt_count)
[ "\$count" -ge 3 ]
SCRIPTEOF
chmod +x "$FLAKY_SCRIPT"

$VRK coax --times 5 -- "$FLAKY_SCRIPT"
assert_exit "coax retries until condition met" 0 $?

final=$($VRK kv get attempt_count)
[ "$final" -ge 3 ] \
  && { echo "PASS: coax ran at least 3 attempts"; PASS=$((PASS+1)); } \
  || { echo "FAIL: attempt_count was $final"; FAIL=$((FAIL+1)); }

# coax exhaustion: verify it exits 1 (not 0) when all retries are spent
ec=$(set +e; $VRK coax --times 2 -- false 2>/dev/null; echo $?)
assert_exit "coax exits 1 when retries exhausted" 1 "$ec"

unset VRK_KV_PATH

# ---------------------------------------------------------------------------
# Section 8 — coax + epoch (retry with backoff, timing)
# ---------------------------------------------------------------------------
echo "--- Section 8: coax + epoch (backoff timing) ---"

start=$(_now_ms)
$VRK coax --times 2 --backoff 100ms -- false || true
end=$(_now_ms)
elapsed_ms=$(( end - start ))

[ "$elapsed_ms" -ge 150 ] \
  && { echo "PASS: coax backoff adds delay (${elapsed_ms}ms)"; PASS=$((PASS+1)); } \
  || { echo "FAIL: coax backoff too fast (${elapsed_ms}ms, expected >= 150ms)"; FAIL=$((FAIL+1)); }

# ---------------------------------------------------------------------------
# Section 9 — jwt + kv (token cache pattern)
# ---------------------------------------------------------------------------
echo "--- Section 9: jwt + kv ---"

VRK_KV_PATH="$TMPDIR/jwt_kv.db"
export VRK_KV_PATH

JWT=$(make_jwt '{"sub":"user_123","role":"admin","exp":9999999999}')
$VRK kv set "jwt:user_123" "$JWT"

cached=$($VRK kv get "jwt:user_123")
claim=$(echo "$cached" | $VRK jwt --claim sub --quiet)
assert_stdout "jwt cached in kv, claim extracted on retrieval" "user_123" "$claim"

$VRK kv set "jwt:ttl_test" "$JWT" --ttl 60s
$VRK kv get "jwt:ttl_test" > /dev/null
assert_exit "jwt cached with TTL is retrievable within window" 0 $?

unset VRK_KV_PATH

# ---------------------------------------------------------------------------
# Section 10 — three-tool chains
# ---------------------------------------------------------------------------
echo "--- Section 10: three-tool chains ---"

VRK_KV_PATH="$TMPDIR/chain_kv.db"
export VRK_KV_PATH

key=$($VRK uuid)
echo "some pipeline content for token counting" | $VRK kv set "$key"
content=$($VRK kv get "$key")
count=$(echo "$content" | $VRK tok --quiet)
[ "$count" -gt 0 ] \
  && { echo "PASS: uuid→kv→tok chain works"; PASS=$((PASS+1)); } \
  || { echo "FAIL: uuid→kv→tok chain failed"; FAIL=$((FAIL+1)); }

ts=$($VRK epoch --now --quiet)
$VRK kv set pipeline_start "$ts"
retrieved=$($VRK kv get pipeline_start)
iso=$(echo "$retrieved" | $VRK epoch --iso)
assert_stdout "epoch→kv→epoch roundtrip produces ISO" "T" "$iso"

exp=$($VRK epoch '+1h' --quiet)
JWT=$(make_jwt "{\"sub\":\"chain\",\"exp\":$exp}")
$VRK kv set "session:chain" "$JWT" --ttl 3600s
result=$($VRK kv get "session:chain" | $VRK jwt --claim sub --quiet)
assert_stdout "jwt→kv→jwt claim chain" "chain" "$result"

unset VRK_KV_PATH

# ---------------------------------------------------------------------------
# Section 11 — prompt offline (--explain; no API call)
# ---------------------------------------------------------------------------
echo "--- Section 11: prompt (offline --explain) ---"

# tok budget gate → prompt --explain: under budget, prompt would be called
text="Summarize this for me"
count=$(echo "$text" | $VRK tok --quiet)
explain=$(echo "$text" | $VRK prompt --explain)
[ "$count" -gt 0 ] && echo "$explain" | grep -q "claude" \
  && { echo "PASS: tok count then prompt --explain contains model"; PASS=$((PASS+1)); } \
  || { echo "FAIL: tok→prompt --explain chain"; FAIL=$((FAIL+1)); }

# jwt claim piped into prompt --explain (verify the decoded claim travels through)
JWT=$(make_jwt '{"sub":"pipeline_user","exp":9999999999}')
claim=$(echo "$JWT" | $VRK jwt --claim sub --quiet)
explain=$(echo "Greet the user: $claim" | $VRK prompt --explain)
assert_stdout "jwt claim → prompt --explain carries claim text" "pipeline_user" "$explain"

# --explain output is valid curl: verify structural markers
explain=$(echo "hello" | $VRK prompt --explain)
assert_stdout "prompt --explain contains API endpoint" "api.anthropic.com" "$explain"
assert_stdout "prompt --explain contains max_tokens" "max_tokens" "$explain"

# --explain stdout must be non-empty; stderr must be empty
explain_out=$(echo "test" | $VRK prompt --explain 2>/dev/null)
[ -n "$explain_out" ] \
  && { echo "PASS: prompt --explain stdout is non-empty"; PASS=$((PASS+1)); } \
  || { echo "FAIL: prompt --explain stdout was empty"; FAIL=$((FAIL+1)); }
explain_err=$(echo "test" | $VRK prompt --explain 2>&1 >/dev/null)
[ -z "$explain_err" ] \
  && { echo "PASS: prompt --explain stderr is empty"; PASS=$((PASS+1)); } \
  || { echo "FAIL: prompt --explain leaked to stderr: $explain_err"; FAIL=$((FAIL+1)); }

# epoch + prompt --explain: embed a timestamp in the prompt, verify it travels through
ts=$($VRK epoch --now --quiet)
explain=$(echo "What happened at unix time $ts?" | $VRK prompt --explain)
assert_stdout "epoch ts embedded in prompt --explain" "$ts" "$explain"

# ---------------------------------------------------------------------------
# Section 12 — prompt (live, gated on ANTHROPIC_API_KEY)
# ---------------------------------------------------------------------------
echo "--- Section 12: prompt (live) ---"

if [ -z "${ANTHROPIC_API_KEY:-}" ] && [ -z "${OPENAI_API_KEY:-}" ]; then
  echo "SKIP: neither ANTHROPIC_API_KEY nor OPENAI_API_KEY set — skipping live prompt tests"
else
  VRK_KV_PATH="$TMPDIR/prompt_kv.db"
  export VRK_KV_PATH

  # tok budget check before prompt: gate the call
  text="What is 2 + 2?"
  echo "$text" | $VRK tok --budget 500 > /dev/null
  assert_exit "tok budget gate before prompt (under limit)" 0 $?

  # prompt + kv: cache the response by a uuid request key
  req_id=$($VRK uuid)
  response=$(echo "$text" | $VRK prompt)
  $VRK kv set "response:$req_id" "$response"
  cached=$($VRK kv get "response:$req_id")
  assert_stdout "prompt response cached in kv via uuid key" "4" "$cached"

  # coax + prompt: retry wrapper around a prompt call (succeeds on first attempt)
  PROMPT_SCRIPT="$TMPDIR/prompt_once.sh"
  VRK_ABS="$(cd "$(dirname "$VRK")" && pwd)/$(basename "$VRK")"
  printf '#!/bin/sh\necho "ping" | "%s" prompt --quiet\n' "$VRK_ABS" > "$PROMPT_SCRIPT"
  chmod +x "$PROMPT_SCRIPT"
  $VRK coax --times 2 -- "$PROMPT_SCRIPT"
  assert_exit "coax + prompt succeeds when API is reachable" 0 $?

  # jwt claim extraction → prompt (inject decoded claim as context)
  JWT=$(make_jwt '{"sub":"alice","role":"admin","exp":9999999999}')
  role=$(echo "$JWT" | $VRK jwt --claim role --quiet)
  answer=$(echo "Reply with only the word: $role" | $VRK prompt)
  assert_stdout "jwt claim → prompt injects decoded value" "admin" "$answer"

  unset VRK_KV_PATH
fi

# ---------------------------------------------------------------------------
# Section 13 — stdout empty on error
# ---------------------------------------------------------------------------
echo "--- Section 13: stdout empty on error ---"

# Helper: capture both stdout and exit code while suppressing stderr.
# Usage: capture_stdout_and_ec <varname_out> <varname_ec> <command...>
# Sets varname_out to stdout, varname_ec to exit code.
check_empty_stdout() {
  local desc="$1" expected_ec="$2"
  shift 2
  local out ec
  out=$(set +e; "$@" 2>/dev/null; true)
  ec=$(set +e; "$@" 2>/dev/null; echo $?)
  [ -z "$out" ] \
    && { echo "PASS: $desc — stdout empty"; PASS=$((PASS+1)); } \
    || { echo "FAIL: $desc — stdout not empty: $out"; FAIL=$((FAIL+1)); }
  assert_exit "$desc — exit code" "$expected_ec" "$ec"
}

# kv get missing key: stdout empty, exit 1 (runtime error — key not found)
VRK_KV_PATH="$TMPDIR/empty_check.db"
export VRK_KV_PATH
check_empty_stdout "kv get missing key" 1 $VRK kv get no_such_key_xyz
unset VRK_KV_PATH

# tok over budget: stdout empty, exit 1 (runtime error — budget exceeded)
check_empty_stdout "tok --budget exceeded" 1 sh -c "echo 'hello world' | $VRK tok --budget 1"

# epoch bad input (natural language): stdout empty, exit 2 (usage error — unparseable input)
check_empty_stdout "epoch bad input" 2 sh -c "echo 'next tuesday' | $VRK epoch"

# prompt with no API key: stdout empty, exit 1 (runtime error — missing credentials)
check_empty_stdout "prompt no-API-key" 1 sh -c "ANTHROPIC_API_KEY='' OPENAI_API_KEY='' $VRK prompt 'hello'"

# jwt garbage input plain mode: stdout empty, exit 1 (runtime error — invalid token)
check_empty_stdout "jwt bad token plain mode" 1 sh -c "echo 'not.a.jwt' | $VRK jwt"

# ---------------------------------------------------------------------------
# Section 14 — positional arguments
# ---------------------------------------------------------------------------
echo "--- Section 14: positional arguments ---"

# epoch positional: same result as stdin
iso_pos=$($VRK epoch --iso 1740009600)
iso_stdin=$(echo "1740009600" | $VRK epoch --iso)
[ "$iso_pos" = "$iso_stdin" ] \
  && { echo "PASS: epoch positional == stdin"; PASS=$((PASS+1)); } \
  || { echo "FAIL: epoch positional '$iso_pos' != stdin '$iso_stdin'"; FAIL=$((FAIL+1)); }

# jwt positional: decode a token passed as arg (not piped)
JWT=$(make_jwt '{"sub":"pos_test","exp":9999999999}')
claim_pos=$($VRK jwt --claim sub --quiet "$JWT")
claim_stdin=$(echo "$JWT" | $VRK jwt --claim sub --quiet)
[ "$claim_pos" = "$claim_stdin" ] \
  && { echo "PASS: jwt positional == stdin"; PASS=$((PASS+1)); } \
  || { echo "FAIL: jwt positional '$claim_pos' != stdin '$claim_stdin'"; FAIL=$((FAIL+1)); }

# tok positional: token count matches stdin path
count_pos=$($VRK tok --quiet "the quick brown fox")
count_stdin=$(echo "the quick brown fox" | $VRK tok --quiet)
[ "$count_pos" = "$count_stdin" ] \
  && { echo "PASS: tok positional == stdin"; PASS=$((PASS+1)); } \
  || { echo "FAIL: tok positional '$count_pos' != stdin '$count_stdin'"; FAIL=$((FAIL+1)); }

# prompt --explain positional: carries text as well as stdin path
explain_pos=$($VRK prompt --explain "hello world")
assert_stdout "prompt --explain positional carries text" "hello world" "$explain_pos"

# kv set with positional value (no stdin pipe)
VRK_KV_PATH="$TMPDIR/pos_kv.db"
export VRK_KV_PATH
key=$($VRK uuid)
$VRK kv set "$key" "positional_value"
result=$($VRK kv get "$key")
assert_stdout "kv set with positional value" "positional_value" "$result"

# three-tool chain using only positional args: epoch → kv → epoch
ts=$($VRK epoch --now --quiet)
$VRK kv set "pos_ts" "$ts"
retrieved=$($VRK kv get "pos_ts")
assert_stdout "epoch→kv→epoch positional chain" "$ts" "$retrieved"

unset VRK_KV_PATH

# ---------------------------------------------------------------------------
# Section 15 — meta-flags (agent discovery surface)
# ---------------------------------------------------------------------------
echo "--- Section 15: meta-flags ---"

# vrk --manifest: must exit 0 and emit valid JSON listing all tools
manifest=$($VRK --manifest)
assert_valid_json "--manifest produces valid JSON" "$manifest"
tool_count=$(echo "$manifest" | python3 -c 'import sys,json; d=json.load(sys.stdin); print(len(d["tools"]))')
[ "$tool_count" -eq 14 ] \
  && { echo "PASS: --manifest lists 14 tools"; PASS=$((PASS+1)); } \
  || { echo "FAIL: --manifest listed $tool_count tools (expected 14)"; FAIL=$((FAIL+1)); }
# each expected tool name must appear
for tool in jwt epoch uuid tok sse coax prompt kv chunk grab plain links validate mask; do
  echo "$manifest" | python3 -c "import sys,json; d=json.load(sys.stdin); names=[t['name'] for t in d['tools']]; sys.exit(0 if '$tool' in names else 1)" \
    && { echo "PASS: --manifest contains tool '$tool'"; PASS=$((PASS+1)); } \
    || { echo "FAIL: --manifest missing tool '$tool'"; FAIL=$((FAIL+1)); }
done

# vrk --skills: must exit 0 and return non-empty documentation
skills=$($VRK --skills)
[ -n "$skills" ] \
  && { echo "PASS: --skills returns non-empty output"; PASS=$((PASS+1)); } \
  || { echo "FAIL: --skills returned empty output"; FAIL=$((FAIL+1)); }
# spot-check that each tool section header is present
for tool in jwt epoch uuid tok sse coax prompt kv chunk grab links validate; do
  grep -q "## $tool" <<< "$skills" \
    && { echo "PASS: --skills contains section for '$tool'"; PASS=$((PASS+1)); } \
    || { echo "FAIL: --skills missing section for '$tool'"; FAIL=$((FAIL+1)); }
done

# vrk --skills <tool>: filtered output contains only that tool's section
for tool in jwt tok kv links; do
  section=$($VRK --skills "$tool")
  echo "$section" | grep -q "## $tool" \
    && { echo "PASS: --skills $tool section header present"; PASS=$((PASS+1)); } \
    || { echo "FAIL: --skills $tool missing section header"; FAIL=$((FAIL+1)); }
  # must not contain another tool's section header
  other="epoch"
  [ "$tool" = "epoch" ] && other="jwt"
  echo "$section" | grep -q "## $other" \
    && { echo "FAIL: --skills $tool leaked section for $other"; FAIL=$((FAIL+1)); } \
    || { echo "PASS: --skills $tool does not bleed into $other"; PASS=$((PASS+1)); }
done

# vrk (no args): must exit 2 and print usage to stderr, nothing to stdout
ec=$(set +e; $VRK 2>/dev/null; echo $?)
assert_exit "vrk no-args exits 2" 2 "$ec"
stdout_no_args=$(set +e; $VRK 2>/dev/null; true)
[ -z "$stdout_no_args" ] \
  && { echo "PASS: vrk no-args stdout is empty"; PASS=$((PASS+1)); } \
  || { echo "FAIL: vrk no-args leaked to stdout: $stdout_no_args"; FAIL=$((FAIL+1)); }
stderr_no_args=$($VRK 2>&1 >/dev/null || true)
echo "$stderr_no_args" | grep -q "usage" \
  && { echo "PASS: vrk no-args stderr contains usage"; PASS=$((PASS+1)); } \
  || { echo "FAIL: vrk no-args stderr missing usage: $stderr_no_args"; FAIL=$((FAIL+1)); }

# vrk unknown-tool: must exit 2
ec=$(set +e; $VRK not_a_real_tool 2>/dev/null; echo $?)
assert_exit "vrk unknown-tool exits 2" 2 "$ec"

# ---------------------------------------------------------------------------
# Section 16 — chunk pipeline composition
# ---------------------------------------------------------------------------
echo "--- Section 16: chunk pipelines ---"

# Build a deterministic input with a known token count.
# "hello" = 1 token, " hello" = 1 token → total = 60 tokens.
CHUNK_INPUT=$(python3 -c "import sys; sys.stdout.write('hello' + ' hello' * 59)")

# ── chunk produces valid JSONL with required fields ──────────────────────────
chunk_jsonl=$($VRK chunk --size 20 <<< "$CHUNK_INPUT")
chunk_count=$(echo "$chunk_jsonl" | wc -l | tr -d ' ')
[ "$chunk_count" -ge 3 ] \
  && { echo "PASS: chunk splits 60-token input into >= 3 chunks"; PASS=$((PASS+1)); } \
  || { echo "FAIL: chunk produced $chunk_count chunks, expected >= 3"; FAIL=$((FAIL+1)); }

fields_ok=$(echo "$chunk_jsonl" | python3 -c "
import sys, json
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    r = json.loads(line)
    if 'index' not in r or 'text' not in r or 'tokens' not in r:
        print('fail'); sys.exit(0)
print('ok')
" 2>/dev/null) || fields_ok="fail"
[ "$fields_ok" = "ok" ] \
  && { echo "PASS: chunk JSONL records have index, text, tokens fields"; PASS=$((PASS+1)); } \
  || { echo "FAIL: chunk JSONL missing required fields"; FAIL=$((FAIL+1)); }

# ── chunk | tok: tok agrees with the tokens field for every chunk ─────────────
# Extract each chunk's text, count with tok, compare to the tokens field.
tok_agree=$(echo "$chunk_jsonl" | python3 -c "
import sys, json, subprocess
ok = True
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    r = json.loads(line)
    result = subprocess.run(['$VRK', 'tok', r['text']], capture_output=True, text=True)
    if result.returncode != 0:
        print('fail: tok exited ' + str(result.returncode)); ok = False; break
    got = int(result.stdout.strip())
    if got != r['tokens']:
        print(f'fail: chunk {r[\"index\"]} tokens field={r[\"tokens\"]} but tok says {got}')
        ok = False; break
print('ok' if ok else '')
" 2>/dev/null) || tok_agree="fail"
[ "$tok_agree" = "ok" ] \
  && { echo "PASS: chunk | tok: tok count matches tokens field for every chunk"; PASS=$((PASS+1)); } \
  || { echo "FAIL: chunk | tok: $tok_agree"; FAIL=$((FAIL+1)); }

# ── chunk | tok: no chunk exceeds --size tokens according to tok ──────────────
tok_within=$(echo "$chunk_jsonl" | python3 -c "
import sys, json, subprocess
size = 20
ok = True
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    r = json.loads(line)
    result = subprocess.run(['$VRK', 'tok', r['text']], capture_output=True, text=True)
    count = int(result.stdout.strip()) if result.returncode == 0 else 9999
    if count > size:
        print(f'fail: chunk {r[\"index\"]} has {count} tokens via tok (limit {size})')
        ok = False; break
print('ok' if ok else '')
" 2>/dev/null) || tok_within="fail"
[ "$tok_within" = "ok" ] \
  && { echo "PASS: chunk | tok: every chunk within --size per tok"; PASS=$((PASS+1)); } \
  || { echo "FAIL: chunk | tok: $tok_within"; FAIL=$((FAIL+1)); }

# ── chunk | kv: store each chunk by index, retrieve and verify text ───────────
VRK_KV_PATH="$TMPDIR/chunk_kv.db"
export VRK_KV_PATH

store_ok="ok"
while IFS= read -r rec; do
  [ -z "$rec" ] && continue
  idx=$(echo "$rec" | python3 -c "import sys,json; print(json.loads(sys.stdin.read())['index'])" 2>/dev/null) || { store_ok="fail"; break; }
  text=$(echo "$rec" | python3 -c "import sys,json; print(json.loads(sys.stdin.read())['text'])" 2>/dev/null) || { store_ok="fail"; break; }
  $VRK kv set "chunk:$idx" "$text" || { store_ok="fail"; break; }
done <<< "$chunk_jsonl"

[ "$store_ok" = "ok" ] \
  && { echo "PASS: chunk | kv set: all chunks stored"; PASS=$((PASS+1)); } \
  || { echo "FAIL: chunk | kv set: failed to store a chunk"; FAIL=$((FAIL+1)); }

# Retrieve chunk 0 and verify it is non-empty and matches the first chunk's text.
first_text=$(echo "$chunk_jsonl" | python3 -c "
import sys, json
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    r = json.loads(line)
    if r['index'] == 0:
        print(r['text']); break
" 2>/dev/null)
retrieved=$($VRK kv get "chunk:0" 2>/dev/null) || retrieved=""
[ -n "$retrieved" ] && [ "$retrieved" = "$first_text" ] \
  && { echo "PASS: chunk | kv get: chunk 0 retrieved correctly"; PASS=$((PASS+1)); } \
  || { echo "FAIL: chunk | kv get: mismatch (got '$retrieved', want '$first_text')"; FAIL=$((FAIL+1)); }

unset VRK_KV_PATH

# ── chunk --by paragraph | tok: paragraph chunks each within --size ───────────
PARA_INPUT="The quick brown fox jumps over the lazy dog.

Pack my box with five dozen liquor jugs.

How vexingly quick daft zebras jump."

para_jsonl=$(printf '%s' "$PARA_INPUT" | $VRK chunk --size 500 --by paragraph 2>/dev/null)
para_tok_ok=$(echo "$para_jsonl" | python3 -c "
import sys, json, subprocess
size = 500
ok = True
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    r = json.loads(line)
    result = subprocess.run(['$VRK', 'tok', r['text']], capture_output=True, text=True)
    count = int(result.stdout.strip()) if result.returncode == 0 else 9999
    if count > size:
        print(f'fail: paragraph chunk {r[\"index\"]} has {count} tokens (limit {size})')
        ok = False; break
print('ok' if ok else '')
" 2>/dev/null) || para_tok_ok="fail"
[ "$para_tok_ok" = "ok" ] \
  && { echo "PASS: chunk --by paragraph | tok: every paragraph chunk within --size"; PASS=$((PASS+1)); } \
  || { echo "FAIL: chunk --by paragraph | tok: $para_tok_ok"; FAIL=$((FAIL+1)); }

# ── chunk --overlap | tok: overlapping chunks still each within --size ─────────
overlap_jsonl=$($VRK chunk --size 20 --overlap 5 <<< "$CHUNK_INPUT")
overlap_tok_ok=$(echo "$overlap_jsonl" | python3 -c "
import sys, json, subprocess
size = 20
ok = True
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    r = json.loads(line)
    result = subprocess.run(['$VRK', 'tok', r['text']], capture_output=True, text=True)
    count = int(result.stdout.strip()) if result.returncode == 0 else 9999
    if count > size:
        print(f'fail: overlap chunk {r[\"index\"]} has {count} tokens (limit {size})')
        ok = False; break
print('ok' if ok else '')
" 2>/dev/null) || overlap_tok_ok="fail"
[ "$overlap_tok_ok" = "ok" ] \
  && { echo "PASS: chunk --overlap | tok: every overlapping chunk within --size"; PASS=$((PASS+1)); } \
  || { echo "FAIL: chunk --overlap | tok: $overlap_tok_ok"; FAIL=$((FAIL+1)); }

# ── chunk empty input in pipeline context exits 0 ─────────────────────────────
ec=$(set +e; printf '' | $VRK chunk --size 100 2>/dev/null; echo $?)
assert_exit "chunk: empty input in pipeline exits 0" 0 "$ec"

# ── chunk | prompt --explain: chunk output can feed a prompt call ─────────────
# Extract first chunk's text and verify prompt --explain accepts it.
first_chunk_text=$(echo "$chunk_jsonl" | python3 -c "
import sys, json
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    r = json.loads(line)
    if r['index'] == 0:
        print(r['text']); break
" 2>/dev/null)
explain_out=$(echo "$first_chunk_text" | $VRK prompt --explain 2>/dev/null) || explain_out=""
[ -n "$explain_out" ] \
  && { echo "PASS: chunk | prompt --explain: first chunk accepted by prompt"; PASS=$((PASS+1)); } \
  || { echo "FAIL: chunk | prompt --explain: empty output"; FAIL=$((FAIL+1)); }

# ---------------------------------------------------------------------------
# Section 17 — grab pipeline composition
# ---------------------------------------------------------------------------
echo "--- Section 17: grab pipelines ---"

# ── grab | tok: token count of fetched content is a positive integer ──────────
grab_tok=$(set +e; $VRK grab https://example.com 2>/dev/null | $VRK tok 2>/dev/null; echo $?)
grab_tok_count=$(set +e; $VRK grab https://example.com 2>/dev/null | $VRK tok 2>/dev/null; true)
[ -n "$grab_tok_count" ] && [ "$grab_tok_count" -gt 0 ] 2>/dev/null \
  && { echo "PASS: grab | tok: token count > 0 ($grab_tok_count)"; PASS=$((PASS+1)); } \
  || { echo "FAIL: grab | tok: unexpected count '$grab_tok_count'"; FAIL=$((FAIL+1)); }

# ── grab --json: all required fields present ──────────────────────────────────
grab_json=$($VRK grab https://example.com --json 2>/dev/null) || grab_json=""
for field in url title content fetched_at status token_estimate; do
  echo "$grab_json" | python3 -c "import sys,json; d=json.loads(sys.stdin.read()); exit(0 if '$field' in d else 1)" 2>/dev/null \
    && { echo "PASS: grab --json has field '$field'"; PASS=$((PASS+1)); } \
    || { echo "FAIL: grab --json missing field '$field'"; FAIL=$((FAIL+1)); }
done

# ── grab | chunk: fetched content can be split into chunks ────────────────────
grab_chunks=$($VRK grab https://example.com 2>/dev/null | $VRK chunk --size 50 2>/dev/null)
grab_chunk_count=$(echo "$grab_chunks" | grep -c '^{' 2>/dev/null || echo 0)
[ "$grab_chunk_count" -ge 1 ] \
  && { echo "PASS: grab | chunk: produced $grab_chunk_count chunk(s)"; PASS=$((PASS+1)); } \
  || { echo "FAIL: grab | chunk: expected >= 1 chunk, got $grab_chunk_count"; FAIL=$((FAIL+1)); }

# ── grab invalid URL exits 2 (usage error, not runtime error) ─────────────────
ec=$(set +e; $VRK grab not-a-url 2>/dev/null; echo $?)
assert_exit "grab: invalid URL exits 2" 2 "$ec"

# ── grab stdout empty on error ────────────────────────────────────────────────
grab_err_stdout=$(set +e; $VRK grab not-a-url 2>/dev/null; true)
[ -z "$grab_err_stdout" ] \
  && { echo "PASS: grab: invalid URL — stdout empty"; PASS=$((PASS+1)); } \
  || { echo "FAIL: grab: invalid URL — stdout not empty: $grab_err_stdout"; FAIL=$((FAIL+1)); }

# ---------------------------------------------------------------------------
# Section 18: links pipelines
# ---------------------------------------------------------------------------
echo "--- Section 18: links pipelines ---"

# ── grab | links: produces at least one link record ───────────────────────
links_out=$($VRK grab https://example.com 2>/dev/null | $VRK links 2>/dev/null)
links_count=$(echo "$links_out" | grep -c '^{' 2>/dev/null || echo 0)
[ "$links_count" -ge 1 ] \
  && { echo "PASS: grab | links: produced $links_count link(s)"; PASS=$((PASS+1)); } \
  || { echo "FAIL: grab | links: expected >= 1 link, got $links_count"; FAIL=$((FAIL+1)); }

# ── grab | links: every record has non-empty url and line >= 1 ────────────
bad=$(echo "$links_out" | python3 -c "
import sys, json
bad = 0
for line in sys.stdin:
    line = line.strip()
    if not line: continue
    rec = json.loads(line)
    if '_vrk' in rec: continue
    if not rec.get('url') or rec.get('line', 0) < 1:
        bad += 1
print(bad)
" 2>/dev/null || echo 1)
[ "$bad" -eq 0 ] \
  && { echo "PASS: grab | links: all records have non-empty url and line >= 1"; PASS=$((PASS+1)); } \
  || { echo "FAIL: grab | links: $bad record(s) have empty url or line < 1"; FAIL=$((FAIL+1)); }

# ── grab | links --bare and --json from same fetch (cache avoids two fetches) ─
grab_cache=$(mktemp)
$VRK grab https://example.com 2>/dev/null > "$grab_cache"

bare_out=$($VRK links --bare < "$grab_cache" 2>/dev/null)
bare_count=$(echo "$bare_out" | grep -c 'http' 2>/dev/null || echo 0)
[ "$bare_count" -ge 1 ] \
  && { echo "PASS: grab | links --bare: produced $bare_count URL(s)"; PASS=$((PASS+1)); } \
  || { echo "FAIL: grab | links --bare: expected >= 1 URL"; FAIL=$((FAIL+1)); }

json_out=$($VRK links --json < "$grab_cache" 2>/dev/null)
meta=$(echo "$json_out" | tail -1)
echo "$meta" | python3 -c "import sys,json; d=json.loads(sys.stdin.read()); sys.exit(0 if d.get('_vrk')=='links' and isinstance(d.get('count'),int) else 1)" \
  && { echo "PASS: grab | links --json: trailing metadata record valid"; PASS=$((PASS+1)); } \
  || { echo "FAIL: grab | links --json: trailing metadata malformed: $meta"; FAIL=$((FAIL+1)); }

# Same cached input → counts must agree.
bare_n=$(echo "$bare_out" | wc -l | tr -d ' ')
json_n=$(echo "$meta" | python3 -c "import sys,json; print(json.loads(sys.stdin.read())['count'])" 2>/dev/null || echo -1)
[ "$bare_n" -eq "$json_n" ] \
  && { echo "PASS: grab | links: --bare count ($bare_n) matches --json count ($json_n)"; PASS=$((PASS+1)); } \
  || { echo "FAIL: grab | links: --bare count ($bare_n) != --json count ($json_n)"; FAIL=$((FAIL+1)); }

rm -f "$grab_cache"

# ---------------------------------------------------------------------------
# Section 19 — validate pipeline composition
# ---------------------------------------------------------------------------
echo "--- Section 19: validate pipelines ---"

SCHEMA_VAL='{"name":"string","age":"number"}'

# validate passes a valid record through to stdout unchanged.
val_out=$(echo '{"name":"alice","age":30}' | $VRK validate --schema "$SCHEMA_VAL" 2>/dev/null)
[ "$val_out" = '{"name":"alice","age":30}' ] \
  && { echo "PASS: validate passes valid record"; PASS=$((PASS+1)); } \
  || { echo "FAIL: validate valid record: got $val_out"; FAIL=$((FAIL+1)); }

# validate | tok: output is a positive integer.
tok_val=$(echo '{"name":"alice","age":30}' | $VRK validate --schema "$SCHEMA_VAL" | $VRK tok 2>/dev/null)
echo "$tok_val" | grep -qE '^[1-9][0-9]*$' \
  && { echo "PASS: validate | tok produces token count ($tok_val)"; PASS=$((PASS+1)); } \
  || { echo "FAIL: validate | tok: expected positive integer, got '$tok_val'"; FAIL=$((FAIL+1)); }

# Invalid record: stdout empty, stderr has warning, exit 0.
val_stdout=$(set +e; echo '{"name":"alice","age":"wrong"}' | $VRK validate --schema "$SCHEMA_VAL" 2>/dev/null; true)
val_exit=$(set +e; echo '{"name":"alice","age":"wrong"}' | $VRK validate --schema "$SCHEMA_VAL" >/dev/null 2>&1; echo $?)
[ -z "$val_stdout" ] \
  && { echo "PASS: validate invalid record — stdout empty"; PASS=$((PASS+1)); } \
  || { echo "FAIL: validate invalid record leaked to stdout: $val_stdout"; FAIL=$((FAIL+1)); }
[ "$val_exit" -eq 0 ] \
  && { echo "PASS: validate invalid record — exit 0 (warn mode)"; PASS=$((PASS+1)); } \
  || { echo "FAIL: validate invalid record — exit $val_exit, want 0"; FAIL=$((FAIL+1)); }

# --strict: exit 1 on first invalid line.
strict_exit=$(set +e; echo '{"name":"alice","age":"wrong"}' | $VRK validate --schema "$SCHEMA_VAL" --strict >/dev/null 2>&1; echo $?)
[ "$strict_exit" -eq 1 ] \
  && { echo "PASS: validate --strict exits 1 on invalid"; PASS=$((PASS+1)); } \
  || { echo "FAIL: validate --strict exited $strict_exit, want 1"; FAIL=$((FAIL+1)); }

# --json metadata record present as last line.
json_out=$(echo '{"name":"alice","age":30}' | $VRK validate --schema "$SCHEMA_VAL" --json 2>/dev/null)
meta_line=$(echo "$json_out" | tail -1)
echo "$meta_line" | python3 -c "import sys,json; d=json.loads(sys.stdin.read()); sys.exit(0 if d.get('_vrk')=='validate' and isinstance(d.get('total'),int) else 1)" \
  && { echo "PASS: validate --json metadata record valid"; PASS=$((PASS+1)); } \
  || { echo "FAIL: validate --json metadata malformed: $meta_line"; FAIL=$((FAIL+1)); }

# ---------------------------------------------------------------------------
# Results
# ---------------------------------------------------------------------------
echo ""
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
