# The retried command must still receive the document on stdin (never an empty prompt).
fail() { echo "expect: $*" >&2; exit 1; }
[ "$STEPS_RC" = 0 ] || fail "steps exited $STEPS_RC: $(head -c 300 "$OUT")"
[ "$(wc -l < "$MOCK_LOG" | tr -d " ")" = 1 ] || fail "want 1 LLM request, got $(wc -l < "$MOCK_LOG" | tr -d " ")"
jq -e '[.body.messages[] | select(.role=="user") | .content | contains("Quarterly report")] | any' "$MOCK_LOG" >/dev/null || fail "model never saw doc.txt"
jq -e 'has("summary")' "$OUT" >/dev/null || fail "output is not summary JSON: $(head -c 200 "$OUT")"

# A 500 on the first request must be retried, not swallowed by the pipe.
"$MOCKLLM" --port 0 --log flaky.jsonl --fail-first 1 2>flaky.err &
mock_pid=$!
trap 'kill $mock_pid 2>/dev/null' EXIT
for _ in 1 2 3 4 5 6 7 8 9 10; do grep -q 'listening on' flaky.err && break; sleep 0.2; done
addr=$(sed -n 's/^listening on //p' flaky.err)
[ -n "$addr" ] || fail "flaky mock did not start"
VRK_LLM_URL="http://$addr/v1" bash "$STEPS" >flaky.out 2>&1 </dev/null || fail "500 then success should exit 0: $(head -c 300 flaky.out)"
[ "$(wc -l < flaky.jsonl | tr -d " ")" = 2 ] || fail "want 2 requests (500, then retry), got $(wc -l < flaky.jsonl | tr -d " ")"
grep -q '"summary"' flaky.out || fail "retry did not produce summary JSON: $(head -c 200 flaky.out)"
