# One request per file, each result stored under result:<name>.
fail() { echo "expect: $*" >&2; exit 1; }
[ "$STEPS_RC" = 0 ] || fail "steps exited $STEPS_RC"
[ "$(wc -l < "$MOCK_LOG" | tr -d " ")" = 3 ] || fail "want 3 LLM requests, got $(wc -l < "$MOCK_LOG" | tr -d " ")"
jq -e '[.body.messages[] | select(.role=="user") | .content | length > 0] | all' "$MOCK_LOG" >/dev/null || fail "empty user message sent"
for n in a b c; do
  v=$(vrk kv get "result:$n.md") || fail "no kv entry result:$n.md"
  case "$v" in MOCK*) ;; *) fail "result:$n.md is '$v'" ;; esac
done
