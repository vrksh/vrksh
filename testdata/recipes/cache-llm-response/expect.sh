# Same doc twice: 1 LLM call. Different doc: 2 calls. The harness already ran the steps once.
fail() { echo "expect: $*" >&2; exit 1; }
count() { wc -l < "$MOCK_LOG" | tr -d " "; }
[ "$STEPS_RC" = 0 ] || fail "first run exited $STEPS_RC"
[ "$(count)" = 1 ] || fail "after run 1: want 1 call, got $(count)"
bash "$STEPS" >/dev/null 2>&1 </dev/null
[ "$(count)" = 1 ] || fail "after run 2 on the same doc: want 1 call, got $(count) (cache not used)"
printf 'A different document about something else.\n' > doc.txt
bash "$STEPS" >/dev/null 2>&1 </dev/null
[ "$(count)" = 2 ] || fail "after run 3 on a new doc: want 2 calls, got $(count)"
