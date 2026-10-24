# Mask must run before the LLM: the model gets [REDACTED], the output is valid summary JSON.
fail() { echo "expect: $*" >&2; exit 1; }
[ "$STEPS_RC" = 0 ] || fail "steps exited $STEPS_RC"
[ "$(wc -l < "$MOCK_LOG" | tr -d " ")" = 1 ] || fail "want 1 LLM request, got $(wc -l < "$MOCK_LOG" | tr -d " ")"
grep -q '' "$MOCK_LOG" && fail "secret reached the LLM"
grep -q 'REDACTED' "$MOCK_LOG" || fail "request has no [REDACTED] marker"
grep -q '' "$OUT" && fail "secret in pipeline output"
jq -e 'has("summary")' "$OUT" >/dev/null || fail "output is not summary JSON: $(head -c 200 "$OUT")"
