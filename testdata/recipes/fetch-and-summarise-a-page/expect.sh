# Rerun the recipe with the fetch pointed at the local fixture site (no network needed).
fail() { echo "expect: $*" >&2; exit 1; }
: > "$MOCK_LOG"
sed "s#grab https://example.com#grab --allow-internal $SITE_URL/index.html#g" "$STEPS" > local.sh
grep -q "$SITE_URL" local.sh || fail "recipe no longer fetches https://example.com; update this expect.sh"
bash local.sh >local.out 2>&1 </dev/null || fail "recipe failed: $(head -c 300 local.out)"
[ "$(wc -l < "$MOCK_LOG" | tr -d " ")" = 1 ] || fail "want 1 LLM request, got $(wc -l < "$MOCK_LOG" | tr -d " ")"
grep -qi quokka "$MOCK_LOG" || fail "page text did not reach the model"
