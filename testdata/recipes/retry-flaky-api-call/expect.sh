# The retried command must still receive the document on stdin (never an empty prompt).
fail() { echo "expect: $*" >&2; exit 1; }
[ "$STEPS_RC" = 0 ] || fail "steps exited $STEPS_RC: $(head -c 300 "$OUT")"
[ "$(wc -l < "$MOCK_LOG" | tr -d " ")" = 1 ] || fail "want 1 LLM request, got $(wc -l < "$MOCK_LOG" | tr -d " ")"
jq -e '[.body.messages[] | select(.role=="user") | .content | contains("Quarterly report")] | any' "$MOCK_LOG" >/dev/null || fail "model never saw doc.txt"
grep -q '^MOCK' "$OUT" || fail "no model answer in output: $(head -c 200 "$OUT")"
