# Joined deltas must equal the model text byte for byte, blank lines included.
fail() { echo "expect: $*" >&2; exit 1; }
[ "$STEPS_RC" = 0 ] || fail "steps exited $STEPS_RC: $(head -c 300 "$OUT")"
[ "$(cat "$OUT")" = "$(cat expected.txt)" ] || fail "text differs: $(od -c "$OUT" | head -3)"
