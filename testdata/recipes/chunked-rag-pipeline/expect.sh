# Every chunk is stored under doc:<id>:chunk:<index>, and the chunks match vrk chunk.
fail() { echo "expect: $*" >&2; exit 1; }
[ "$STEPS_RC" = 0 ] || fail "steps exited $STEPS_RC"
want=$(vrk chunk --size 512 --overlap 64 < doc.txt | wc -l | tr -d ' ')
[ "$want" -ge 2 ] || fail "fixture too small, only $want chunks"
keys=$(vrk kv list | grep -c ':chunk:' || true)
[ "$keys" = "$want" ] || fail "stored $keys chunks, want $want"
k=$(vrk kv list | grep ':chunk:0$' | head -1)
[ -n "$k" ] || fail "no chunk:0 key"
first=$(vrk chunk --size 512 --overlap 64 < doc.txt | head -1 | jq -r .text)
[ "$(vrk kv get "$k")" = "$first" ] || fail "chunk 0 differs from vrk chunk output"
