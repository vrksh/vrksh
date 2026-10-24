# Under budget: one call. Over budget: tok stops the pipeline and nothing is sent.
fail() { echo "expect: $*" >&2; exit 1; }
[ "$STEPS_RC" = 0 ] || fail "steps exited $STEPS_RC: $(head -c 300 "$OUT")"
[ "$(wc -l < "$MOCK_LOG" | tr -d " ")" = 1 ] || fail "want 1 LLM request, got $(wc -l < "$MOCK_LOG" | tr -d " ")"
for _ in $(seq 6000); do printf 'word '; done > prompt.txt
if bash "$STEPS" >over.out 2>&1 </dev/null; then fail "over-budget run exited 0"; fi
[ "$(wc -l < "$MOCK_LOG" | tr -d " ")" = 1 ] || fail "over-budget input still reached the LLM (over-budget run)"
