#!/usr/bin/env bash
# vrk v0.2.0 acceptance checks. Run from the repo root.
# Needs: bash, jq, go. NETWORK=1 enables network checks.
set -u
export LC_ALL=C.UTF-8
cd "$(dirname "$0")/.."
[ -x ./vrk ] || go build -o vrk .
VRK="$PWD/vrk"
T=$(mktemp -d); trap 'kill $MOCK_PID 2>/dev/null; rm -rf "$T"' EXIT
exec </dev/null
pass=0; fail=0
ok()  { echo "PASS $1"; pass=$((pass+1)); }
bad() { echo "FAIL $1: $2"; fail=$((fail+1)); }
# check ID expected_exit expected_stdout(literal, or - to skip) -- command...
# Set WANT_ERR=<substring> before check to also require that text on stderr.
check() {
  local id=$1 want_exit=$2 want_out=$3; shift 4
  local out code
  out=$("$@" 2>"$T/err"); code=$?
  [ "$code" = "$want_exit" ] || { bad "$id" "exit $code, want $want_exit; stderr: $(head -c 200 "$T/err")"; return; }
  [ "$want_out" = "-" ] || [ "$out" = "$want_out" ] || { bad "$id" "stdout '$(printf %s "$out" | head -c 200)', want '$want_out'"; return; }
  [ -z "${WANT_ERR:-}" ] || grep -qF "$WANT_ERR" "$T/err" || { bad "$id" "stderr lacks '$WANT_ERR': $(head -c 200 "$T/err")"; return; }
  ok "$id"
}
v() { "$VRK" "$@"; }

# ---- toolchain (DEP-1): the binary must be built with go.mod's pinned toolchain ----
want_tc=$(sed -n 's/^toolchain //p' go.mod)
have_tc=$(go version "$VRK" | awk '{print $2}')
[ -z "$want_tc" ] || [ "$have_tc" = "$want_tc" ] && ok A-TC || bad A-TC "binary built with $have_tc, go.mod pins $want_tc"

