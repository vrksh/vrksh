# The mock model echoes the start of the doc, key included. mask must scrub it before kv.
fail() { echo "expect: $*" >&2; exit 1; }
[ "$STEPS_RC" = 0 ] || fail "steps exited $STEPS_RC: $(head -c 300 "$OUT")"
grep -q '' "$MOCK_LOG" || fail "fixture broken: key not in request"
v=$(vrk kv get summary) || fail "summary not stored"
case "$v" in **) fail "secret stored unredacted" ;; esac
case "$v" in *REDACTED*) ;; *) fail "summary has no [REDACTED]: $v" ;; esac
