# Model output is validated against entities.json before it is stored.
fail() { echo "expect: $*" >&2; exit 1; }
[ "$STEPS_RC" = 0 ] || fail "steps exited $STEPS_RC: $(head -c 300 "$OUT")"
v=$(vrk kv get entities) || fail "entities not stored"
echo "$v" | jq -e '.entities | type == "array"' >/dev/null || fail "stored value is not valid entities JSON: $v"