# ---- mock LLM ----
MOCK_LOG="$T/mock.jsonl"; : > "$MOCK_LOG"
# build first: killing `go run` would leave the compiled mock holding the port
go build -o "$T/mockllm" ./internal/mockllm
"$T/mockllm" --port 18765 --log "$MOCK_LOG" 2>"$T/mock.err" &
MOCK_PID=$!
for _ in $(seq 50); do grep -q listening "$T/mock.err" 2>/dev/null && break; sleep 0.2; done
grep -q listening "$T/mock.err" || { echo "mock LLM did not start (port 18765 busy?): $(cat "$T/mock.err")" >&2; exit 2; }
export OPENAI_API_KEY=sk-test
EP=(--endpoint http://127.0.0.1:18765/v1 -m mock)

# ---- coax ----
check A-01 0 "a b" -- v coax --times 1 -- printf '%s\n' 'a b'
check A-02 0 "x; echo INJECTED" -- v coax --times 1 -- echo 'x; echo INJECTED'
check A-03 3 - -- v coax --times 1 --backoff 1ms -- sh -c 'exit 3'
check A-04 0 - -- bash -c "sleep 5 | timeout 3 '$VRK' coax --times 1 -- true"
check A-05 1 "abcabcabc" -- bash -c "printf abc | '$VRK' coax --times 2 --backoff 1ms -- sh -c 'cat; exit 1'"

# ---- 2 MiB lines ----
head -c 2097152 /dev/zero | tr '\0' a > "$T/big.txt"; echo >> "$T/big.txt"
printf '{"t":"%s"}\n' "$(head -c 2097152 /dev/zero | tr '\0' a)" > "$T/big.jsonl"
printf 'data: {"t":"%s"}\n\n' "$(head -c 2097152 /dev/zero | tr '\0' a)" > "$T/big.sse"
for c in "mask" "emit" "sip --first 1" "throttle --rate 1000/s" "slug" "recase --to snake"; do
  check "A-07 $c" 0 - -- bash -c "'$VRK' $c < '$T/big.txt' > /dev/null"
done
check "A-07 sse" 0 - -- bash -c "'$VRK' sse < '$T/big.sse' > /dev/null"
for c in "jsonl --collect" "validate --schema '{\"t\":\"string\"}'" "assert .t"; do
  check "A-07 $c" 0 - -- bash -c "'$VRK' $c < '$T/big.jsonl' > /dev/null"
done

# ---- chunk ----
FFFD=$(printf '\xef\xbf\xbd')
for s in "यह एक साधारण वाक्य है। " "Emoji party 🎉🚀🔥 time. " "Ελληνικά κείμενο εδώ. " "東京は日本の首都です。"; do
  for _ in $(seq 60); do printf '%s' "$s"; done > "$T/u.txt"
  n=$(v chunk --size 20 < "$T/u.txt" | jq -r '.text' | grep -c "$FFFD")
  [ "$n" = 0 ] && ok "A-08 ${s:0:8}" || bad "A-08 ${s:0:8}" "$n chunks with U+FFFD"
  joined=$(v chunk --size 20 --overlap 0 < "$T/u.txt" | jq -j '.text')
  [ "$joined" = "$(cat "$T/u.txt")" ] && ok "A-09 ${s:0:8}" || bad "A-09 ${s:0:8}" "rejoin differs"
done
for _ in $(seq 30000); do printf 'the quick brown fox jumps over the lazy dog '; done | head -c 1048576 > "$T/1mb.txt"
s=$(date +%s%N); v chunk --size 500 < "$T/1mb.txt" > /dev/null; ms=$(( ($(date +%s%N)-s)/1000000 ))
[ "$ms" -lt 2000 ] && ok "A-10 chunk 1MiB ${ms}ms" || echo "WARN A-10 chunk 1MiB took ${ms}ms (target 2000)"

# ---- slug ----
check A-11 0 "$(printf 'a\nनमस्ते-दुनिया\nstrasse\n東京-2026')" -- bash -c "printf 'a\nनमस्ते दुनिया\nStraße\n東京 2026\n' | '$VRK' slug"

# ---- sse ----
check A-12a 0 "Hi" -- bash -c "printf 'data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n' | '$VRK' sse --field 'data.choices[0].delta.content'"
WANT_ERR="matched no events" check A-12b 1 - -- bash -c "printf 'data: {\"a\":1}\n\n' | '$VRK' sse --field data.b"
sse_md() {
  for t in '# Title' '\n\n' 'First para.' '\n\n' '- item'; do
    printf 'event: content_block_delta\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":"%s"}}\n\n' "$t"
  done
}
sse_md | v sse --event content_block_delta --field data.delta.text --raw > "$T/raw.out" 2>"$T/err"
printf '# Title\n\nFirst para.\n\n- item' | cmp -s - "$T/raw.out" && ok A-12c || bad A-12c "raw output differs: $(od -c "$T/raw.out" | head -3)"

# ---- digest ----
H=$(echo m | v digest --hmac --key k)
check A-15a 0 - -- bash -c "echo m | '$VRK' digest --hmac --key k --verify '$H'"
W=$(echo other | v digest --hmac --key k)
check A-15b 1 - -- bash -c "echo m | '$VRK' digest --hmac --key k --verify '$W'"
check A-15c 2 - -- bash -c "echo m | '$VRK' digest --hmac --key k --verify 'md5:${H#sha256:}'"

# ---- validate ----
printf '{"a":"x"}\n{"a":5}\n' | v validate --schema '{"a":"string"}' > /dev/null 2> "$T/verr"
grep -q "line 2:" "$T/verr" && ok A-16 || bad A-16 "no 'line 2:' in: $(cat "$T/verr")"
WANT_ERR="JSON Schema" check A-17 2 - -- bash -c "echo '{}' | '$VRK' validate --schema '{\"type\":\"object\",\"properties\":{\"a\":{\"type\":\"string\"}}}'"

# ---- epoch ----
check A-18a 0 "2026-10-15T05:31:15Z" -- v epoch --iso 1792042275000
check A-18b 1 - -- v epoch not-a-date
WANT_ERR="invalid date" check A-18c 1 - -- v epoch 2026-02-29

# ---- mask ----
miss=0; while IFS= read -r l; do printf '%s\n' "$l" | v mask | grep -q '\[REDACTED\]' || { miss=$((miss+1)); echo "  not redacted: $l"; }; done < testdata/mask/secrets.txt
[ "$miss" = 0 ] && ok A-19a || bad A-19a "$miss secrets not redacted"
v mask < testdata/mask/decoys.txt | diff - testdata/mask/decoys.txt > "$T/md" && ok A-19b || bad A-19b "$(cat "$T/md")"
check A-19c 0 "key [REDACTED], please" -- bash -c "echo 'key sk-proj-FAKE0000example0000notreal0000000, please' | '$VRK' mask"

# ---- prompt ----
echo j | v prompt -m gpt-4o-mini --schema '{"a":"string"}' --explain > "$T/ex" 2>&1
body=$(sed -n "s/.*-d '\(.*\)'.*/\1/p" "$T/ex")
echo "$body" | jq -e '.response_format.json_schema.schema.type=="object" and (.response_format.json_schema.schema.required|index("a"))!=null' >/dev/null \
  && ok A-20a || bad A-20a "schema not valid JSON Schema: $body"
: > "$MOCK_LOG"
check A-20b 1 - -- bash -c "echo j | '$VRK' prompt ${EP[*]} --system 'respond json {\"z\":\"string\"}' --schema '{\"a\":\"string\"}' --fail"
: > "$MOCK_LOG"
check A-20c 1 - -- bash -c "echo j | '$VRK' prompt ${EP[*]} --system 'respond json {\"z\":\"string\"}' --schema '{\"a\":\"string\"}' --retry 2"
[ "$(wc -l < "$MOCK_LOG" | tr -d " ")" = 3 ] && ok "A-20c requests" || bad "A-20c requests" "$(wc -l < "$MOCK_LOG"), want 3"
: > "$MOCK_LOG"
echo j | v prompt "${EP[@]}" --schema '{"a":"string"}' > /dev/null 2>&1
jq -e '[.body.messages[] | select(.role=="system") | .content | test("\"a\"")] | any' "$MOCK_LOG" >/dev/null \
  && ok A-20d || bad A-20d "no system message with the schema key"
: > "$MOCK_LOG"
check A-20e 1 - -- bash -c "printf '' | '$VRK' prompt ${EP[*]}"
[ "$(wc -l < "$MOCK_LOG" | tr -d " ")" = 0 ] && ok "A-20e requests" || bad "A-20e requests" "a request was sent for an empty prompt"

# ---- CLI surface ----
ver=$(v --version 2>/dev/null); case "$ver" in "vrk "*) ok "A-21 --version ($ver)";; *) bad A-21 "--version gave '$ver'";; esac
v help | grep -qE '^  tok ' && ok "A-21 help lists tools" || bad A-21 "help does not list tok"
check "A-21 bare alias" 0 - -- v bare --dry-run
mv=$(printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}' | v mcp | jq -r '.result.serverInfo.version')
[ "vrk $mv" = "$ver" ] && ok "A-21 mcp version" || bad A-21 "mcp version '$mv' vs '$ver'"
comp=$(bash --norc --noprofile -c "
  eval \"\$('$VRK' completions bash)\"
  COMP_WORDS=(vrk to); COMP_CWORD=1; COMP_LINE='vrk to'; COMP_POINT=6
  f=\$(complete -p vrk | sed -E 's/.*-F ([^ ]+).*/\1/'); \$f
  printf '%s\n' \"\${COMPREPLY[@]}\"")
printf '%s\n' "$comp" | grep -qx tok && ok A-22 || bad A-22 "completion gave: $comp"

# ---- install (network) ----
if [ "${NETWORK:-0}" = 1 ]; then
  latest=$(curl -fsSL https://api.github.com/repos/vrksh/vrksh/releases/latest | sed -n 's/.*"tag_name": *"v\([^"]*\)".*/\1/p')
  VRK_INSTALL_DIR="$T/bin" sh hugo/static/install.sh > /dev/null 2>&1
  [ "$("$T/bin/vrk" --version)" = "vrk $latest" ] && ok "A-23 install $latest" || bad A-23 "installed $("$T/bin/vrk" --version 2>&1), latest $latest"
else
  echo "SKIP A-23 (set NETWORK=1)"
fi

# ---- docs and recipes ----
check "A-24 doccheck" 0 - -- make -s doccheck
check "A-24 recipecheck" 0 - -- make -s recipecheck

echo; echo "passed: $pass  failed: $fail"
[ "$fail" = 0 ]
