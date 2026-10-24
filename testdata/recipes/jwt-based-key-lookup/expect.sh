# The sub claim of TOKEN selects the kv entry. env seeds user:alice.
fail() { echo "expect: $*" >&2; exit 1; }
[ "$STEPS_RC" = 0 ] || fail "steps exited $STEPS_RC: $(head -c 300 "$OUT")"
[ "$(cat "$OUT")" = '{"plan":"pro"}' ] || fail "output was: $(head -c 200 "$OUT")"
