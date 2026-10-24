#!/usr/bin/env bash
# testdata/completions/smoke.sh
#
# End-to-end smoke tests for vrk completions. The bash script must work in a
# plain `bash --norc` shell with no bash-completion package (no _init_completion).
#
# Usage: VRK=./vrk ./testdata/completions/smoke.sh

set -euo pipefail

VRK="${VRK:-./vrk}"
PASS=0
FAIL=0

ok()   { echo "  PASS  $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL  $1"; echo "        $2"; FAIL=$((FAIL + 1)); }

# complete_bash <word-list...> - runs the generated bash completion for the
# given words (last one is being completed) and prints COMPREPLY, one per line.
complete_bash() {
  local n=$#
  bash --norc --noprofile -c '
    vrk="$1"; shift
    eval "$("$vrk" completions bash)"
    COMP_WORDS=(vrk "$@"); COMP_CWORD=$#
    f=$(complete -p vrk | sed -E "s/.*-F ([^ ]+).*/\1/")
    $f
    printf "%s\n" "${COMPREPLY[@]}"
  ' _ "$VRK" "$@"
}

echo "vrk completions - smoke tests"

out=$(complete_bash to)
if printf '%s\n' "$out" | grep -qx tok; then ok "bash: 'vrk to' completes tok"; else fail "bash: tool name" "got: $out"; fi

out=$(complete_bash tok --j)
if printf '%s\n' "$out" | grep -qx -- --json; then ok "bash: 'vrk tok --j' completes --json"; else fail "bash: flag" "got: $out"; fi

set +e
"$VRK" completions bash | bash -n; rc=$?
set -e
if [ "$rc" -eq 0 ]; then ok "bash: script passes bash -n"; else fail "bash -n" "exit $rc"; fi

echo ""
echo "---"
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
